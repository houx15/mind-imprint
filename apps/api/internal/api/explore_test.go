package api_test

// explore_test.go — 今日新闻星图的库级测试。
//
// 测的是这一屏最容易静默出错的四件事：
//
//   - 生成失败时**这一屏是空的，并且说得出为什么** —— 绝不拿昨天的冒充今天的。
//   - 「今天试过一次」的章**盖在生成之前**。按产出判断的话，全源挂掉的那天，
//     每个打开星图的学生都会再触发一次全量抓取 + 一次旗舰调用。这条教训第三次
//     出现了（0104 / 0116 / 0120）。
//   - 收藏一颗星真的把词种进了**同一棵树**，evidence 是那颗星的钩子。
//   - 收藏是幂等的：连点两下不会把强度刷上去。

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type exploreResp struct {
	Day        string `json:"day"`
	Note       string `json:"note"`
	RetryAfter int    `json:"retryAfter"`
	Planets    []struct {
		ID         string `json:"id"`
		Rank       int    `json:"rank"`
		TitleZh    string `json:"titleZh"`
		Hook       string `json:"hook"`
		URL        string `json:"url"`
		Source     string `json:"source"`
		Field      string `json:"field"`
		Keyword    string `json:"keyword"`
		Saved      bool   `json:"saved"`
		ReadingID  string `json:"readingId"`
		Discipline *struct {
			ID   string `json:"id"`
			Zh   string `json:"zh"`
			Asks string `json:"asks"`
		} `json:"discipline"`
	} `json:"planets"`
}

func getExplore(t *testing.T, h http.Handler, cookie *http.Cookie) exploreResp {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/explore/today", nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /explore/today = %d; body=%s", rec.Code, rec.Body)
	}
	var out exploreResp
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v; body=%s", err, rec.Body)
	}
	return out
}

// appToday 是**这个应用**眼里的今天，和 getExploreToday 用的是同一个定义。
//
// 🚨 千万不要在这个文件里写 CURRENT_DATE。那是**数据库会话时区**里的今天，
// 而 explore.go 用的是 `time.Now().In(time.Local)` —— 学生的今天。生产代码里
// 一句 CURRENT_DATE 都没有（今天是谁说了算，只有 app 一个出处），所以测试里
// 出现一句就等于凭空造出了第二个互相矛盾的定义。
//
// 两者在一天里的大部分时候是相等的，所以这件事藏得很好：本机 CST（UTC+8）
// 而库跑在 UTC，于是**只有本地午夜到 UTC 午夜之间那 8 小时**不相等。
// 2026-09-04 凌晨这一整组测试就是这样变红的 —— 前一晚同一份代码全绿。
// 一个只在凌晨红、白天自己变绿的测试，比一个一直红的测试坏得多。
func appToday() time.Time {
	n := time.Now().In(time.Local)
	// 和 explore.go 一样：取本地的年月日，再按 UTC 零点封成一个 date。
	return time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC)
}

// seedPlanet 直接写库，绕开抓取与模型 —— 这些测试要验的是读与收藏那一半。
func seedPlanet(t *testing.T, pool *pgxpool.Pool, rank int, interestID, disciplineID string) string {
	t.Helper()
	var id string
	err := pool.QueryRow(t.Context(), `
		INSERT INTO news_planet (day, rank, title_zh, title_en, summary, hook, url,
		                         source_name, field, discipline_id, interest_id, published_at)
		VALUES ($4::date, $1, '深海珊瑚在 30 度水里活下来了', 'Corals survive 30C',
		        '红海北端一片珊瑚在超过白化阈值的水温里没有白化。',
		        '四平方公里的珊瑚，能代表一整片海吗？',
		        'https://example.org/coral', 'Nature', 'science', $2, $3, now())
		RETURNING id::text`, rank, disciplineID, interestID, appToday()).Scan(&id)
	if err != nil {
		t.Fatalf("seed planet: %v", err)
	}
	// 同时盖上「今天试过了」的章，否则读取时会去触发一次真的生成（会打网络）。
	if _, err := pool.Exec(t.Context(),
		`INSERT INTO news_day (day, planet_count) VALUES ($1::date, 1)
		 ON CONFLICT (day) DO NOTHING`, appToday()); err != nil {
		t.Fatalf("seed day: %v", err)
	}
	return id
}

func savePlanetHTTP(h http.Handler, cookie *http.Cookie, id string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/explore/planets/"+id+"/save", nil)
	h.ServeHTTP(rec, withCookie(req, cookie))
	return rec
}

/* ── 生成失败时的姿态 ───────────────────────────────────────────────────── */

// 🚨 没有模型通道时（测试就是这样），这一屏必须是**空的 + 一句原话**。
// 一个显示着昨天五条新闻、标题写着「今日」的星图，是这个产品最不该撒的谎。
func TestExploreToday_EmptyAndHonestWhenGenerationFails(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	got := getExplore(t, h, cookie)

	if len(got.Planets) != 0 {
		t.Fatalf("没有模型通道却出了 %d 颗星", len(got.Planets))
	}
	if got.Note == "" {
		t.Error("空星图没有说出为什么 —— 学生和我们该看到同一句话")
	}
	if got.Day == "" {
		t.Error("没有回今天的日期")
	}
}

// 🚨 第三次出现的同一条教训（0104 reading.questions_at、0116
// atom.interest_harvested_at、这里）。章盖在生成之前，语义是「今天试过一次」。
// 写反了的症状：每次打开星图都重新抓十二个源、重发一次模型调用，而且永远
// 不会被发现，因为界面看起来一样。
func TestExploreToday_StampsTheDayEvenWhenNothingWasGenerated(t *testing.T) {
	h, cookie, _, pool := liteHandler(t)
	getExplore(t, h, cookie)

	var n int
	if err := pool.QueryRow(t.Context(),
		`SELECT count(*) FROM news_day WHERE day = $1::date`, appToday()).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Fatalf("news_day 里有 %d 行，want 1 —— 零产出的一天必须也盖章", n)
	}

	// 再打开一次，冷却还没过去，所以不该有第二次尝试。
	//
	// 🚨 这里看的是 attempts 而不是行数：migration 0135 之后重试是同一行上的
	// 计次，行数永远是 1，光数行数已经证明不了「没有重抓十二个源」。
	getExplore(t, h, cookie)
	var attempts int
	if err := pool.QueryRow(t.Context(),
		`SELECT attempts FROM news_day WHERE day = $1::date`, appToday()).Scan(&attempts); err != nil {
		t.Fatalf("attempts: %v", err)
	}
	if attempts != 1 {
		t.Errorf("冷却期内又试了一次生成：attempts = %d，want 1", attempts)
	}
}

// 🚨 失败的那一天必须还能再试。
//
// 上一版盖完章就再也不试了，于是上游一次 503 就锁死一整天，界面上那个「重试」
// 按下去什么都不会发生。这里验的是接口那一半：空星图要带出「还要等几秒」，
// 界面才有可能诚实。
func TestExploreToday_SaysWhenTheFailedDayCanBeTriedAgain(t *testing.T) {
	h, cookie, _, pool := liteHandler(t)
	got := getExplore(t, h, cookie)

	if len(got.Planets) != 0 {
		t.Fatalf("没有模型通道却出了 %d 颗星", len(got.Planets))
	}
	if got.RetryAfter <= 0 {
		t.Errorf("失败的一天 retryAfter = %d，want > 0 —— 这一天还没试满，应该说得出等多久", got.RetryAfter)
	}

	// 试满之后就不再试了，界面据此把那个按钮撤掉，而不是留一个按不动的。
	if _, err := pool.Exec(t.Context(),
		`UPDATE news_day SET attempts = 99 WHERE day = $1::date`, appToday()); err != nil {
		t.Fatalf("update attempts: %v", err)
	}
	if got := getExplore(t, h, cookie); got.RetryAfter != -1 {
		t.Errorf("试满之后 retryAfter = %d，want -1", got.RetryAfter)
	}
}

/* ── 读 ─────────────────────────────────────────────────────────────────── */

func TestExploreToday_ReturnsSeededPlanetsWithTheDisciplineHydrated(t *testing.T) {
	h, cookie, _, pool := liteHandler(t)
	seedPlanet(t, pool, 1, "climate", "climate-ocean")

	got := getExplore(t, h, cookie)
	if len(got.Planets) != 1 {
		t.Fatalf("回了 %d 颗星", len(got.Planets))
	}
	p := got.Planets[0]
	if p.Hook == "" {
		t.Error("星球没有钩子 —— 那正是这一屏存在的理由")
	}
	if p.URL == "" || p.Source == "" {
		t.Error("星球没有出处或链接")
	}
	if p.Saved {
		t.Error("还没收藏就显示成已收藏")
	}
	// 学科内容从静态表补齐，库里只存 id（同兴趣树的做法）。
	if p.Discipline == nil || p.Discipline.Zh == "" || p.Discipline.Asks == "" {
		t.Errorf("学科没有从静态表补齐：%+v", p.Discipline)
	}
}

func TestExploreToday_DropsAnEdgeToADisciplineThatNoLongerExists(t *testing.T) {
	// 学科表是内容，会改。指向一个已删除 id 的边要整条跳过，而不是发一个只有
	// id 的空壳让前端渲染一张没有名字的卡。
	h, cookie, _, pool := liteHandler(t)
	seedPlanet(t, pool, 1, "climate", "astrology-of-the-ancients")

	got := getExplore(t, h, cookie)
	if got.Planets[0].Discipline != nil {
		t.Errorf("不存在的学科被发出去了：%+v", got.Planets[0].Discipline)
	}
	if got.Planets[0].TitleZh == "" {
		t.Error("为了一条坏边把整颗星丢了")
	}
}

/* ── 收藏 ───────────────────────────────────────────────────────────────── */

func TestSavePlanet_PutsTheArticleInTheReadingRoom(t *testing.T) {
	// 🚨 2026-09-07 改：收一颗星球**不再往树上种词**，它在阅读室里建一篇。
	//
	// 产品负责人：她在地图上看到的只有一个标题和两句摘要，还没读过任何东西，
	// 树上就多了一个词 —— 这一步来得太早。词该长在她真的读完之后（读完的报告
	// 上会提出候选词让她自己认，见 interest_proposal）。
	h, cookie, _, pool := liteHandler(t)
	id := seedPlanet(t, pool, 1, "climate", "climate-ocean")

	rec := savePlanetHTTP(h, cookie, id)
	if rec.Code != http.StatusOK {
		t.Fatalf("save = %d; body=%s", rec.Code, rec.Body)
	}
	var saved struct {
		Saved     bool   `json:"saved"`
		ReadingID string `json:"readingId"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &saved); err != nil {
		t.Fatalf("save 回包读不出来：%v", err)
	}
	if !saved.Saved || saved.ReadingID == "" {
		t.Fatalf("没有交回那一篇：%+v", saved)
	}

	// 树上不该因此多出任何词。
	tree := getTree(t, h, cookie)
	for _, k := range tree.Keywords {
		if k.TextZh == "气候" {
			t.Fatalf("收一颗星球把词种到树上去了：%+v", k)
		}
	}

	// 那一篇真的在阅读室里，标题就是这颗星的标题。
	list := httptest.NewRecorder()
	h.ServeHTTP(list, withCookie(httptest.NewRequest("GET", "/api/v1/readings", nil), cookie))
	if list.Code != http.StatusOK {
		t.Fatalf("readings = %d; body=%s", list.Code, list.Body)
	}
	if !strings.Contains(list.Body.String(), saved.ReadingID) {
		t.Errorf("阅读室里没有这一篇：%s", list.Body)
	}

	// 收下之后再读星图，那颗星是已收下，并且带着同一篇。
	got := getExplore(t, h, cookie)
	if !got.Planets[0].Saved {
		t.Error("收下后星图上没有显示已收下")
	}
	if got.Planets[0].ReadingID != saved.ReadingID {
		t.Errorf("星图上的那一篇和收下时的不是同一篇：%q vs %q",
			got.Planets[0].ReadingID, saved.ReadingID)
	}
}

func TestSavePlanet_IsIdempotent(t *testing.T) {
	// 连点两下不该在阅读室里攒出两篇同名的 —— 其中一篇她再也找不到。
	// 「稍后读」之后再「现在读」走的是同一个端点，所以这条不是洁癖。
	h, cookie, _, pool := liteHandler(t)
	id := seedPlanet(t, pool, 1, "climate", "climate-ocean")

	first := ""
	for i := 0; i < 3; i++ {
		rec := savePlanetHTTP(h, cookie, id)
		if rec.Code != http.StatusOK {
			t.Fatalf("save #%d = %d", i, rec.Code)
		}
		var saved struct {
			ReadingID string `json:"readingId"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &saved); err != nil {
			t.Fatalf("save #%d 回包读不出来：%v", i, err)
		}
		if i == 0 {
			first = saved.ReadingID
			continue
		}
		if saved.ReadingID != first {
			t.Fatalf("第 %d 次收下换了一篇：%q vs %q", i, saved.ReadingID, first)
		}
	}
}

func TestSavePlanet_UnknownPlanetIs404(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	rec := savePlanetHTTP(h, cookie, "00000000-0000-0000-0000-000000000abc")
	if rec.Code != http.StatusNotFound {
		t.Errorf("不存在的星球得到 %d，want 404", rec.Code)
	}
}

func TestSavePlanet_MalformedIDIs400(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	if rec := savePlanetHTTP(h, cookie, "not-a-uuid"); rec.Code != http.StatusBadRequest {
		t.Errorf("坏 id 得到 %d，want 400", rec.Code)
	}
}

func TestExploreToday_IsScopedToTheSignedInStudent(t *testing.T) {
	// 星图本身是全局的（今天的新闻对每个人一样），但**收藏是她自己的**。
	h, cookie, _, pool := liteHandler(t)
	id := seedPlanet(t, pool, 1, "climate", "climate-ocean")
	if rec := savePlanetHTTP(h, cookie, id); rec.Code != http.StatusOK {
		t.Fatalf("save = %d", rec.Code)
	}
	// 把这条收藏改挂到别人名下，她这边就该显示成未收藏。
	if _, err := pool.Exec(t.Context(), `
		UPDATE news_saved SET user_id = (
			SELECT id FROM users WHERE id <> news_saved.user_id LIMIT 1
		)`); err != nil {
		t.Fatalf("reassign: %v", err)
	}
	if got := getExplore(t, h, cookie); got.Planets[0].Saved {
		t.Error("看到了别人的收藏")
	}
}
