package api

// interest.go —— 兴趣模型的胶水层：把 internal/interest 的纯逻辑接到 Postgres
// 和网关上，并把她的那棵树读出来。
//
// # 事务边界，以及为什么路由发生在事务外面
//
// plantKeywords 分两段：
//
//	第一段（一个事务）  认词 · 插来源 · 按来源条数重算强度
//	第二段（事务外）    给还没有学科的词跑路由，需要时才发那一次模型调用
//
// 拆开是有原因的：T3 档要发一次网络请求，而一个横跨模型调用的事务会把
// interest_keyword 上的行锁按秒计地攥在手里。第二段的每一步都是单条幂等语句
// （UpsertKeywordDiscipline 是 upsert），中途失败只是这个词暂时没有学科，
// 下次采集会再试 —— 而她的词和来源已经稳稳落库了。
//
// # 失败姿态
//
// 采集与路由都是**锦上添花**：它们绝不能让学生的那次请求失败，也绝不能凭空
// 编一个词出来。所以这里所有的错误都是 slog.Warn + 少长一个词，没有一个会
// 冒泡成 HTTP 错误。见 AGENTS.md 与 memory 里的 ai-errors-must-surface-never-fake：
// 「不编」和「不吵」在这里并不矛盾 —— 采集失败对学生是不可见的（她本来也没
// 要求长词），而一个编出来的关键词会以她无从反驳的方式挂在她的树上。

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/disciplines"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/interest"
	"mindimprint/api/internal/store/sqlc"
)

/* ── 落库 ───────────────────────────────────────────────────────────────── */

// plantKeywords 把一次采集的产物种进她的树。
//
// refID 是这次完成的 atom（news / quiz 没有 atom，传 uuid.Nil）。同一个
// (词, 类型, 来源) 只会计一次来源，所以重复调用是安全的 —— 重新打开一篇已经
// 采集过的阅读不会把强度刷上去。
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

	planted := make([]sqlc.InterestKeyword, 0, len(hs))
	for _, h := range hs {
		// 防御性重复一次纯逻辑层的规则：没有原话的词绝不落库。解析器已经挡过
		// 一遍，但这条是树的地基，值得在写库前再站一个人。
		if h.Evidence == "" || !disciplines.IsField(h.Field) {
			continue
		}
		norm := interest.Norm(h.TextZh)
		if norm == "" {
			continue
		}
		row, err := qtx.UpsertInterestKeyword(ctx, sqlc.UpsertInterestKeywordParams{
			UserID: userID, TextZh: h.TextZh, TextEn: h.TextEn,
			Norm: norm, Field: h.Field, Note: h.Note,
		})
		if err != nil {
			slog.Warn("interest: upsert keyword failed", "err", err, "keyword", h.TextZh)
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
		planted = append(planted, row)
	}

	if err := tx.Commit(ctx); err != nil {
		slog.Warn("interest: commit failed", "err", err, "user_id", userID)
		return
	}

	// 第二段：路由。事务已经提交，词和来源不会再因为路由失败而丢。
	a.routeKeywords(ctx, userID, planted, hs)
}

// routeKeywords 给还没有学科的词连上学科。
//
// 便宜的两档先跑；两档都空了，才为这一个词花一次模型调用。**已经有边的词直接
// 跳过** —— 一个词路由一次就永久缓存，这就是路由成本是 O(新词) 而不是 O(活动)
// 的原因。
func (a *API) routeKeywords(
	ctx context.Context,
	userID uuid.UUID,
	planted []sqlc.InterestKeyword,
	hs []interest.Harvested,
) {
	if len(planted) == 0 {
		return
	}
	rows, err := a.d.Queries.ListRoutedKeywordsForUser(ctx, userID)
	if err != nil {
		slog.Warn("interest: load routed keywords failed", "err", err, "user_id", userID)
		return
	}

	known := make([]interest.Known, 0, len(rows))
	routed := make(map[uuid.UUID]bool, len(rows))
	refsOf := make(map[uuid.UUID][]string, len(rows))
	for _, r := range rows {
		known = append(known, interest.Known{
			KeywordNorm: r.Norm, DisciplineIDs: r.DisciplineIds, SourceRefs: r.SourceRefs,
		})
		if len(r.DisciplineIds) > 0 {
			routed[r.ID] = true
		}
		refsOf[r.ID] = r.SourceRefs
	}

	// 采集器给的原话，按词索引 —— T3 的 prompt 少了它就只能猜词面。
	evidenceOf := make(map[string]string, len(hs))
	for _, h := range hs {
		evidenceOf[interest.Norm(h.TextZh)] = h.Evidence
	}

	for _, k := range planted {
		if routed[k.ID] {
			continue
		}
		routes := interest.RouteCheap(k.TextZh, refsOf[k.ID], known)
		if routes == nil {
			// 两档都没命中 —— 现在才值得花那一次调用。
			routes = a.routeByModel(ctx, userID, k, evidenceOf[k.Norm])
		}
		for _, rt := range routes {
			if err := a.d.Queries.UpsertKeywordDiscipline(ctx, sqlc.UpsertKeywordDisciplineParams{
				KeywordID: k.ID, DisciplineID: rt.DisciplineID,
				Confidence: rt.Confidence, How: rt.How, Rationale: rt.Rationale,
			}); err != nil {
				slog.Warn("interest: upsert edge failed", "err", err, "keyword_id", k.ID)
			}
		}
	}
}

// routeByModel 是 T3 档。失败一律返回 nil：这个词暂时没有学科，下次采集再试。
// 绝不返回一个猜的学科 —— 一条编出来的「你的兴趣属于艺术史」比没有边伤害大得多。
func (a *API) routeByModel(
	ctx context.Context, userID uuid.UUID, k sqlc.InterestKeyword, evidence string,
) []interest.Route {
	// ClassReflex 的定义就是「一次路由选择，没有自由文本」——正是这件事。
	resolved, ok := a.route(ctx, gateway.ClassReflex)
	if !ok {
		slog.Warn("interest: no provider resolved for routing", "keyword_id", k.ID)
		return nil
	}
	system, user := interest.BuildRoutePrompt(k.TextZh, evidence, k.Field)
	res, err := gateway.Collect(ctx, a.d.Provider, resolved, gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: system},
			{Role: gateway.RoleUser, Content: user},
		},
	})
	a.recordLiteLLMCall(ctx, userID, uuid.Nil, "interest_route", resolved, res.Usage)
	if err != nil {
		slog.Warn("interest: routing call failed", "err", err, "keyword_id", k.ID)
		return nil
	}
	routes, perr := interest.ParseRouteReply(res.Text, k.Field)
	if perr != nil {
		slog.Warn("interest: unparseable routing reply", "err", perr, "keyword_id", k.ID)
		return nil
	}
	return routes
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
	ID          string                  `json:"id"`
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
//
// 三张表各查一次，Go 侧按 keyword_id 分组。每个词发一次查询，会在一棵二十个词
// 的树上变成四十次往返。
func (a *API) getInterestTree(w http.ResponseWriter, r *http.Request) {
	u, ok := UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, r, httpx.ErrUnauthorized("未登录"))
		return
	}
	ctx := r.Context()

	keywords, err := a.d.Queries.ListInterestKeywords(ctx, u.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	sources, err := a.d.Queries.ListKeywordSourcesForUser(ctx, u.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	edges, err := a.d.Queries.ListKeywordDisciplinesForUser(ctx, u.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
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
			ID: k.ID.String(), TextZh: k.TextZh, TextEn: k.TextEn, Field: k.Field,
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

	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"fields":   fields,
		"keywords": out,
	})
}
