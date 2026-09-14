package api

// interest.go —— 兴趣模型的胶水层：把 internal/interest 的纯逻辑接到 Postgres 上，
// 并把她的那棵树读出来。
//
// # 一次采集只有一个事务了
//
// 2026-09-04 之前这里分两段：先在事务里认词、插来源、重算强度，再在事务外跑
// 三档路由（因为 T3 要发一次模型调用，而横跨模型调用的事务会把行锁按秒攥住）。
//
// **路由没有了。** 领域词表自带 disciplines[] 这条静态边，一个词连哪几门学科
// 是查表得到的，不花调用、不占时间，所以写边和写词现在在同一个事务里完成。
//
// # 失败姿态
//
// 采集是**锦上添花**：它绝不能让学生的那次请求失败，也绝不能凭空编一个词出来。
// 所以这里所有的错误都是 slog.Warn + 少长一个词，没有一个会冒泡成 HTTP 错误。
// 见 AGENTS.md 与 memory 里的 ai-errors-must-surface-never-fake：「不编」和
// 「不吵」在这里并不矛盾 —— 采集失败对学生是不可见的（她本来也没要求长词），
// 而一个编出来的关键词会以她无从反驳的方式挂在她的树上。

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/disciplines"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/interest"
	"mindimprint/api/internal/interests"
	"mindimprint/api/internal/store/sqlc"
)

/* ── 落库 ───────────────────────────────────────────────────────────────── */

// plantKeywords 把一次采集的产物种进她的树。
//
// refID 是这次完成的 atom（news / quiz 没有 atom，传 uuid.Nil）。同一个
// (词, 类型, 来源) 只会计一次来源，所以重复调用是安全的 —— 重新打开一篇已经
// 采集过的阅读不会把强度刷上去。
//
// 🔑 **中文名、英文名、主枝全部从词表查表得到**，`Harvested` 里根本没有这三个
// 字段。模型交出来的只有一个 id，它说不出树上的那几个字。
func (a *API) plantKeywords(
	ctx context.Context,
	userID uuid.UUID,
	kind string,
	refID uuid.UUID,
	label string,
	hs []interest.Harvested,
) {
	if len(hs) == 0 {
		return
	}

	tx, err := a.d.Pool.Begin(ctx)
	if err != nil {
		slog.Warn("interest: begin failed", "err", err, "user_id", userID)
		return
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	qtx := a.d.Queries.WithTx(tx)

	for _, h := range hs {
		// 闭表那条不变量在写库前再站一个人。解析器已经挡过一遍，但这一条和
		// 「没有原话的词不落库」一样是树的地基，值得重复。
		it, ok := interests.ByID(h.InterestID)
		if !ok || h.Evidence == "" {
			continue
		}
		norm := interest.Norm(it.Zh)
		if norm == "" {
			continue
		}
		row, err := qtx.UpsertInterestKeyword(ctx, sqlc.UpsertInterestKeywordParams{
			UserID: userID, TextZh: it.Zh, TextEn: it.En,
			Norm: norm, Field: it.Field, Note: h.Note,
			InterestID: &it.ID,
		})
		if err != nil {
			slog.Warn("interest: upsert keyword failed", "err", err, "interest_id", it.ID)
			continue
		}
		if err := qtx.AddKeywordSource(ctx, sqlc.AddKeywordSourceParams{
			KeywordID: row.ID, Kind: kind,
			RefID:      pgtype.UUID{Bytes: refID, Valid: refID != uuid.Nil},
			Label:      label,
			Evidence:   h.Evidence,
			HappenedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
		}); err != nil {
			slog.Warn("interest: add source failed", "err", err, "keyword_id", row.ID)
			continue
		}
		n, err := qtx.CountKeywordSources(ctx, row.ID)
		if err != nil {
			slog.Warn("interest: count sources failed", "err", err, "keyword_id", row.ID)
			continue
		}
		// 强度只有这一条写入路径：来源条数的函数，从不由调用方指定。
		if err := qtx.RecountKeywordStrength(ctx, sqlc.RecountKeywordStrengthParams{
			ID: row.ID, Strength: int32(interest.Strength(int(n))),
		}); err != nil {
			slog.Warn("interest: recount strength failed", "err", err, "keyword_id", row.ID)
			continue
		}
		// 词 → 学科的边。查表，不判定。
		linkCatalogDisciplines(ctx, qtx, row.ID, it)
	}

	if err := tx.Commit(ctx); err != nil {
		slog.Warn("interest: commit failed", "err", err, "user_id", userID)
	}
}

// linkCatalogDisciplines 把词表里写好的那几条边写进库。
//
// how='catalog' 而不是 'llm'：这条边是人写在 interests.json 里的，比任何一档
// 模型判定都确定。confidence 恒为 1 —— 一个作者写下的归属没有「有多确定」这
// 回事，把它写成 0.8 只是在假装这里还有一个概率。
//
// UpsertKeywordDiscipline 的 ON CONFLICT 带着 `WHERE how <> 'student'`，所以
// **她自己在抽屉里改过的边不会被覆盖**。这条保证在路由退休之后仍然成立，因为
// 它一直住在 SQL 里，不在 Go 里。
func linkCatalogDisciplines(
	ctx context.Context, q *sqlc.Queries, keywordID uuid.UUID, it interests.Interest,
) {
	for _, did := range it.Disciplines {
		if _, ok := disciplines.ByID(did); !ok {
			// TestEveryDisciplineIDIsReal 守着这件事不该发生；真发生了，
			// 跳过一条边好过写一个指向空气的 id 进库。
			slog.Warn("interest: catalog points at unknown discipline",
				"interest_id", it.ID, "discipline_id", did)
			continue
		}
		if err := q.UpsertKeywordDiscipline(ctx, sqlc.UpsertKeywordDisciplineParams{
			KeywordID: keywordID, DisciplineID: did,
			Confidence: 1, How: "catalog", Rationale: "",
		}); err != nil {
			slog.Warn("interest: upsert edge failed", "err", err, "keyword_id", keywordID)
		}
	}
}

/* ── 读出那棵树 ─────────────────────────────────────────────────────────── */

type interestSourceDTO struct {
	Kind       string `json:"kind"`
	RefID      string `json:"refId,omitempty"`
	Label      string `json:"label"`
	Evidence   string `json:"evidence"`
	HappenedAt string `json:"happenedAt"`
}

// interestDisciplineDTO 把那条边和学科表里的静态内容合在一起，因为前端不该为了
// 渲染一个抽屉再去查一次学科表 —— asks / method / exemplar / syllabus 都是内容，
// 一起发出去比让前端 join 便宜也更不容易错。
type interestDisciplineDTO struct {
	ID         string                    `json:"id"`
	Zh         string                    `json:"zh"`
	En         string                    `json:"en"`
	Asks       string                    `json:"asks"`
	Method     string                    `json:"method"`
	Exemplar   string                    `json:"exemplar"`
	Syllabus   []disciplines.SyllabusRef `json:"syllabus"`
	Confidence float32                   `json:"confidence"`
	How        string                    `json:"how"`
	Rationale  string                    `json:"rationale"`
}

type interestKeywordDTO struct {
	ID string `json:"id"`
	// InterestID 指向 interests.json 的那一条。前端拿它算「你可能还会感兴趣的」：
	// 排除她已经有的词，靠 id 而不是靠中文名对得上——中文名今天唯一，但那是词表
	// 的一条测试在守，不是这个接口的保证。
	InterestID  string                  `json:"interestId"`
	TextZh      string                  `json:"textZh"`
	TextEn      string                  `json:"textEn"`
	Field       string                  `json:"field"`
	Strength    int32                   `json:"strength"`
	Note        string                  `json:"note"`
	FirstSeenAt string                  `json:"firstSeenAt"`
	Sources     []interestSourceDTO     `json:"sources"`
	Disciplines []interestDisciplineDTO `json:"disciplines"`
}

type interestFieldDTO struct {
	ID           string `json:"id"`
	Label        string `json:"label"`
	KeywordCount int    `json:"keywordCount"`
}

// getInterestTree —— GET /api/v1/interest/tree
func (a *API) getInterestTree(w http.ResponseWriter, r *http.Request) {
	u, ok := UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, r, httpx.ErrUnauthorized("未登录"))
		return
	}
	// **这是一次纯读。** 采集在后台队列里跑（interest_jobs.go）。
	tree, err := a.buildInterestTree(r.Context(), u.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, tree)
}

// buildInterestTree is the tree payload for one user. Shared by the
// student's own GET /interest/tree and the teacher's read-only view of a
// student (lite_teacher_tree.go).
//
// 三张表各查一次，Go 侧按 keyword_id 分组。每个词发一次查询，会在一棵二十个词
// 的树上变成四十次往返。
func (a *API) buildInterestTree(ctx context.Context, userID uuid.UUID) (map[string]any, error) {
	keywords, err := a.d.Queries.ListInterestKeywords(ctx, userID)
	if err != nil {
		return nil, err
	}
	sources, err := a.d.Queries.ListKeywordSourcesForUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	edges, err := a.d.Queries.ListKeywordDisciplinesForUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	srcByKeyword := make(map[uuid.UUID][]interestSourceDTO, len(keywords))
	for _, s := range sources {
		ref := ""
		if s.RefID.Valid {
			ref = uuid.UUID(s.RefID.Bytes).String()
		}
		srcByKeyword[s.KeywordID] = append(srcByKeyword[s.KeywordID], interestSourceDTO{
			Kind: s.Kind, RefID: ref, Label: s.Label, Evidence: s.Evidence,
			HappenedAt: s.HappenedAt.Format(time.RFC3339),
		})
	}

	edgeByKeyword := make(map[uuid.UUID][]interestDisciplineDTO, len(keywords))
	for _, e := range edges {
		d, ok := disciplines.ByID(e.DisciplineID)
		if !ok {
			// 学科表里删掉过的 id。跳过好过发一个只有 id 的空壳给前端去渲染。
			continue
		}
		edgeByKeyword[e.KeywordID] = append(edgeByKeyword[e.KeywordID], interestDisciplineDTO{
			ID: d.ID, Zh: d.Zh, En: d.En, Asks: d.Asks, Method: d.Method,
			Exemplar: d.Exemplar, Syllabus: d.Syllabus,
			Confidence: e.Confidence, How: e.How, Rationale: e.Rationale,
		})
	}

	out := make([]interestKeywordDTO, 0, len(keywords))
	perField := map[string]int{}
	for _, k := range keywords {
		perField[k.Field]++
		out = append(out, interestKeywordDTO{
			ID: k.ID.String(), InterestID: derefString(k.InterestID),
			TextZh: k.TextZh, TextEn: k.TextEn, Field: k.Field,
			Strength: k.Strength, Note: k.Note,
			FirstSeenAt: k.FirstSeenAt.Format(time.RFC3339),
			Sources:     srcByKeyword[k.ID],
			Disciplines: edgeByKeyword[k.ID],
		})
	}

	// 七根主枝**全部**发出去，包括一个词都没有的那几根。空枝不是缺数据，它是
	// 这棵树上最有用的一条信息：她还没走过的方向。前端拿它画未点亮的枝。
	fields := make([]interestFieldDTO, 0, len(disciplines.Fields))
	for _, f := range disciplines.Fields {
		fields = append(fields, interestFieldDTO{
			ID: f, Label: disciplines.FieldLabels[f], KeywordCount: perField[f],
		})
	}

	return map[string]any{"fields": fields, "keywords": out}, nil
}

// derefString 把可空的 interest_id 变成一个字符串。空的那种情况是迁移 0134
// 之前留下的行——它们在归档表里，正常路径上取不到，但列的类型仍然可空。
func derefString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
