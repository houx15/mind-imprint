package api_test

import (
	"encoding/json"
	"net/http"
	"testing"
)

// 🚨 一条支线由印记先开口，而且它得知道这条支线是从哪句话上长出来的。
//
// 产品负责人 2026-09-02：「always the first sentence is seeded by AI. system
// should gives AI a context about this branch first so that we can guide
// student」。让她一进来面对一个空房间，等于把「深挖」变成了又一个输入框。
func TestPblSession_OpensWithYinjiSpeaking(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, pblCoachSaying(
		`{"reply":"你刚说食堂阿姨每天都这么讲。我们就单看这一句。","hook":"","hook_kind":""}`))
	pid := newProjectViaAPI(t, h, cookie)

	// 主线上先有一句她说的话，支线才有东西可接。
	if rec := pblPost(t, h, cookie, "/api/v1/pbl/projects/"+pid+"/turn",
		`{"text":"阿姨说每天都这样"}`); rec.Code != http.StatusOK {
		t.Fatalf("main turn = %d; body=%s", rec.Code, rec.Body)
	}

	rec := pblPost(t, h, cookie, "/api/v1/pbl/projects/"+pid+"/sessions",
		`{"kind":"free","question":"「每天都这样」是真的每天吗？","anchorKind":"hook","anchorRef":"2"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("open session = %d; body=%s", rec.Code, rec.Body)
	}
	var sess struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &sess)

	// 空文本的一轮：她还没说话，是这条支线刚开，印记要先说一句。
	rec = pblPost(t, h, cookie, "/api/v1/pbl/projects/"+pid+"/turn",
		`{"text":"","sessionId":"`+sess.ID+`"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("opening turn = %d, want 200; body=%s", rec.Code, rec.Body)
	}

	// 支线里应该只有印记那一条，不该凭空多出一条"她说的话"。
	rec = pblReq(t, h, cookie, "GET",
		"/api/v1/pbl/projects/"+pid+"/thread?session="+sess.ID, "")
	var msgs []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &msgs); err != nil {
		t.Fatalf("decode thread: %v — body=%s", err, rec.Body)
	}
	if len(msgs) != 1 {
		t.Fatalf("支线里有 %d 条消息，want 1（只有印记那一句）：%+v", len(msgs), msgs)
	}
	if msgs[0].Role != "ai" {
		t.Fatalf("支线第一条是 %q，want ai —— 这一层该由印记开口", msgs[0].Role)
	}
	if msgs[0].Content == "" {
		t.Fatal("印记开了口却什么也没说")
	}
}

// 主线上一句话都没有时，不给空文本——那是印记在自言自语。
func TestPblTurn_EmptyTextNeedsSomethingToReactTo(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, pblCoachSaying(`{"reply":"…"}`))
	pid := newProjectViaAPI(t, h, cookie)
	if rec := pblPost(t, h, cookie, "/api/v1/pbl/projects/"+pid+"/turn",
		`{"text":"   "}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("empty turn on an empty thread = %d, want 400", rec.Code)
	}
}
