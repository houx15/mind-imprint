package api_test

// explore_body_test.go —— feed 自己带的正文，是取正文的第三条路。
//
// 「现在读」建的那一篇原来**没有正文**：只有标题和链接进阅读室，然后再去抓一次
// 原页面 —— 抓得到就有，抓不到（403 / 反爬 / 付费墙）就摆粘贴框让她自己贴。
// 全链路走查里抓的那一次就是 400，她落在粘贴框上。
//
// 而很多源的正文本来就在 feed 里（content:encoded），生成星图那一刻已经在手上。
// 存下来，她点进去就能读。三条路产出完全一样，都落进 reading_source：
//
//	1. feed 自带（这个文件）
//	2. 抓原页面
//	3. 她自己粘
//
// 🚨 第二条不能覆盖第一条：前端在「现在读」之后照样会带着 url 调一次
// PUT /source，那一次去抓原页面，抓到会把好正文换成另一份，抓不到就把一篇本来
// 能读的文章变成一条红字。

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// 三段，段落之间一个空行 —— 和她粘进来的文章一模一样的形状。
const feedBody = "第一段：太阳能中午最多，用电高峰在傍晚，这中间是错位。\n\n" +
	"第二段：填这个错位要靠储能、需求响应或者跨区输电。\n\n" +
	"第三段：白天多出来的电卖不掉，甚至要被弃掉。"

func seedPlanetWithBody(t *testing.T, pool *pgxpool.Pool, body string) string {
	t.Helper()
	var id string
	err := pool.QueryRow(t.Context(), `
		INSERT INTO news_planet (day, rank, title_zh, title_en, summary, hook, url,
		                         source_name, field, discipline_id, interest_id,
		                         published_at, body)
		VALUES ($2::date, 1, '电网才是瓶颈', 'The grid is the bottleneck',
		        '一句摘要。', '装机再多，电存不下来还算数吗？',
		        'https://example.org/grid', 'Grist', 'science', '', '', now(), $1)
		RETURNING id::text`, body, appToday()).Scan(&id)
	if err != nil {
		t.Fatalf("seed planet: %v", err)
	}
	if _, err := pool.Exec(t.Context(),
		`INSERT INTO news_day (day, planet_count) VALUES ($1::date, 1)
		 ON CONFLICT (day) DO NOTHING`, appToday()); err != nil {
		t.Fatalf("seed day: %v", err)
	}
	return id
}

func readingSourceHTTP(t *testing.T, h http.Handler, cookie *http.Cookie, readingID string) (int, []string) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(
		httptest.NewRequest("GET", "/api/v1/readings/"+readingID+"/source", nil), cookie))
	if rec.Code != http.StatusOK {
		return rec.Code, nil
	}
	var got struct {
		Blocks []struct {
			Text string `json:"text"`
		} `json:"blocks"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode source: %v (%s)", err, rec.Body)
	}
	out := make([]string, 0, len(got.Blocks))
	for _, b := range got.Blocks {
		out = append(out, b.Text)
	}
	return rec.Code, out
}

func savedReadingID(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("save planet got %d: %s", rec.Code, rec.Body)
	}
	var p struct {
		ReadingID string `json:"readingId"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("decode planet: %v", err)
	}
	if p.ReadingID == "" {
		t.Fatal("收藏之后没有给出阅读 id")
	}
	return p.ReadingID
}

// feed 带了正文，她点进去就是一篇能读的文章 —— 没有粘贴框那一屏。
func TestSavePlanet_FillsTheArticleFromTheFeed(t *testing.T) {
	h, cookie, _, pool := liteHandler(t)
	id := seedPlanetWithBody(t, pool, feedBody)

	readingID := savedReadingID(t, savePlanetHTTP(h, cookie, id))

	code, blocks := readingSourceHTTP(t, h, cookie, readingID)
	if code != http.StatusOK {
		t.Fatalf("正文没有落进阅读室，GET /source 得到 %d —— 她会看到粘贴框", code)
	}
	// 🚨 分成三块，不是一整块。阅读室整间屋子都建在段落上：她点一段才有段落
	// 工具条，印记挂卡片、排精读段落靠的都是块 id。
	if len(blocks) != 3 {
		t.Fatalf("切出 %d 块，应该是 3 块 —— 段落没了，阅读室就是一堵墙：%q", len(blocks), blocks)
	}
	if !strings.Contains(blocks[0], "错位") || !strings.Contains(blocks[2], "弃掉") {
		t.Errorf("段落顺序或内容不对：%q", blocks)
	}
}

// feed 没带正文的源（实测半数如此）照旧走另外两条路：这里没有正文，
// 阅读室摆粘贴框 —— 这是**正常结果**，不是失败。
func TestSavePlanet_NoFeedBodyStillLeavesTheOtherRoutes(t *testing.T) {
	h, cookie, _, pool := liteHandler(t)
	id := seedPlanetWithBody(t, pool, "")

	readingID := savedReadingID(t, savePlanetHTTP(h, cookie, id))

	if code, _ := readingSourceHTTP(t, h, cookie, readingID); code != http.StatusNotFound {
		t.Errorf("feed 没带正文时不该凭空有正文，GET /source = %d", code)
	}
}

// 🚨 前端在「现在读」之后会带着 url 再调一次 PUT /source。那一次不能把已经放好
// 的正文换掉 —— 抓得到会换成另一份，抓不到就直接报错，把一篇能读的文章变成红字。
func TestPutReadingSource_DoesNotClobberWhatTheFeedAlreadyGave(t *testing.T) {
	h, cookie, _, pool := liteHandler(t)
	id := seedPlanetWithBody(t, pool, feedBody)
	readingID := savedReadingID(t, savePlanetHTTP(h, cookie, id))

	// 前端那一次调用：带 url，不带正文。
	rec := httptest.NewRecorder()
	body := strings.NewReader(`{"title":"电网才是瓶颈","url":"https://example.org/grid"}`)
	req := httptest.NewRequest("PUT", "/api/v1/readings/"+readingID+"/source", body)
	req.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(rec, withCookie(req, cookie))

	if rec.Code != http.StatusOK {
		t.Fatalf("这一次调用把她的文章弄成了 %d：%s", rec.Code, rec.Body)
	}
	code, blocks := readingSourceHTTP(t, h, cookie, readingID)
	if code != http.StatusOK || len(blocks) != 3 {
		t.Fatalf("正文被覆盖或弄丢了：code=%d blocks=%q", code, blocks)
	}
	if !strings.Contains(blocks[0], "错位") {
		t.Errorf("正文变了：%q", blocks)
	}
}

// 她**粘**一份新的进来照旧覆盖 —— 那是她明确要换掉这一篇，上面那条挡的不是这个。
func TestPutReadingSource_HerOwnPasteStillReplacesIt(t *testing.T) {
	h, cookie, _, pool := liteHandler(t)
	id := seedPlanetWithBody(t, pool, feedBody)
	readingID := savedReadingID(t, savePlanetHTTP(h, cookie, id))

	rec := httptest.NewRecorder()
	body := strings.NewReader(`{"title":"我自己找的一篇","text":"她粘的第一段。\n\n她粘的第二段。"}`)
	req := httptest.NewRequest("PUT", "/api/v1/readings/"+readingID+"/source", body)
	req.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(rec, withCookie(req, cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("粘贴被拒了：%d %s", rec.Code, rec.Body)
	}

	_, blocks := readingSourceHTTP(t, h, cookie, readingID)
	if len(blocks) != 2 || !strings.Contains(blocks[0], "她粘的第一段") {
		t.Errorf("她粘的那一份没有覆盖掉 feed 那一份：%q", blocks)
	}
}
