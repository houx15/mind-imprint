package api_test

// chat_test.go — Task 5 (Slice 11): handler tests for the Chat surface
// (thread list/create, message history, the SSE turn, thin card
// submit/skip). Mirrors studioturn_test.go's SSE-driving pattern (real
// testcontainers Postgres + fakeProvider/fakeResolver) and
// assessment_test.go's plain-JSON handler pattern.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/store/sqlc"
)

// TestChatThreads_CreateListRoundtrip — POST /chat/threads then GET
// /chat/threads returns the created thread.
func TestChatThreads_CreateListRoundtrip(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool}).Handler()
	cookie := signInSeed(t, pool)

	// 401 unauth.
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/api/v1/chat/threads", nil))
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("unauth: want 401, got %d", rr.Code)
	}

	rr = httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/chat/threads", strings.NewReader(`{"title":"我的对话"}`))
	h.ServeHTTP(rr, withCookie(req, cookie))
	if rr.Code != http.StatusCreated {
		t.Fatalf("create thread: %d — %s", rr.Code, rr.Body.String())
	}
	var created ChatThreadDTO
	if err := json.Unmarshal(rr.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created thread: %v", err)
	}
	if created.Title != "我的对话" {
		t.Fatalf("title = %q, want 我的对话", created.Title)
	}
	if created.ID == "" || created.CreatedAt == "" {
		t.Fatalf("created thread missing id/createdAt: %+v", created)
	}

	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, withCookie(httptest.NewRequest("GET", "/api/v1/chat/threads", nil), cookie))
	if rr2.Code != http.StatusOK {
		t.Fatalf("list threads: %d — %s", rr2.Code, rr2.Body.String())
	}
	var list []ChatThreadDTO
	if err := json.Unmarshal(rr2.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	found := false
	for _, th := range list {
		if th.ID == created.ID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("created thread %s not found in list: %+v", created.ID, list)
	}
}

// TestChatMessages_OwnershipHidden — GET .../messages on another user's
// thread 404s (ownership hidden as not-found, never 403), same convention as
// every other owned resource in this package.
func TestChatMessages_OwnershipHidden(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool}).Handler()
	q := sqlc.New(pool)
	other := createStudent(t, pool, SeedSchoolID, "chat-other@demo.local")
	th, err := q.CreateStandaloneThread(context.Background(), sqlc.CreateStandaloneThreadParams{
		UserID: other, Title: "别人的对话",
	})
	if err != nil {
		t.Fatalf("create thread: %v", err)
	}
	cookie := signInSeed(t, pool) // Phoebe (seed), not `other`

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("GET", "/api/v1/chat/threads/"+th.ID.String()+"/messages", nil), cookie))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("ownership: want 404, got %d — %s", rr.Code, rr.Body.String())
	}

	// A bogus/non-existent thread id also 404s (same hidden-as-not-found path).
	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, withCookie(httptest.NewRequest("GET", "/api/v1/chat/threads/00000000-0000-0000-0000-0000000009ff/messages", nil), cookie))
	if rr2.Code != http.StatusNotFound {
		t.Fatalf("foreign: want 404, got %d — %s", rr2.Code, rr2.Body.String())
	}
}

// TestChatTurn_TextOnly — a plain-text turn (no URL, no un-carded article)
// yields a text frame + done, no card frame, and records exactly one
// llm_call row with surface=chat and project_id NULL.
func TestChatTurn_TextOnly(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: fakeProvider(), ChatResolver: fakeResolver(), SpecByID: cards.ByID,
	}).Handler()
	q := sqlc.New(pool)
	cookie := signInSeed(t, pool)
	th, err := q.CreateStandaloneThread(context.Background(), sqlc.CreateStandaloneThreadParams{
		UserID: SeedUserID, Title: "对话",
	})
	if err != nil {
		t.Fatalf("create thread: %v", err)
	}

	// 401 unauth.
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("POST", "/api/v1/chat/threads/"+th.ID.String()+"/turn", strings.NewReader(`{"user_input":"hi there enough"}`)))
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("unauth: want 401, got %d", rr.Code)
	}

	rr = httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/chat/threads/"+th.ID.String()+"/turn", strings.NewReader(`{"user_input":"你好，我想聊聊这次的论文题目"}`))
	h.ServeHTTP(rr, withCookie(req, cookie))
	if rr.Code != http.StatusOK {
		t.Fatalf("turn: %d — %s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	if !strings.Contains(body, "event: text") {
		t.Fatalf("expected a text frame:\n%s", body)
	}
	if !strings.Contains(body, "event: done") {
		t.Fatalf("expected a done frame:\n%s", body)
	}
	if strings.Contains(body, "event: card") {
		t.Fatalf("did not expect a card frame (no url, no un-carded article):\n%s", body)
	}

	// 404 non-owned thread.
	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, withCookie(httptest.NewRequest("POST", "/api/v1/chat/threads/00000000-0000-0000-0000-0000000009ff/turn", strings.NewReader(`{"user_input":"whatever long enough"}`)), cookie))
	if rr2.Code != http.StatusNotFound {
		t.Fatalf("foreign: want 404, got %d", rr2.Code)
	}

	// One llm_call row recorded, surface=chat, project_id NULL.
	var surface string
	var projectIDNull bool
	err = pool.QueryRow(context.Background(),
		`SELECT surface, project_id IS NULL FROM llm_call WHERE user_id = $1 ORDER BY created_at DESC LIMIT 1`, SeedUserID,
	).Scan(&surface, &projectIDNull)
	if err != nil {
		t.Fatalf("query llm_call: %v", err)
	}
	if surface != "chat" || !projectIDNull {
		t.Fatalf("llm_call surface=%q projectIDNull=%v, want chat/true", surface, projectIDNull)
	}
}

// TestChatTurn_URLMessageSurfacesCraapCard — a message carrying an http(s)
// URL mints a thread article material and (since the thread has no craap
// card_instance yet) surfaces a fresh CRAAP offer in the SAME turn — the SSE
// stream carries a `card` frame in addition to text/done.
func TestChatTurn_URLMessageSurfacesCraapCard(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: fakeProvider(), ChatResolver: fakeResolver(), SpecByID: cards.ByID,
	}).Handler()
	q := sqlc.New(pool)
	cookie := signInSeed(t, pool)
	th, err := q.CreateStandaloneThread(context.Background(), sqlc.CreateStandaloneThreadParams{
		UserID: SeedUserID, Title: "对话",
	})
	if err != nil {
		t.Fatalf("create thread: %v", err)
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/chat/threads/"+th.ID.String()+"/turn", strings.NewReader(`{"user_input":"这篇讲得挺好 https://www.nature.com/articles/example"}`))
	h.ServeHTTP(rr, withCookie(req, cookie))
	if rr.Code != http.StatusOK {
		t.Fatalf("turn: %d — %s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	if !strings.Contains(body, "event: card") || !strings.Contains(body, `"card_id":"craap"`) {
		t.Fatalf("expected a craap card event:\n%s", body)
	}

	mats, err := q.ListMaterialsByThread(context.Background(), pgUUID(th.ID))
	if err != nil {
		t.Fatalf("ListMaterialsByThread: %v", err)
	}
	if len(mats) != 1 || mats[0].Kind != "article" {
		t.Fatalf("expected one article material minted from the pasted url, got %+v", mats)
	}
}

// TestChatCard_SubmitAndSkip — submit flips a thread card to completed;
// skip flips a (different) thread card to skipped.
func TestChatCard_SubmitAndSkip(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, SpecByID: cards.ByID}).Handler()
	q := sqlc.New(pool)
	cookie := signInSeed(t, pool)
	th, err := q.CreateStandaloneThread(context.Background(), sqlc.CreateStandaloneThreadParams{
		UserID: SeedUserID, Title: "对话",
	})
	if err != nil {
		t.Fatalf("create thread: %v", err)
	}
	ci1, err := q.CreateThreadCardInstance(context.Background(), sqlc.CreateThreadCardInstanceParams{
		ThreadID: pgUUID(th.ID), CardID: "craap", Status: "proposed",
	})
	if err != nil {
		t.Fatalf("create card instance 1: %v", err)
	}
	ci2, err := q.CreateThreadCardInstance(context.Background(), sqlc.CreateThreadCardInstanceParams{
		ThreadID: pgUUID(th.ID), CardID: "craap", Status: "proposed",
	})
	if err != nil {
		t.Fatalf("create card instance 2: %v", err)
	}

	// submit -> completed.
	rr := httptest.NewRecorder()
	submitBody := `{"field_values":{"q":"a"},"event_trace":[{"kind":"submit"}],"anchors":[]}`
	req := httptest.NewRequest("POST", "/api/v1/chat/threads/"+th.ID.String()+"/cards/"+ci1.ID.String()+"/submit", strings.NewReader(submitBody))
	h.ServeHTTP(rr, withCookie(req, cookie))
	if rr.Code != http.StatusOK {
		t.Fatalf("submit: %d — %s", rr.Code, rr.Body.String())
	}
	var submitResp struct {
		CardStatus string `json:"card_status"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &submitResp); err != nil {
		t.Fatalf("decode submit resp: %v", err)
	}
	if submitResp.CardStatus != "completed" {
		t.Fatalf("card_status = %q, want completed", submitResp.CardStatus)
	}
	got1, err := q.GetCardInstance(context.Background(), ci1.ID)
	if err != nil {
		t.Fatalf("GetCardInstance 1: %v", err)
	}
	if got1.Status != "completed" {
		t.Fatalf("persisted status 1 = %q, want completed", got1.Status)
	}

	// skip -> skipped.
	rr2 := httptest.NewRecorder()
	req2 := httptest.NewRequest("POST", "/api/v1/chat/threads/"+th.ID.String()+"/cards/"+ci2.ID.String()+"/skip", nil)
	h.ServeHTTP(rr2, withCookie(req2, cookie))
	if rr2.Code != http.StatusOK {
		t.Fatalf("skip: %d — %s", rr2.Code, rr2.Body.String())
	}
	var skipResp struct {
		CardStatus string `json:"card_status"`
	}
	if err := json.Unmarshal(rr2.Body.Bytes(), &skipResp); err != nil {
		t.Fatalf("decode skip resp: %v", err)
	}
	if skipResp.CardStatus != "skipped" {
		t.Fatalf("card_status = %q, want skipped", skipResp.CardStatus)
	}
	got2, err := q.GetCardInstance(context.Background(), ci2.ID)
	if err != nil {
		t.Fatalf("GetCardInstance 2: %v", err)
	}
	if got2.Status != "skipped" {
		t.Fatalf("persisted status 2 = %q, want skipped", got2.Status)
	}

	// ownership: another user's thread's card 404s on submit/skip too.
	other := createStudent(t, pool, SeedSchoolID, "chat-cardowner@demo.local")
	otherCookie := signInAs(t, pool, other)
	rr3 := httptest.NewRecorder()
	req3 := httptest.NewRequest("POST", "/api/v1/chat/threads/"+th.ID.String()+"/cards/"+ci1.ID.String()+"/skip", nil)
	h.ServeHTTP(rr3, withCookie(req3, otherCookie))
	if rr3.Code != http.StatusNotFound {
		t.Fatalf("cross-user card skip: want 404, got %d — %s", rr3.Code, rr3.Body.String())
	}
}
