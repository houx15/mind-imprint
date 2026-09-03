package api_test

// explore_test.go — 今日新闻星图的库级测试。
//
// 测的是这一屏最容易静默出错的四件事：
//
//   - 生成失败时**这一屏是空的，并且说得出为什么** —— 绝不拿昨天的冒充今天的。
//   - 「今天试过一次」的章**盖在生成之前**。按产出判断的话，全源挂掉的那天，
//     每个打开星图的学生都会再触发一次全量抓取 + 一次旗舰调用。这条教训第三次
//     出现了（0104 / 0116 / 0118）。
//   - 收藏一颗星真的把词种进了**同一棵树**，evidence 是那颗星的钩子。
//   - 收藏是幂等的：连点两下不会把强度刷上去。

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

type exploreResp struct {
	Day     string `json:"day"`
	Note    string `json:"note"`
	Planets []struct {
		ID         string `json:"id"`
		Rank       int    `json:"rank"`
		TitleZh    string `json:"titleZh"`
		Hook       string `json:"hook"`
		URL        string `json:"url"`
		Source     string `json:"source"`
		Field      string `json:"field"`
		Keyword    string `json:"keyword"`
		Saved      bool   `json:"saved"`
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

// seedPlanet 直接写库，绕开抓取与模型 —— 这些测试要验的是读与收藏那一半。
func seedPlanet(t *testing.T, pool *pgxpool.Pool, rank int, keyword, disciplineID string) string {
	t.Helper()
	var id string
	err := pool.QueryRow(t.Context(), `
		INSERT INTO news_planet (day, rank, title_zh, title_en, summary, hook, url,
		                         source_name, field, discipline_id, keyword, published_at)
		VALUES (CURRENT_DATE, $1, '深海珊瑚在 30 度水里活下来了', 'Corals survive 30C',
		        '红海北端一片珊瑚在超过白化阈值的水温里没有白化。',
		        '四平方公里的珊瑚，能代表一整片海吗？',
		        'https://example.org/coral', 'Nature', 'science', $2, $3, now())
		RETURNING id::text`, rank, disciplineID, keyword).Scan(&id)
	if err != nil {
		t.Fatalf("seed planet: %v", err)
	}
	// 同时盖上「今天试过了」的章，否则读取时会去触发一次真的生成（会打网络）。
	if _, err := pool.Exec(t.Context(),
		`INSERT INTO news_day (day, planet_count) VALUES (CURRENT_DATE, 1)
		 ON CONFLICT (day) DO NOTHING`); err != nil {
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
		`SELECT count(*) FROM news_day WHERE day = CURRENT_DATE`).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Fatalf("news_day 里有 %d 行，want 1 —— 零产出的一天必须也盖章", n)
	}

	// 再打开一次，仍然只有一行（没有第二次尝试）。
	getExplore(t, h, cookie)
	if err := pool.QueryRow(t.Context(),
		`SELECT count(*) FROM news_day WHERE day = CURRENT_DATE`).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Errorf("第二次打开又试了一次生成：news_day 有 %d 行", n)
	}
}

/* ── 读 ─────────────────────────────────────────────────────────────────── */

func TestExploreToday_ReturnsSeededPlanetsWithTheDisciplineHydrated(t *testing.T) {
	h, cookie, _, pool := liteHandler(t)
	seedPlanet(t, pool, 1, "样本代表性", "climate-ocean")

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
	seedPlanet(t, pool, 1, "样本代表性", "astrology-of-the-ancients")

	got := getExplore(t, h, cookie)
	if got.Planets[0].Discipline != nil {
		t.Errorf("不存在的学科被发出去了：%+v", got.Planets[0].Discipline)
	}
	if got.Planets[0].TitleZh == "" {
		t.Error("为了一条坏边把整颗星丢了")
	}
}

/* ── 收藏 ───────────────────────────────────────────────────────────────── */

func TestSavePlanet_PlantsTheKeywordIntoTheSameTree(t *testing.T) {
	h, cookie, _, pool := liteHandler(t)
	id := seedPlanet(t, pool, 1, "样本代表性", "climate-ocean")

	if rec := savePlanetHTTP(h, cookie, id); rec.Code != http.StatusOK {
		t.Fatalf("save = %d; body=%s", rec.Code, rec.Body)
	}

	tree := getTree(t, h, cookie)
	var found bool
	for _, k := range tree.Keywords {
		if k.TextZh != "样本代表性" {
			continue
		}
		found = true
		if len(k.Sources) != 1 || k.Sources[0].Kind != "news" {
			t.Errorf("来源不对：%+v", k.Sources)
			continue
		}
		// evidence 是那颗星的钩子 —— 她按下收藏时看着的就是那个问题，所以它
		// 有资格作为「这个词为什么在你树上」的答案。
		if k.Sources[0].Evidence != "四平方公里的珊瑚，能代表一整片海吗？" {
			t.Errorf("evidence 不是那颗星的钩子：%q", k.Sources[0].Evidence)
		}
	}
	if !found {
		t.Fatalf("收藏的词没有出现在树上：%+v", tree.Keywords)
	}

	// 收藏之后再读星图，那颗星是已收藏。
	if got := getExplore(t, h, cookie); !got.Planets[0].Saved {
		t.Error("收藏后星图上没有显示已收藏")
	}
}

func TestSavePlanet_IsIdempotent(t *testing.T) {
	// 连点两下不该把强度刷上去 —— 强度是「几件不同的事」，不是「点了几次」。
	h, cookie, _, pool := liteHandler(t)
	id := seedPlanet(t, pool, 1, "样本代表性", "climate-ocean")

	for i := 0; i < 3; i++ {
		if rec := savePlanetHTTP(h, cookie, id); rec.Code != http.StatusOK {
			t.Fatalf("save #%d = %d", i, rec.Code)
		}
	}
	tree := getTree(t, h, cookie)
	for _, k := range tree.Keywords {
		if k.TextZh == "样本代表性" {
			if len(k.Sources) != 1 {
				t.Errorf("收藏三次长出了 %d 条来源", len(k.Sources))
			}
			if k.Strength != 1 {
				t.Errorf("强度被刷到了 %d", k.Strength)
			}
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
	id := seedPlanet(t, pool, 1, "样本代表性", "climate-ocean")
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
