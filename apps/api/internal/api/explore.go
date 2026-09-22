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
	"mindimprint/api/internal/materialize"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/disciplines"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/httpx"
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

// studentZone 是学生所在的时区。
//
// 🚨 **写死东八区，不用 time.Local，也不靠 TZ 环境变量。** 两个理由：
//
//  1. api 跑在 distroless 镜像里，镜像里**没有 tzdata**，compose 里也没设 TZ。
//     于是 `time.Local` 是 UTC —— 2026-09-10 在生产上量到的：星图的「今天」是
//     一个 UTC 日，凌晨到早上八点之间，学生看到的仍然是昨天那五颗星，而地图上
//     写着「今日探索地图」。一天一屏的产品，换屏的时刻不能是早上八点。
//  2. 用固定偏移而不是 LoadLocation("Asia/Shanghai")：后者在没有 tzdata 的镜像
//     里会失败，而失败的样子是悄悄退回 UTC —— 正是我们要修的那个 bug。
//     东八区没有夏令时，固定偏移就是完整的答案。
//
// 这条产品线是中国的（迁移 0120 的注释：「学生和服务器都在东八区」）。将来真要
// 跨时区，改的是「按谁的时区算」，不是这个常量。
var studentZone = time.FixedZone("CST", 8*60*60)

// starmapDay 把一个时刻折算成星图的「那一天」。
//
// 纯函数，因为这是这条路上唯一「读代码看不出对错」的判断：它判错的样子不是报错，
// 是学生在早上七点看到昨天的新闻。
func starmapDay(now time.Time) pgtype.Date {
	d := now.In(studentZone)
	return pgtype.Date{
		Time:  time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC),
		Valid: true,
	}
}

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
	Hook string `json:"hook"`
	/**
	 * Hook 出自正文的哪一句，逐字照抄（原文语言）。写入前已验过它确实出现在
	 * 正文里 —— 界面把它摆在问题旁边，她自己就能判断这个问题问得对不对。
	 * 空串 = 2026-09-10 之前生成的旧行，界面照常显示，只是没有出处。
	 */
	Evidence    string                `json:"evidence"`
	URL         string                `json:"url"`
	Source      string                `json:"source"`
	Field       string                `json:"field"`
	Keyword     string                `json:"keyword"`
	Discipline  *exploreDisciplineDTO `json:"discipline"`
	Saved       bool                  `json:"saved"`
	PublishedAt string                `json:"publishedAt"`
	// InterestID 是这颗星落在领域词表里的哪一条。地图靠它和 DisciplineID 把
	// 五颗星连到她树上已经有的词上（`explore/skyLayout.ts` 的 planetThreads）。
	InterestID string `json:"interestId"`
	// ReadingID 是她收下这颗星时落进阅读室的那一篇。空 = 还没收，或者是 0138
	// 之前收的老行。
	ReadingID string `json:"readingId"`
	// Finished 说的是那一篇**真的读完了**（reading.status = 'finished'）。
	// 和 Saved 分开，因为地图上只有一个标记的时候，「在阅读室里」会被读成
	// 「读完了」—— 2026-09-08 产品负责人点了「现在读」就退出来，星球上出现一个
	// 绿色对勾，她读到的是「这条我已经读完了」。
	Finished bool `json:"finished"`
}

// savedPlanet 是「这颗星她收过」这件事的两半：落在哪一篇，那一篇读完没有。
type savedPlanet struct {
	reading  uuid.UUID
	finished bool
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

	dayOnly := starmapDay(time.Now())

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
	savedIDs, err := a.d.Queries.ListSavedPlanets(ctx, u.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	saved := make(map[uuid.UUID]savedPlanet, len(savedIDs))
	for _, s := range savedIDs {
		var reading uuid.UUID
		// 可空列在 sqlc 里是 pgtype.UUID，不是 *uuid.UUID —— 两者不能互换。
		if s.ReadingID.Valid {
			reading = uuid.UUID(s.ReadingID.Bytes)
		}
		saved[s.PlanetID] = savedPlanet{reading: reading, finished: s.ReadingStatus == "finished"}
	}

	out := exploreTodayDTO{Day: dayOnly.Time.Format("2006-01-02"), Planets: []planetDTO{}}
	for _, p := range rows {
		s, ok := saved[p.ID]
		out.Planets = append(out.Planets, planetToDTO(p, ok, s.reading, s.finished))
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
// 收一颗星球 = **把这篇放进阅读室**，不是往树上种一个词（2026-09-07 改，
// 迁移 0138）。
//
// 原来这里直接种词，evidence 用这颗星的钩子。产品负责人指出这一步来得太早：
// 她在地图上看到的只有一个标题和两句摘要，一个词就上了树。词该长在她真的读完
// 之后 —— 阅读读完会有报告，报告上再提出候选词让她自己认。
//
// 所以这个端点现在只做一件确定的事：建一篇阅读（标题就是这颗星的标题），把
// 它记在 news_saved 上。「现在读」和「稍后读」调的是同一个端点，差别只在前端
// 之后跳不跳过去 —— 两个动作落到同一篇上，重复点也不会攒出第二篇。
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
	// 已经收过了就把那一篇原样交回去。**幂等在这里不是洁癖**：她点「稍后读」
	// 之后再点「现在读」，要落在同一篇上，否则阅读室里会出现两篇同名的。
	if existing, err := a.d.Queries.GetPlanetSaveReading(ctx, sqlc.GetPlanetSaveReadingParams{
		UserID: u.ID, PlanetID: id,
	}); err == nil {
		var reading uuid.UUID
		finished := false
		if existing.Valid {
			reading = uuid.UUID(existing.Bytes)
			// 她可能早就读完过这一篇。不查一下就回 finished=false，地图上
			// 「已读完」会在她再点一次的瞬间掉回「在阅读室」。
			if rd, rerr := a.d.Queries.GetReading(ctx, reading); rerr == nil {
				finished = rd.Status == "finished"
			}
		}
		httpx.WriteJSON(w, http.StatusOK, planetToDTO(p, true, reading, finished))
		return
	} else if !errors.Is(err, pgx.ErrNoRows) {
		httpx.WriteError(w, r, err)
		return
	}

	reading, err := a.mintReadingForPlanet(ctx, u.ID, p)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := a.d.Queries.SavePlanet(ctx, sqlc.SavePlanetParams{
		UserID: u.ID, PlanetID: id, ReadingID: pgtype.UUID{Bytes: reading, Valid: true},
	}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	// 刚建出来的那一篇，一个字都还没读。
	httpx.WriteJSON(w, http.StatusOK, planetToDTO(p, true, reading, false))
}

// planetFetchMinRunes 是「**抓回来的**这份算不算正文」。
//
// 和 news.GroundText 用的是同一个数（internal/news/write.go 的 groundMinRunes），
// 同一个理由：抓一个页面抓回来的东西可能是导航条、Cookie 提示、付费墙那一屏 ——
// 六百字以下的那份多半不是文章。
//
// 🚨 这个门槛**只管抓回来的那一份，不管 feed 自带的那一份**。content:encoded
// 是出版方自己填的正文字段，它就是这篇文章；拿六百字去卡它，会把一篇真的很短的
// 报道判成「只有摘要」，然后在一篇完整的文章下面摆一条「我们无法直接获取正文」。
// feed 没带正文的那些源，这一列本来就是空串（实测 Aeon / Psyche / ScienceDaily
// / Phys.org 都是），所以「非空 = 有正文」这条判据在真实数据上是准的。
const planetFetchMinRunes = 600

// planetArticle 是一颗星球最终落进阅读室的那份文本。
type planetArticle struct {
	Body string
	// ExcerptOnly —— 这里放的只是摘要/导语。阅读室据此摆出「跳转原网站」那一条。
	ExcerptOnly bool
}

// resolvePlanetArticle 决定她点开这颗星之后读到的是什么。
//
// 三条路，按可信度排（和 news.GroundText 同一套顺序，同一个理由）：
//
//  1. feed 自己带的正文（0141 存下来的那一列）—— 免费、不会被挡。
//  2. 现去抓一次原页面 —— 有的站行，有的 403。
//  3. feed 的导语 —— 这就是「只有摘要」那种情况。
//
// 🚨 第三条**不是失败**，它是一个要说出来的状态。2026-09-16 之前这里只走第一条，
// 走不通就留一篇没有正文的空阅读，由前端另开一页原文让她自己粘。产品负责人把
// 那个自动跳转否掉了，于是「只有摘要」第一次需要在数据里留下痕迹 —— 否则阅读室
// 手上只有一段文字，分不出它是一整篇还是一段导语。
//
// 🚨 抓取在事务**外面**做。一次跨网的 HTTP 请求最长十二秒，把它关在事务里就是
// 让一个数据库连接跟着它一起等。
func (a *API) resolvePlanetArticle(ctx context.Context, p sqlc.NewsPlanet) planetArticle {
	// 🚨 feed 自带的 body **也要够长才算正文**。
	//
	// 2026-09-17 之前这里只看非空。有的源（Quanta 那一条）在 body 里放的就是
	// 摘要加一行「Source」，一共 403 个字符 —— 她打开就在两段话上开始「通读」，
	// 屏幕上没有任何提醒。第二条路（现抓）早就有 planetFetchMinRunes 这道线，
	// 第一条路没有。短的那一份不丢：它往下走，抓不到原页面时它就是那段摘要。
	// feed 自带的正文（content:encoded）也会带着网站的「推荐阅读」尾巴，和抓回来的
	// 页面一样切掉（materialize.CutRelatedTrailer，2026-09-17）。
	feedBody := materialize.CutRelatedTrailer(strings.TrimSpace(p.Body))
	if len([]rune(feedBody)) >= planetFetchMinRunes {
		return planetArticle{Body: feedBody}
	}
	// 🚨 这里抓的 URL 来自我们自己那张源表挑出来的那一条，不是学生贴的、更不是
	// 模型挑的。Fetcher 那道 SSRF 守卫照旧生效。
	if a.d.Fetcher != nil && strings.TrimSpace(p.Url) != "" {
		fctx, cancel := context.WithTimeout(ctx, articleFetchTimeout)
		_, text, _, ferr := a.d.Fetcher.FetchReadable(fctx, p.Url)
		cancel()
		if ferr != nil {
			// 不是错误，是第二条路没走通。第三条还在。
			slog.Info("explore: 这一篇的原页面抓不到，退回摘要", "err", ferr, "url", p.Url)
		} else if fetched := strings.TrimSpace(text); len([]rune(fetched)) >= planetFetchMinRunes {
			return planetArticle{Body: fetched}
		}
	}
	// 三条路都没走通。剩下的是一段摘要 —— feed 那份短 body 和导语，哪个长用哪个。
	excerpt := strings.TrimSpace(p.Summary)
	if len([]rune(feedBody)) > len([]rune(excerpt)) {
		excerpt = feedBody
	}
	return planetArticle{Body: excerpt, ExcerptOnly: true}
}

// mintReadingForPlanet 为一颗星球建一篇阅读。
//
// atom + reading 一个事务，理由和 createReading 那边一样：一个没有 reading 行
// 的 atom 是一个渲染不出来的身份。正文由 resolvePlanetArticle 在事务外面定下来
// —— 抓得到就是正文，抓不到就是那段摘要加一条「跳转原网站」。
func (a *API) mintReadingForPlanet(ctx context.Context, userID uuid.UUID, p sqlc.NewsPlanet) (uuid.UUID, error) {
	article := a.resolvePlanetArticle(ctx, p)

	tx, err := a.d.Pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := a.d.Queries.WithTx(tx)

	at, err := qtx.CreateAtom(ctx, sqlc.CreateAtomParams{Kind: "reading", UserID: userID})
	if err != nil {
		return uuid.Nil, err
	}
	title := p.TitleZh
	if len([]rune(title)) > 200 {
		title = string([]rune(title)[:200])
	}
	if _, err := qtx.CreateReading(ctx, sqlc.CreateReadingParams{
		AtomID: at.ID, Title: title, Lang: "zh",
	}); err != nil {
		return uuid.Nil, err
	}
	// 正文（或那段摘要）就在这里落进 reading_source —— 她点开就是一篇能读的
	// 东西，没有粘贴框那一屏。
	//
	// 这和她自己粘进来走的是同一张表、同一个形状（纯文本、空行分段），所以
	// 阅读室那边什么都不用改：SplitBlocks 照常切块，段落工具条、挂卡片、精读
	// 段落全都照常。excerpt_only 是唯一多出来的那一位。
	//
	// 存不进去不算失败：她自己粘那条路还在，回滚整篇反而把一篇本来能读的文章
	// 弄没了。
	if article.Body != "" {
		if _, err := qtx.UpsertReadingSource(ctx, sqlc.UpsertReadingSourceParams{
			AtomID: at.ID, Title: title, Body: article.Body,
			SourceUrl: nullableText(p.Url), ExcerptOnly: article.ExcerptOnly,
		}); err != nil {
			slog.Warn("explore: 星球正文没能落进阅读室", "err", err, "atom_id", at.ID)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, err
	}
	return at.ID, nil
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
			Evidence: p.Evidence,
			Field:    p.Field, DisciplineID: p.DisciplineID, InterestID: interestIDArg(p.InterestID),
			PublishedAt: published,
			// feed 自己带的正文（迁移 0141）。取正文的第三条路：这一份已经在
			// 手上了，她点进去就能读，不必再抓一次原页面、也不必让她粘。
			// 源的 feed 没带正文时是空串，另外两条路照旧。
			Body: src.Body,
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

// buildStarmap 抓 → 过滤 → 选 → 照着正文写。返回 (星球, 失败原话)。
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
		return nil, fmt.Sprintf("抓取失败：当前获取到 %d 条候选新闻，需要至少 %d 条。", len(pool), news.PlanetCount)
	}
	picked, note := a.selectPlanets(ctx, resolved, pool)
	if note != "" {
		return nil, note
	}
	written := a.writePlanets(ctx, resolved, picked)
	if len(written) == 0 {
		return nil, "生成失败：未生成可用的新闻内容：未获取到正文，或模型引用未通过原文核验。"
	}
	return written, ""
}

// selectPlanets 是第一步：从候选池里挑出今天的那几条，并给每一条归类。
//
// 这一步**不产出任何给学生看的字**，所以它的回复很短，被截断的风险比上一版低
// 一个数量级。
func (a *API) selectPlanets(ctx context.Context, resolved gateway.Resolved, pool []news.Item) ([]plannedPlanet, string) {
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

// articleFetchTimeout 是单篇正文抓取的上限。
//
// 12 秒。抓不到就退回 feed 自带的那份，所以这里宁可放弃得早一点 —— 五篇并行，
// 最慢的那篇决定学生等多久。
const articleFetchTimeout = 12 * time.Second

// writePlanets 是第二步：把每一条的正文抓回来，照着正文写。
//
// **并行，一条一次调用。** 两个理由，都不是为了快：
//
//   - 一条写坏只丢一条。上一版一次调用写五条，回复被截断过一次，那一整天就
//     没有星图（select.go 里那段 salvage 就是为它加的）。
//   - 每次调用的上下文里只有一篇文章，模型没有机会把 A 篇的争议按到 B 篇上。
//
// 返回的条数可能少于传进来的（正文抓不到、出处对不上就丢），所以选星那一步多
// 挑了 news.WriteMargin 条。最后截到 PlanetCount。
func (a *API) writePlanets(ctx context.Context, resolved gateway.Resolved, picked []plannedPlanet) []plannedPlanet {
	type result struct {
		p  plannedPlanet
		ok bool
	}
	results := make([]result, len(picked))
	var wg sync.WaitGroup
	for i, pp := range picked {
		wg.Add(1)
		go func(i int, pp plannedPlanet) {
			defer wg.Done()
			src := news.Item{}
			if pp.Index >= 0 && pp.Index < len(pp.candidates) {
				src = pp.candidates[pp.Index]
			}
			w, err := a.writeOnePlanet(ctx, resolved, src)
			if err != nil {
				slog.Warn("explore: could not write this planet from its article",
					"err", err, "source", src.Source, "title", src.Title)
				return
			}
			pp.Written = w
			results[i] = result{p: pp, ok: true}
		}(i, pp)
	}
	wg.Wait()

	out := make([]plannedPlanet, 0, news.PlanetCount)
	for _, r := range results {
		if !r.ok {
			continue
		}
		out = append(out, r.p)
		if len(out) == news.PlanetCount {
			break
		}
	}
	return out
}

// writeOnePlanet 抓一篇的正文，让模型照着它写。
//
// 校验与重试都在 news.WriteOne 里 —— 这里只负责「怎么问模型」。这么分是为了让
// LIVE_LLM 那个用例跑的是同一段循环：一个只在生产里跑的重试逻辑，等于没被测过。
func (a *API) writeOnePlanet(ctx context.Context, resolved gateway.Resolved, src news.Item) (news.Written, error) {
	fetched := ""
	// 🚨 这里抓的 URL 来自**我们自己那张源表**（internal/news/sources.go），不是
	// 学生贴的、更不是模型挑的。Fetcher 那道 SSRF 守卫照旧生效，只是这一次
	// 「谁决定抓哪个地址」的答案是我们。
	if a.d.Fetcher != nil && src.Link != "" {
		fctx, cancel := context.WithTimeout(ctx, articleFetchTimeout)
		_, text, _, ferr := a.d.Fetcher.FetchReadable(fctx, src.Link)
		cancel()
		if ferr != nil {
			// 不是错误，是三条路里的第一条没走通。剩下两条照旧。
			slog.Info("explore: article body not fetchable, falling back to the feed",
				"source", src.Source, "err", ferr)
		}
		fetched = text
	}
	ground := news.GroundText(fetched, src)
	if strings.TrimSpace(ground) == "" {
		return news.Written{}, fmt.Errorf("这一条既抓不到正文，feed 里也没有摘要")
	}

	return news.WriteOne(src, ground, func(system string, turns []news.Turn) (string, error) {
		if len(turns) > 1 {
			// 第一版没过校验，这是带着理由的那次重写。记下来 —— 重写的比例高了，
			// 说明 prompt 或者哪道校验该改。
			slog.Info("explore: rewriting this planet after the first version did not check out",
				"source", src.Source)
		}
		msgs := []gateway.ChatMessage{{Role: gateway.RoleSystem, Content: system}}
		for _, t := range turns {
			msgs = append(msgs, gateway.ChatMessage{Role: t.Role, Content: t.Content})
		}
		res, cerr := gateway.Collect(ctx, a.d.Provider, resolved, gateway.ChatRequest{
			Messages:       msgs,
			ResponseFormat: gateway.ResponseFormatJSONObject,
		})
		a.recordLiteLLMCall(ctx, uuid.Nil, uuid.Nil, "news_planet_write", resolved, res.Usage)
		if cerr != nil {
			return "", cerr
		}
		return res.Text, nil
	})
}

/* ── 转换 ───────────────────────────────────────────────────────────────── */

func planetToDTO(p sqlc.NewsPlanet, saved bool, readingID uuid.UUID, finished bool) planetDTO {
	dto := planetDTO{
		ID: p.ID.String(), Rank: int(p.Rank),
		TitleZh: p.TitleZh, TitleEn: p.TitleEn, Summary: p.Summary, Hook: p.Hook,
		Evidence: p.Evidence,
		URL:      p.Url, Source: p.SourceName, Field: p.Field, Keyword: planetKeywordZh(p.InterestID),
		Saved: saved, Finished: finished, InterestID: derefString(p.InterestID),
	}
	if readingID != uuid.Nil {
		dto.ReadingID = readingID.String()
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
