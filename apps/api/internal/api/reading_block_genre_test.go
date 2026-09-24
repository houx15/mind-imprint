package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// 工具单那个端点要把**体裁**一起发出去。
//
// 产品负责人 2026-09-24：「for short poems, maybe we can have a better
// display. it is always short and it is somekind of old poems need some
// ancient vibe, maybe we can use a card-like format to show them.」
//
// 正文那一栏要按体裁换一种摆法，就得知道这一篇是什么。前端从工具单里反推是
// 做得到的（单子里有 poem_shape 就是诗），但那条路把「这一篇是什么」和
// 「这一篇有哪几件工具」绑死了：日后诗词少一件工具，正文的样子会跟着塌。
// 判定只有一个点 —— 服务端算好了发出来。
func TestBlockToolsCarryTheGenre(t *testing.T) {
	const plan = `{"genre":"poem","routineKey":"zh-poem","focusBlocks":["b1"],
	  "steps":[{"kind":"read","detail":"先把这首诗读一遍。"}],
	  "oneLine":"一个人在雪天独自钓鱼","gist":"四句写尽一个空无一人的雪天。",
	  "shape":"由远到近","load":{"b1":"core"},"parts":[]}`
	h, cookie, _, _ := liteHandlerWithProvider(t, writingTextStubProvider(plan))
	id := createReadingAtom(t, h, cookie)
	// 🚨 一句一行、中间没有空行 —— SplitBlocks 只在空行处切，所以这**是一段**，
	// 段里带着换行。正文那一栏原来把这几个换行折成空格，于是一首五绝在屏幕上
	// 是连成一行的二十个字。
	putReadingSourceHTTP(t, h, cookie, id, "江雪",
		"千山鸟飞绝，\n万径人踪灭。\n孤舟蓑笠翁，\n独钓寒江雪。")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/readings/"+id+"/plan", nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("排读法失败：%d %s", rec.Code, rec.Body)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/readings/"+id+"/blocks/tools", nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET tools = %d; body=%s", rec.Code, rec.Body)
	}
	var out struct {
		Lang  string `json:"lang"`
		Genre string `json:"genre"`
		Tools []struct {
			ID    string `json:"id"`
			Scope string `json:"scope"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode tools: %v — body=%s", err, rec.Body)
	}
	if out.Genre != "poem" {
		t.Errorf("工具单里的体裁 = %q，要的是 poem —— 正文那一栏据此决定按不按诗的样子摆", out.Genre)
	}
	if out.Lang != "zh" {
		t.Errorf("lang = %q", out.Lang)
	}
	// 顺带守住：诗那一件整篇工具确实在单子里，而它带着 scope。
	var found bool
	for _, tool := range out.Tools {
		if tool.ID == "poem_shape" {
			found = true
			if tool.Scope != "article" {
				t.Errorf("poem_shape 的 scope = %q", tool.Scope)
			}
		}
	}
	if !found {
		t.Errorf("诗的工具单里没有 poem_shape：%s", rec.Body)
	}
}

// 体裁认不出来时发的是空串，不是猜一个。前端据此走原来那条路。
func TestBlockToolsGenreIsEmptyWhenUnknown(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, writingTextStubProvider("x"))
	id := createReadingAtom(t, h, cookie)
	putReadingSourceHTTP(t, h, cookie, id, "城市为什么比郊区热？", zhArticle)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/readings/"+id+"/blocks/tools", nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET tools = %d", rec.Code)
	}
	var out struct {
		Genre string `json:"genre"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Genre != "" {
		t.Errorf("没排过读法就报出了体裁 %q", out.Genre)
	}
}
