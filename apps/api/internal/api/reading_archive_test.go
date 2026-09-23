package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// 产品负责人 2026-09-23 第 3 条：
//
//	「自己粘贴文本后，系统会自动分段，如果学生发现分段分错了，无法重新编辑，
//	  只能再开一个新的，阅读列表里旧的也没办法删除。」
//
// 两件事，两组判据：
//   - 收起来（这个文件）：列表里去得掉，而且**不毁过程数据**。
//   - 重新编辑（TestSourceStaysEditableUntilSomethingAnchorsToIt）：
//     服务端一直允许，现在把这件事告诉前端。

func listReadingIDs(t *testing.T, h http.Handler, cookie *http.Cookie) []string {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/readings", nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("list readings = %d; body=%s", rec.Code, rec.Body)
	}
	var env struct {
		Readings []struct {
			ID string `json:"id"`
		} `json:"readings"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode list: %v (body=%s)", err, rec.Body)
	}
	out := make([]string, 0, len(env.Readings))
	for _, r := range env.Readings {
		out = append(out, r.ID)
	}
	return out
}

func listHasReading(ids []string, id string) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}

func TestArchiveReadingTakesItOutOfHerList(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	keep := createReadingAtomHTTP(t, h, cookie, "留着这一篇")
	drop := createReadingAtomHTTP(t, h, cookie, "粘错了的那一篇")

	if ids := listReadingIDs(t, h, cookie); !listHasReading(ids, drop) {
		t.Fatal("刚建出来的那一篇不在列表里")
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("DELETE", "/api/v1/readings/"+drop, nil), cookie))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("DELETE = %d, want 204; body=%s", rec.Code, rec.Body)
	}

	ids := listReadingIDs(t, h, cookie)
	if listHasReading(ids, drop) {
		t.Error("收起来之后它还在列表里")
	}
	if !listHasReading(ids, keep) {
		t.Error("🚨 把别的那一篇也收掉了")
	}

	// 🚨 收起来**不毁数据**：那一篇按 id 还打得开（铁律④「过程即数据」，
	// 而且老师那一侧照旧看得见）。真 DELETE 会让这一条变成 404。
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/readings/"+drop, nil), cookie))
	if rec.Code != http.StatusOK {
		t.Errorf("收起来之后按 id 打不开了（%d）—— 那就不是收起来，是删掉了", rec.Code)
	}
}

// 收起来是可逆的。一条单向的门迟早要靠「再开一个新的」绕过去，
// 而那正是这条 bug 的由来。
func TestUnarchivePutsItBack(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	id := createReadingAtomHTTP(t, h, cookie, "收错了")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("DELETE", "/api/v1/readings/"+id, nil), cookie))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("DELETE = %d; body=%s", rec.Code, rec.Body)
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/readings/"+id+"/unarchive", nil), cookie))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("unarchive = %d; body=%s", rec.Code, rec.Body)
	}
	if ids := listReadingIDs(t, h, cookie); !listHasReading(ids, id) {
		t.Error("放回去之后它没回到列表里")
	}
}

// 幂等：她连点两下不该报错。
func TestArchiveTwiceIsFine(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	id := createReadingAtomHTTP(t, h, cookie, "连点两下")
	for i := 0; i < 2; i++ {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, withCookie(httptest.NewRequest("DELETE", "/api/v1/readings/"+id, nil), cookie))
		if rec.Code != http.StatusNoContent {
			t.Fatalf("第 %d 次 DELETE = %d; body=%s", i+1, rec.Code, rec.Body)
		}
	}
}

// 别人的阅读收不掉。门是所有权，不是状态。
func TestArchiveRefusesSomeoneElsesReading(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	id := createReadingAtomHTTP(t, h, cookie, "我的")
	rec := httptest.NewRecorder()
	// 不带 cookie：没登录。
	h.ServeHTTP(rec, httptest.NewRequest("DELETE", "/api/v1/readings/"+id, nil))
	if rec.Code == http.StatusNoContent {
		t.Fatal("没登录也能收起别人的阅读")
	}
}

// 重新编辑那一半：服务端**一直允许**，只是没告诉过前端。
// 这一条钉的是那个信号（sourceDTO.editable），因为界面据它决定摆不摆
// 「重新编辑原文」。客户端不自己猜 —— 猜错的那一侧是她按下去拿到 409。
func TestSourceSaysWhetherItIsStillEditable(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	id := createReadingAtomHTTP(t, h, cookie, "她自己粘的一篇")

	body := strings.NewReader(`{"title":"她自己粘的一篇","text":"第一段。\n\n第二段。\n\n第三段。"}`)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("PUT", "/api/v1/readings/"+id+"/source", body), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT source = %d; body=%s", rec.Code, rec.Body)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/readings/"+id+"/source", nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET source = %d; body=%s", rec.Code, rec.Body)
	}
	var got struct {
		Editable bool `json:"editable"`
		Blocks   []struct {
			Text string `json:"text"`
		} `json:"blocks"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v (body=%s)", err, rec.Body)
	}
	if len(got.Blocks) != 3 {
		t.Fatalf("按空行分成了 %d 段，想要 3 段", len(got.Blocks))
	}
	// 刚粘完，什么都还没锚上去 —— 她该能改。
	if !got.Editable {
		t.Error("🚨 刚粘完就说不能改了 —— 那她只能「再开一个新的」，正是这条 bug")
	}

	// 而且真的改得动：重新粘一次，段落跟着变。
	again := strings.NewReader(`{"title":"她自己粘的一篇","text":"第一段。第二段。\n\n第三段。"}`)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("PUT", "/api/v1/readings/"+id+"/source", again), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("重新粘 = %d; body=%s", rec.Code, rec.Body)
	}
	var re struct {
		Blocks []struct {
			Text string `json:"text"`
		} `json:"blocks"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &re); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(re.Blocks) != 2 {
		t.Errorf("重新分段之后是 %d 段，想要 2 段", len(re.Blocks))
	}
}
