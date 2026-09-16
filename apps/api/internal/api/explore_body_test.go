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

// readingSourceDetail 和 readingSourceHTTP 同一个请求，多读一位 excerptOnly。
func readingSourceDetail(t *testing.T, h http.Handler, cookie *http.Cookie, readingID string) (int, []string, bool) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(
		httptest.NewRequest("GET", "/api/v1/readings/"+readingID+"/source", nil), cookie))
	if rec.Code != http.StatusOK {
		return rec.Code, nil, false
	}
	var got struct {
		Blocks []struct {
			Text string `json:"text"`
		} `json:"blocks"`
		ExcerptOnly bool `json:"excerptOnly"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode source: %v (%s)", err, rec.Body)
	}
	out := make([]string, 0, len(got.Blocks))
	for _, b := range got.Blocks {
		out = append(out, b.Text)
	}
	return rec.Code, out, got.ExcerptOnly
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

// feed 没带正文、原页面也抓不到的时候（实测半数源如此），她拿到的是**那段
// 摘要**，并且这一篇被标成 excerptOnly —— 阅读室据此摆出「我们无法直接获取
// 正文，如果想要阅读全文，请跳转原网站」加一颗跳转按钮。
//
// 🚨 这一条 2026-09-16 改过方向。原来断言的是「没有正文 → GET /source 404」，
// 也就是她落在一个粘贴框上，而前端会**另开一页原文**让她自己复制。产品负责人
// 把那个自动跳转否掉了：
//
//	> we jump to reading page. if we extracted the texts successfully, then
//	> begin reading directly. or if we only have abstract, we go to reading
//	> with abstract, with below a button …
//
// 于是「只有摘要」第一次成了一个要**说出来**的状态，而不是一个空屏。
func TestSavePlanet_NoFeedBodyFallsBackToTheAbstract(t *testing.T) {
	h, cookie, _, pool := liteHandler(t)
	id := seedPlanetWithBody(t, pool, "")

	readingID := savedReadingID(t, savePlanetHTTP(h, cookie, id))

	code, blocks, excerptOnly := readingSourceDetail(t, h, cookie, readingID)
	if code != http.StatusOK {
		t.Fatalf("抓不到正文时她该拿到摘要，GET /source = %d", code)
	}
	if len(blocks) == 0 || !strings.Contains(blocks[0], "一句摘要") {
		t.Fatalf("摘要没有落进阅读室：%q", blocks)
	}
	// 🚨 这一位是整条链子的意义所在。不说出来，两句话的摘要和一篇很短的报道
	// 在屏幕上长得一模一样，她读完两句就以为读完了。
	if !excerptOnly {
		t.Error("只有摘要却没有标成 excerptOnly —— 她不会知道这不是全文")
	}
}

// feed 带了正文的那些，**不**标 excerptOnly：在一篇完整的文章下面摆一条
// 「我们无法直接获取正文」，比不摆更糟。
func TestSavePlanet_AFeedBodyIsNotAnExcerpt(t *testing.T) {
	h, cookie, _, pool := liteHandler(t)
	id := seedPlanetWithBody(t, pool, feedBody)
	readingID := savedReadingID(t, savePlanetHTTP(h, cookie, id))

	_, _, excerptOnly := readingSourceDetail(t, h, cookie, readingID)
	if excerptOnly {
		t.Error("feed 给的正文被当成了摘要")
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
