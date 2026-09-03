package api

// explore.go —— 今日新闻星图。
//
//	GET  /api/v1/explore/today            今天的五颗星（没有就现生成）
//	POST /api/v1/explore/planets/{id}/save 收藏一颗星 → 种进她的树
//
// # 生成为什么是惰性的
//
// 和兴趣采集同一个理由：**这个 repo 里没有 river worker。** 迁移建过 river 的表，
// 但没有 client、没有 worker，所有 `go func()` 不是 SSE 心跳就是同请求内的
// WaitGroup 汇合。一个「每天早上六点跑一次」的任务需要一个跑它的东西，而那个
// 东西还不存在。
//
// 所以：**第一个打开星图的学生触发生成**，拿一把 advisory lock，其余人等它
// 提交后直接读。和 atom_report 的首次生成是同一个套路。P5 之后这块该变成一个
// 入队任务。
//
// # 「今天试过一次」不是「今天出过星图」
//
// `news_day` 的章**盖在生成之前**。所有源都挂掉的那天，产出是零颗星，而零颗星
// 和「今天从没试过」在 news_planet 里长得一模一样 —— 按产出判断，每个打开星图的
// 学生都会再触发一次全量抓取加一次旗舰调用。这条教训第三次出现了
// （reading.questions_at 0104、atom.interest_harvested_at 0116）。
//
// # 绝不拿昨天的冒充今天的
//
// 生成失败时这一屏是空的，并且说出后台原话。一个显示着昨天五条新闻、标题写着
// 「今日」的星图，是这个产品最不该撒的那类谎。

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/disciplines"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/interest"
	"mindimprint/api/internal/news"
	"mindimprint/api/internal/store/sqlc"
)

// freshWindow 是「多新算今天的」。
//
// 72 小时而不是 24：源的时区五花八门，周末很多期刊不发，而一篇周五发的 Nature
// 论文在周一依然是新闻。窗口太窄的直接后果是周一的星图凑不满五颗。
const freshWindow = 72 * time.Hour

/* ── DTO ────────────────────────────────────────────────────────────────── */

type planetDTO struct {
	ID      string `json:"id"`
	Rank    int    `json:"rank"`
	TitleZh string `json:"titleZh"`
	TitleEn string `json:"titleEn"`
	Summary string `json:"summary"`
	/** 她能自己追问的那个问题。永远非空。 */
	Hook       string `json:"hook"`
	URL        string `json:"url"`
	Source     string `json:"source"`
	Field      string `json:"field"`
	Keyword    string `json:"keyword"`
	Discipline *exploreDisciplineDTO `json:"discipline"`
	Saved      bool   `json:"saved"`
	PublishedAt string `json:"publishedAt"`
}

type exploreDisciplineDTO struct {
	ID   string `json:"id"`
	Zh   string `json:"zh"`
	En   string `json:"en"`
	Asks string `json:"asks"`
}

type exploreTodayDTO struct {
	Day     string      `json:"day"`
	Planets []planetDTO `json:"planets"`
	/**
	 * 生成失败时的后台原话。非空表示今天这一屏是空的，而且我们知道为什么。
	 * 界面照原样显示（界面文案 §8）—— 绝不用昨天的顶上。
	 */
	Note string `json:"note"`
}

/* ── 端点 ───────────────────────────────────────────────────────────────── */

// getExploreToday —— GET /api/v1/explore/today
func (a *API) getExploreToday(w http.ResponseWriter, r *http.Request) {
	u, ok := UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, r, httpx.ErrUnauthorized("未登录"))
		return
	}

	day := time.Now().In(time.Local)
	dayOnly := pgtype.Date{Time: time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.UTC), Valid: true}

	// 生成用脱离请求生命周期的 context：她如果在等待时切走，已经花掉的抓取与
	// 模型调用不该被取消 —— 星图照样出，下一个人（或者她自己）立刻就看得到。
	gCtx, cancel := detachedModelCtx(r)
	defer cancel()
	a.ensureTodayStarmap(gCtx, dayOnly)

	ctx := r.Context()
	rows, err := a.d.Queries.ListNewsPlanets(ctx, dayOnly)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	savedIDs, err := a.d.Queries.ListSavedPlanetIDs(ctx, u.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	saved := make(map[uuid.UUID]bool, len(savedIDs))
	for _, id := range savedIDs {
		saved[id] = true
	}

	out := exploreTodayDTO{Day: dayOnly.Time.Format("2006-01-02"), Planets: []planetDTO{}}
	for _, p := range rows {
		out.Planets = append(out.Planets, planetToDTO(p, saved[p.ID]))
	}
	// 一屏都没有时，把「为什么」一起给出去。
	if len(out.Planets) == 0 {
		if d, err := a.d.Queries.GetNewsDay(ctx, dayOnly); err == nil {
			out.Note = d.Note
		}
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

// savePlanet —— POST /api/v1/explore/planets/{id}/save
//
// 收藏 = **把这颗星加到我的树上**。它走 plantKeywords 的同一条路
// （`kind="news"`），evidence 用这颗星的钩子 —— 那句话是她按下收藏时看着的
// 那个问题，所以它有资格作为「这个词为什么在你树上」的答案。
func (a *API) savePlanet(w http.ResponseWriter, r *http.Request) {
	u, ok := UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, r, httpx.ErrUnauthorized("未登录"))
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_planet_id", "星球 id 无效", nil))
		return
	}
	ctx := r.Context()

	p, err := a.d.Queries.GetNewsPlanet(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteError(w, r, httpx.ErrNotFound("这颗星球不存在"))
			return
		}
		httpx.WriteError(w, r, err)
		return
	}
	if err := a.d.Queries.SavePlanet(ctx, sqlc.SavePlanetParams{UserID: u.ID, PlanetID: id}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	// 有关键词才种。没有关键词的星球照样能收藏 —— 收藏这件事本身已经记下了。
	if p.Keyword != "" && p.Hook != "" {
		a.plantKeywords(ctx, u.ID, "news", p.ID, p.TitleZh, []interest.Harvested{{
			TextZh:   p.Keyword,
			TextEn:   "",
			Field:    p.Field,
			Note:     p.Summary,
			Evidence: p.Hook,
		}})
	}
	httpx.WriteJSON(w, http.StatusOK, planetToDTO(p, true))
}

/* ── 生成 ───────────────────────────────────────────────────────────────── */

// ensureTodayStarmap 保证今天这一屏已经试过生成一次。
//
// 幂等：advisory lock + 「今天试过没有」的复查。两个学生在同一秒第一次打开，
// 只有一次抓取和一次模型调用。
func (a *API) ensureTodayStarmap(ctx context.Context, day pgtype.Date) {
	// 先看一眼，绝大多数请求在这里就返回了 —— 不必为了读五行去拿锁。
	if _, err := a.d.Queries.GetNewsDay(ctx, day); err == nil {
		return
	}

	tx, err := a.d.Pool.Begin(ctx)
	if err != nil {
		slog.Warn("explore: begin failed", "err", err)
		return
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()

	key := "news-starmap:" + day.Time.Format("2006-01-02")
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, key); err != nil {
		slog.Warn("explore: advisory lock failed", "err", err)
		return
	}
	qtx := a.d.Queries.WithTx(tx)

	// 拿到锁之后**再看一次**：等锁的那个人可能刚刚生成完。
	if _, err := qtx.GetNewsDay(ctx, day); err == nil {
		return
	}
	// 🚨 盖章在生成之前。见文件头。
	if err := qtx.MarkNewsDayAttempted(ctx, day); err != nil {
		slog.Warn("explore: stamp failed", "err", err)
		return
	}

	planets, note := a.buildStarmap(ctx)
	for i, p := range planets {
		src := news.Item{}
		if p.Index >= 0 && p.Index < len(p.candidates) {
			src = p.candidates[p.Index]
		}
		published := pgtype.Timestamptz{}
		if !src.Published.IsZero() {
			published = pgtype.Timestamptz{Time: src.Published, Valid: true}
		}
		if _, err := qtx.InsertNewsPlanet(ctx, sqlc.InsertNewsPlanetParams{
			Day: day, Rank: int32(i + 1),
			TitleZh: p.TitleZh, TitleEn: p.TitleEn, Summary: p.Summary, Hook: p.Hook,
			Url: src.Link, SourceName: src.Source,
			Field: p.Field, DisciplineID: p.DisciplineID, Keyword: p.Keyword,
			PublishedAt: published,
		}); err != nil {
			slog.Warn("explore: insert planet failed", "err", err, "rank", i+1)
		}
	}
	if err := qtx.FinishNewsDay(ctx, sqlc.FinishNewsDayParams{
		Day: day, PlanetCount: int32(len(planets)), Note: note,
	}); err != nil {
		slog.Warn("explore: finish day failed", "err", err)
	}
	if err := tx.Commit(ctx); err != nil {
		slog.Warn("explore: commit failed", "err", err)
	}
}

// plannedPlanet 是一颗星加上它来自的候选池（为了取回链接、出处、时间）。
type plannedPlanet struct {
	news.Planet
	candidates []news.Item
}

// buildStarmap 抓 → 过滤 → 选五颗。返回 (星球, 失败原话)。
//
// 任何一步失败都返回零颗星加一句原话。**绝不返回昨天的，也绝不编五条新闻。**
func (a *API) buildStarmap(ctx context.Context) ([]plannedPlanet, string) {
	// 🚨 先看通道，**再**抓。反过来的话，一个没配模型的环境（测试就是）会先
	// 打十二个源的十二次 HTTP 请求，然后才发现这些候选没人能用。抓取是这条
	// 路径上唯一昂贵的一步，它应该发生在确定用得上之后。
	if a.d.Provider == nil {
		return nil, "生成失败：模型通道未配置。"
	}
	resolved, ok := a.route(ctx, gateway.ClassDigest)
	if !ok {
		return nil, "生成失败：没有可用的模型通道。"
	}
	pool := news.NewFetcher().FetchAll(ctx, freshWindow)
	if len(pool) < news.PlanetCount {
		return nil, fmt.Sprintf("抓取失败：今天只凑到 %d 条候选，不足 %d 条。", len(pool), news.PlanetCount)
	}
	// 🚨 用 BuildSelectPrompt 交回来的那批候选去解析，**不是完整的 pool**：
	// prompt 里只描述了前 maxCandidates 条，下标必须按同一个切片解释。
	system, user, candidates := news.BuildSelectPrompt(pool)
	res, cerr := gateway.Collect(ctx, a.d.Provider, resolved, gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: system},
			{Role: gateway.RoleUser, Content: user},
		},
	})
	// 这次调用不属于任何学生，也不属于任何 atom：它是一天一次的公共开销。
	a.recordLiteLLMCall(ctx, uuid.Nil, uuid.Nil, "news_starmap", resolved, res.Usage)
	if cerr != nil {
		return nil, "生成失败：" + cerr.Error()
	}
	picked, perr := news.ParseSelectReply(res.Text, candidates)
	if perr != nil {
		return nil, "生成失败：" + perr.Error()
	}
	out := make([]plannedPlanet, 0, len(picked))
	for _, p := range picked {
		out = append(out, plannedPlanet{Planet: p, candidates: candidates})
	}
	return out, ""
}

/* ── 转换 ───────────────────────────────────────────────────────────────── */

func planetToDTO(p sqlc.NewsPlanet, saved bool) planetDTO {
	dto := planetDTO{
		ID: p.ID.String(), Rank: int(p.Rank),
		TitleZh: p.TitleZh, TitleEn: p.TitleEn, Summary: p.Summary, Hook: p.Hook,
		URL: p.Url, Source: p.SourceName, Field: p.Field, Keyword: p.Keyword,
		Saved: saved,
	}
	if p.PublishedAt.Valid {
		dto.PublishedAt = p.PublishedAt.Time.Format(time.RFC3339)
	}
	// 学科内容从静态表补齐 —— 库里只存 id（同 interest tree 的做法）。
	if d, ok := disciplines.ByID(p.DisciplineID); ok {
		dto.Discipline = &exploreDisciplineDTO{ID: d.ID, Zh: d.Zh, En: d.En, Asks: d.Asks}
	}
	return dto
}
