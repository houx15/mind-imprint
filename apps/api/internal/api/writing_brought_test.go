package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// 带一篇写完的进来。
//
// 产品负责人 2026-09-11：
//
//	> we also make students available to upload a written one to seek for advice
//
// 这里守的是四件事，其中两件是**诚实**而不是功能：
//
//  1. 正文落进 writing_draft，而且这一篇直接在 draft 那一步——她要的是意见，
//     不是从零开始，让她先走一遍结构和段落是在浪费她的时间。
//  2. origin 记成 brought，界面和报告据此说明那两步**没有发生过**。
//  3. 🚨 正文**不进转录**。转录里的 student 行是「她在这个房间里说过的话」，
//     混进一篇她在别处写完的作文，过程评估就再也分不清哪些是这里发生的；
//     而且转录是每一条下游提示词的共同材料，一整篇作文会把它们全顶爆。
//  4. 不给正文就是老路：还在 outline，origin 还是 here。

type broughtMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func writingMessages(t *testing.T, h http.Handler, cookie *http.Cookie, id string) []broughtMsg {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/writings/"+id+"/messages", nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET messages = %d; body=%s", rec.Code, rec.Body)
	}
	var out struct {
		Messages []broughtMsg `json:"messages"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode messages: %v — body=%s", err, rec.Body)
	}
	return out.Messages
}

func createBroughtWritingHTTP(t *testing.T, h http.Handler, cookie *http.Cookie, idea, body string) string {
	t.Helper()
	payload, err := json.Marshal(map[string]string{"idea": idea, "lang": "zh", "body": body})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(
		httptest.NewRequest("POST", "/api/v1/writings", strings.NewReader(string(payload))), cookie))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create brought writing = %d; body=%s", rec.Code, rec.Body)
	}
	var out struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || out.ID == "" {
		t.Fatalf("decode id: %v — body=%s", err, rec.Body)
	}
	return out.ID
}

func TestBroughtWriting_LandsInDraftWithHerText(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, nil)
	essay := "学校食堂每天倒掉的饭特别多。上周五我数了一下，回收桶里有六个桶是满的。"
	id := createBroughtWritingHTTP(t, h, cookie, "我写完了一篇关于食堂浪费的作文，想要点意见", essay)

	wr := getWritingHTTP(t, h, cookie, id)
	if wr.Stage != "draft" {
		t.Fatalf("stage = %q, want draft — 带进来的一篇不该再走一遍结构和段落", wr.Stage)
	}
	if wr.Origin != "brought" {
		t.Fatalf("origin = %q, want brought — 报告要靠这一列说清那两步没有发生过", wr.Origin)
	}

	draft, _ := getWritingDraftHTTP(t, h, cookie, id)
	if draft.Body != essay {
		t.Fatalf("draft body = %q, want her essay verbatim %q", draft.Body, essay)
	}
}

// 🚨 正文绝不能以「她说的话」的身份进转录。
func TestBroughtWriting_EssayStaysOutOfTheTranscript(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, nil)
	essay := "这是我带进来的那篇作文的正文，它不该出现在对话记录里。"
	id := createBroughtWritingHTTP(t, h, cookie, "带一篇进来", essay)

	msgs := writingMessages(t, h, cookie, id)
	for _, m := range msgs {
		if m.Role == "student" && strings.Contains(m.Content, "它不该出现在对话记录里") {
			t.Fatalf("她带进来的正文以 student 的身份进了转录：%+v", m)
		}
	}
	// 但这件事本身要留下记录——和 stage 变更同一种记法。
	var sawOrigin bool
	for _, m := range msgs {
		if m.Role == "system" && strings.Contains(m.Content, "origin: brought") {
			sawOrigin = true
		}
	}
	if !sawOrigin {
		t.Fatal("带进来这件事必须留下一条 system 记录，否则过程树上看不出它发生过")
	}
}

func TestBroughtWriting_NoBodyIsTheOldPath(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, nil)
	id := createWritingAtomHTTP(t, h, cookie, "我想写食堂浪费这件事")

	wr := getWritingHTTP(t, h, cookie, id)
	if wr.Stage != "outline" {
		t.Fatalf("stage = %q, want outline — 不给正文就还是原来那条路", wr.Stage)
	}
	if wr.Origin != "here" {
		t.Fatalf("origin = %q, want here", wr.Origin)
	}
}

// 太长的不静默截断，明确报错——截掉一半再去评，评的是另一篇文章。
func TestBroughtWriting_TooLongIsRefusedNotTruncated(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, nil)
	huge := strings.Repeat("字", 20001)
	payload, _ := json.Marshal(map[string]string{"idea": "太长的一篇", "lang": "zh", "body": huge})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(
		httptest.NewRequest("POST", "/api/v1/writings", strings.NewReader(string(payload))), cookie))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("create = %d, want 400 body_too_long; body=%s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), "body_too_long") {
		t.Fatalf("want the error CODE in the body so the client can tell them apart: %s", rec.Body)
	}
}
