package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// writing_outline_test.go — Task 5: 大纲 (outline). Reuses writing_turn_test.go's
// helpers (createWritingAtomHTTP, writingTextStubProvider,
// writingStreamErrorProvider, liteHandlerWithProvider) — same package
// (api_test), same lite-writing test harness.

type writingOutlineItem struct {
	ID       string `json:"id"`
	Text     string `json:"text"`
	Role     string `json:"role"`
	Depth    int32  `json:"depth"`
	Position int32  `json:"position"`
	Source   string `json:"source"`
}

func getWritingOutlineHTTP(t *testing.T, h http.Handler, cookie *http.Cookie, id string) []writingOutlineItem {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/writings/"+id+"/outline", nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET outline = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var out struct {
		Outline []writingOutlineItem `json:"outline"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode outline: %v — body=%s", err, rec.Body)
	}
	return out.Outline
}

func putWritingOutlineHTTP(t *testing.T, h http.Handler, cookie *http.Cookie, id, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("PUT", "/api/v1/writings/"+id+"/outline", strings.NewReader(body)), cookie))
	return rec
}

// postWritingPlanTurn drives the planning conversation — the endpoint that
// replaced BOTH the retired POST /outline/generate and the short-lived
// structure picker that briefly stood in for it. The ownership test below
// guards this one, so that assertion keeps testing ownership instead of
// quietly passing because the route it named no longer exists at all.
func postWritingPlanTurn(t *testing.T, h http.Handler, cookie *http.Cookie, id, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/writings/"+id+"/plan/turn", strings.NewReader(body)), cookie))
	return rec
}

// TestWritingOutline_PutIsFullReplaceNotMerge — the task's central assertion:
// PUT is 全量替换 ("full replace"), never 合并 ("merge"). Three items, then
// two — only the second PUT's two items survive, position = array index.
func TestWritingOutline_PutIsFullReplaceNotMerge(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, writingTextStubProvider("x"))
	id := createWritingAtomHTTP(t, h, cookie, "写一篇关于气候变化的议论文")

	first := `{"outline":[{"text":"引言","depth":0},{"text":"论点一：碳排放","depth":1},{"text":"结论","depth":0}]}`
	if rec := putWritingOutlineHTTP(t, h, cookie, id, first); rec.Code != http.StatusOK {
		t.Fatalf("first PUT = %d; body=%s", rec.Code, rec.Body)
	}
	after1 := getWritingOutlineHTTP(t, h, cookie, id)
	if len(after1) != 3 {
		t.Fatalf("after first PUT = %d items, want 3: %+v", len(after1), after1)
	}

	second := `{"outline":[{"text":"新引言","depth":0},{"text":"新结论","depth":0}]}`
	if rec := putWritingOutlineHTTP(t, h, cookie, id, second); rec.Code != http.StatusOK {
		t.Fatalf("second PUT = %d; body=%s", rec.Code, rec.Body)
	}
	after2 := getWritingOutlineHTTP(t, h, cookie, id)
	if len(after2) != 2 {
		t.Fatalf("after second PUT = %d items, want 2 (full replace, not merge): %+v", len(after2), after2)
	}
	if after2[0].Text != "新引言" || after2[0].Position != 0 {
		t.Fatalf("item 0 = %+v, want 新引言 at position 0", after2[0])
	}
	if after2[1].Text != "新结论" || after2[1].Position != 1 {
		t.Fatalf("item 1 = %+v, want 新结论 at position 1", after2[1])
	}
	// Neither surviving row is one of the first PUT's ids — a real replace
	// mints fresh rows, it does not edit the old ones in place.
	for _, it := range after2 {
		for _, old := range after1 {
			if it.ID == old.ID {
				t.Fatalf("row %+v reused an id from the replaced outline %+v — not a real full replace", it, old)
			}
		}
	}
}

// TestWritingOutline_DepthClampedTo0To2 — depth is clamped, never rejected.
func TestWritingOutline_DepthClampedTo0To2(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, writingTextStubProvider("x"))
	id := createWritingAtomHTTP(t, h, cookie, "写一篇关于气候变化的议论文")

	body := `{"outline":[{"text":"太深","depth":9},{"text":"太浅","depth":-3}]}`
	if rec := putWritingOutlineHTTP(t, h, cookie, id, body); rec.Code != http.StatusOK {
		t.Fatalf("PUT = %d; body=%s", rec.Code, rec.Body)
	}
	items := getWritingOutlineHTTP(t, h, cookie, id)
	if len(items) != 2 || items[0].Depth != 2 || items[1].Depth != 0 {
		t.Fatalf("depths = %+v, want [2, 0] (clamped)", items)
	}
}

// TestWritingOutline_PutEmptyArrayClearsOutline — an empty PUT is a valid
// full replace too: it clears the outline entirely.
func TestWritingOutline_PutEmptyArrayClearsOutline(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, writingTextStubProvider("x"))
	id := createWritingAtomHTTP(t, h, cookie, "写一篇关于气候变化的议论文")

	if rec := putWritingOutlineHTTP(t, h, cookie, id, `{"outline":[{"text":"一","depth":0}]}`); rec.Code != http.StatusOK {
		t.Fatalf("first PUT = %d; body=%s", rec.Code, rec.Body)
	}
	if rec := putWritingOutlineHTTP(t, h, cookie, id, `{"outline":[]}`); rec.Code != http.StatusOK {
		t.Fatalf("empty PUT = %d; body=%s", rec.Code, rec.Body)
	}
	items := getWritingOutlineHTTP(t, h, cookie, id)
	if len(items) != 0 {
		t.Fatalf("outline after empty PUT = %+v, want none", items)
	}
}

// TestWritingOutline_RequiresOwnWriting — cross-kind isolation, mirrors
// TestWritingTurn_RequiresOwnWriting: a nonexistent/foreign id is a flat 404.
func TestWritingOutline_RequiresOwnWriting(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, writingTextStubProvider("x"))
	bogus := "00000000-0000-0000-0000-000000000000"
	if rec := putWritingOutlineHTTP(t, h, cookie, bogus, `{"outline":[]}`); rec.Code != http.StatusNotFound {
		t.Fatalf("PUT on nonexistent writing = %d, want 404; body=%s", rec.Code, rec.Body)
	}
	if rec := postWritingPlanTurn(t, h, cookie, bogus, `{"text":"在吗"}`); rec.Code != http.StatusNotFound {
		t.Fatalf("plan turn on nonexistent writing = %d, want 404; body=%s", rec.Code, rec.Body)
	}
}

/* ── 材料的出处（0158，2026-09-16） ────────────────────────────────────────
 *
 * 规划这一步原来只问「你自己有没有经历过」。产品负责人的原话：
 *
 *	> in writing, currently we focus too much on personal experience. but we
 *	> can also let students to search for other materials, give back the
 *	> supporting materials and ai give feedbacks.
 *
 * 她找回来的材料要能带着出处进图 —— 因为印记 查它靠的就是出处（是谁说的、
 * 哪一年、多大样本）。这几条守的是那一列**活得下来**。
 * ------------------------------------------------------------------------ */

func TestWritingOutline_KeepsWhereAMaterialCameFrom(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, writingTextStubProvider("x"))
	id := createWritingAtomHTTP(t, h, cookie, "上学时间该不该推迟一小时")

	body := `{"outline":[
	  {"text":"上学时间该往后推一小时","role":"中心论点","depth":0},
	  {"text":"睡眠不足影响上课","role":"一条理由","depth":1},
	  {"text":"青少年平均每晚少睡 1.5 小时","role":"一组数据","depth":2,
	   "source":"中国睡眠研究会 2023 年报告"}
	]}`
	if rec := putWritingOutlineHTTP(t, h, cookie, id, body); rec.Code != http.StatusOK {
		t.Fatalf("PUT = %d; body=%s", rec.Code, rec.Body)
	}
	got := getWritingOutlineHTTP(t, h, cookie, id)
	if len(got) != 3 {
		t.Fatalf("want 3 items, got %d: %+v", len(got), got)
	}
	if got[2].Source != "中国睡眠研究会 2023 年报告" {
		t.Errorf("出处没存下来：%+v", got[2])
	}
	// 她自己的经历留空 —— 不编一个「本人」出来。
	if got[1].Source != "" {
		t.Errorf("没有出处的那一条被填了东西：%q", got[1].Source)
	}
}

// 🚨 她改一个字，出处不能跟着没。
//
// PUT 是全量替换，客户端必须把服务端发给它的 source 原样回传 —— 和 role 同一个
// 坑（0100 加 role 那次就是这么发现的）。少回传一个字段，症状是**她编辑任何一
// 块之后，整张图上所有的出处一起消失**，而她不会知道发生过什么。
func TestWritingOutline_EditingOneBlockKeepsEveryOtherSource(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, writingTextStubProvider("x"))
	id := createWritingAtomHTTP(t, h, cookie, "上学时间该不该推迟一小时")

	first := `{"outline":[
	  {"text":"上学时间该往后推一小时","role":"中心论点","depth":0},
	  {"text":"青少年平均每晚少睡 1.5 小时","role":"一组数据","depth":1,
	   "source":"中国睡眠研究会 2023 年报告"}
	]}`
	if rec := putWritingOutlineHTTP(t, h, cookie, id, first); rec.Code != http.StatusOK {
		t.Fatalf("first PUT = %d; body=%s", rec.Code, rec.Body)
	}
	before := getWritingOutlineHTTP(t, h, cookie, id)

	// 她把第一块改了一个字，客户端照着服务端发的那一份回传整张图。
	edited := `{"outline":[
	  {"text":"上学时间该往后推两小时","role":"中心论点","depth":0},
	  {"text":"` + before[1].Text + `","role":"` + before[1].Role + `","depth":1,
	   "source":"` + before[1].Source + `"}
	]}`
	if rec := putWritingOutlineHTTP(t, h, cookie, id, edited); rec.Code != http.StatusOK {
		t.Fatalf("second PUT = %d; body=%s", rec.Code, rec.Body)
	}
	after := getWritingOutlineHTTP(t, h, cookie, id)
	if len(after) != 2 || after[1].Source != "中国睡眠研究会 2023 年报告" {
		t.Errorf("改一块之后出处没了：%+v", after)
	}
}

// 出处超长就截断，不把整次保存退回去。她已经打完了，为一个格式问题让她重来，
// 代价比截掉一截大。
func TestWritingOutline_TruncatesAnOverlongSource(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, writingTextStubProvider("x"))
	id := createWritingAtomHTTP(t, h, cookie, "上学时间该不该推迟一小时")

	long := strings.Repeat("长", 500)
	body := `{"outline":[{"text":"一组数据","role":"一组数据","depth":0,"source":"` + long + `"}]}`
	if rec := putWritingOutlineHTTP(t, h, cookie, id, body); rec.Code != http.StatusOK {
		t.Fatalf("PUT = %d; body=%s", rec.Code, rec.Body)
	}
	got := getWritingOutlineHTTP(t, h, cookie, id)
	if len(got) != 1 {
		t.Fatalf("want 1 item: %+v", got)
	}
	if n := len([]rune(got[0].Source)); n != 300 {
		t.Errorf("出处 %d 字，应该截到 300", n)
	}
}
