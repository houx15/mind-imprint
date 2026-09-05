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
// # 🚨 但盖了章不等于这一天到此为止
//
// 上一版盖完章就再也不试了，于是**一次失败锁死一整天**：上游一个 503、模型回了
// 一段不是 JSON 的话，这一天的星图就永远是空的，而界面上那个「重试」按钮按下去
// 什么都不会发生 —— 请求照发，服务端在第一行 GetNewsDay 就返回了。2026-09-04 的
// 模拟学生走查里，六次启动撞上两次；对一个四天的营来说，一整天的探索面没了。
//
// 所以章改成计次（migration 0135）：失败的那天允许再试，`starmapRetryable` 定
// 什么时候、还剩几次。成功的那天 planet_count > 0，一次都不会再试。冷却间隔是
// 为了让它跨过一次短暂的上游故障，不是为了拖住她 —— 所以界面把还要等几秒说出来，
// 而不是留一个按下去没反应的按钮。
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
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/disciplines"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/interest"
	"mindimprint/api/internal/interests"
	"mindimprint/api/internal/news"
	"mindimprint/api/internal/store/sqlc"
)

// freshWindow 是「多新算今天的」。
//
// 72 小时而不是 24：源的时区五花八门，周末很多期刊不发，而一篇周五发的 Nature
// 论文在周一依然是新闻。窗口太窄的直接后果是周一的星图凑不满五颗。
const freshWindow = 72 * time.Hour

// 失败之后再试几次、隔多久。
//
// 8 次 × 90 秒：抓取加一次 digest 调用是这条路上唯一的开销，一天封顶八次是可以
// 忽略的钱；而 90 秒足够跨过上游的一次抖动，又不至于让她盯着屏幕干等。两个数
// 一起是「一整天不会白丢，也不会被一个坏掉的源刷爆」。
const (
	maxStarmapAttempts  = 8
	starmapRetryCoolOff = 90 * time.Second
)

// clipRunes 截一段模型输出，用来放进日志。
//
// 600 字够看出它是写跑题了还是被截断了，又不至于把一整屏候选新闻灌进日志。
func clipRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// starmapRetryable 说的是：这一天还该不该再试一次生成。
//
// 纯函数，因为这是这条路上唯一「读代码看不出对错」的判断，而它判错的两种方式
// 都很贵：判紧了，一整天的探索面没了；判松了，一个坏掉的源会被反复抓。
//
// now 由调用方传，测试才不用等 90 秒。
func starmapRetryable(d sqlc.NewsDay, now time.Time) bool {
	if d.PlanetCount > 0 {
		return false // 今天出过星图，这一天就结束了。
	}
	if d.Attempts >= maxStarmapAttempts {
		return false
	}
	return now.Sub(d.AttemptedAt) >= starmapRetryCoolOff
}

// starmapRetryAfter 是「还要等几秒才能再试」。
//
// 0 = 现在就能试；-1 = 今天不再试了。界面照这个数决定那个按钮的样子 —— 一个按
// 下去没反应的按钮，比没有按钮糟得多。
func starmapRetryAfter(d sqlc.NewsDay, now time.Time) int {
	if d.PlanetCount > 0 || d.Attempts >= maxStarmapAttempts {
		return -1
	}
	left := starmapRetryCoolOff - now.Sub(d.AttemptedAt)
	if left <= 0 {
		return 0
	}
	return int(left.Seconds()) + 1
}

/* ── DTO ────────────────────────────────────────────────────────────────── */

type planetDTO struct {
	ID      string `json:"id"`
	Rank    int    `json:"rank"`
	TitleZh string `json:"titleZh"`
	TitleEn string `json:"titleEn"`
	Summary string `json:"summary"`
	/** 她能自己追问的那个问题。永远非空。 */
	Hook        string                `json:"hook"`
	URL         string                `json:"url"`
	Source      string                `json:"source"`
	Field       string                `json:"field"`
	Keyword     string                `json:"keyword"`
	Discipline  *exploreDisciplineDTO `json:"discipline"`
	Saved       bool                  `json:"saved"`
	PublishedAt string                `json:"publishedAt"`
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
	/**
	 * 还要等几秒才能再试一次。0 = 现在就能试，-1 = 今天不再试了。
	 * 空着的那一屏靠这个数决定「重试」按钮的样子。
	 */
	RetryAfter int `json:"retryAfter"`
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
	// 一屏都没有时，把「为什么」和「还能不能再试」一起给出去。
	if len(out.Planets) == 0 {
		out.RetryAfter = -1
		if d, err := a.d.Queries.GetNewsDay(ctx, dayOnly); err == nil {
			out.Note = d.Note
			out.RetryAfter = starmapRetryAfter(d, time.Now())
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

	// 有领域才种。挑不出领域的星球照样能收藏 —— 收藏这件事本身已经记下了。
	if p.InterestID != nil && *p.InterestID != "" && p.Hook != "" {
		a.plantKeywords(ctx, u.ID, "news", p.ID, p.TitleZh, []interest.Harvested{{
			InterestID: *p.InterestID,
			Note:       p.Summary,
			Evidence:   p.Hook,
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
	if d, err := a.d.Queries.GetNewsDay(ctx, day); err == nil && !starmapRetryable(d, time.Now()) {
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

	// 拿到锁之后**再看一次**：等锁的那个人可能刚刚生成完，或者刚刚用掉这一轮
	// 的那次重试。
	if d, err := qtx.GetNewsDay(ctx, day); err == nil && !starmapRetryable(d, time.Now()) {
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
			Field: p.Field, DisciplineID: p.DisciplineID, InterestID: interestIDArg(p.InterestID),
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
		// 🚨 这一屏一整天只生成一次，解析失败一次就没有星图了。要得到 JSON 就
		// 直说要 JSON —— sliceJSONObject 那层容错留着，但它是兜底，不该是第一
		// 道防线。（DashScope 走 OpenAI 兼容口，认这个字段。）
		ResponseFormat: gateway.ResponseFormatJSONObject,
	})
	// 这次调用不属于任何学生，也不属于任何 atom：它是一天一次的公共开销。
	a.recordLiteLLMCall(ctx, uuid.Nil, uuid.Nil, "news_starmap", resolved, res.Usage)
	if cerr != nil {
		return nil, "生成失败：" + cerr.Error()
	}
	picked, perr := news.ParseSelectReply(res.Text, candidates)
	if perr != nil {
		// 🚨 把模型原样回的那段记下来。上一版只把一句 "unexpected end of JSON
		// input" 放进响应体，slog 里一个字都没有——线上出了这件事根本查不出模型
		// 到底写了什么，而这一屏一天只生成一次，错过就要等明天。
		slog.Warn("explore: could not read the select reply",
			"err", perr, "model", resolved.ModelID,
			"reply", clipRunes(res.Text, 600))
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
		URL: p.Url, Source: p.SourceName, Field: p.Field, Keyword: planetKeywordZh(p.InterestID),
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

// planetKeywordZh 把星球上的领域 id 翻成显示用的中文名。
//
// 接口里这个字段仍然叫 keyword，因为前端拿它写「会在你的树上加一个词：「游戏」」
// —— 学生看的是那个词，不是它的 id。空字符串表示这颗星不长词，前端据此把收藏
// 按钮置灰。
func planetKeywordZh(id *string) string {
	if id == nil || *id == "" {
		return ""
	}
	it, ok := interests.ByID(*id)
	if !ok {
		return ""
	}
	return it.Zh
}

// interestIDArg 把空 id 写成 NULL 而不是空字符串，好让「没挑出领域」在库里
// 只有一种表示。
func interestIDArg(id string) *string {
	if strings.TrimSpace(id) == "" {
		return nil
	}
	return &id
}
