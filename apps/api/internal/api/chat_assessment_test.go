package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/store/sqlc"
)

func countChatAssessmentLLMCalls(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM llm_call WHERE surface = 'chat' AND purpose = 'assessment'`).Scan(&n); err != nil {
		t.Fatalf("count chat assessment llm_call: %v", err)
	}
	return n
}

// countThreadEvaluations counts evaluation rows scoped to the given chat
// thread — used to prove a rejected assessment persists nothing (mirrors
// countProjectEvaluations in project_finish_test.go).
func countThreadEvaluations(t *testing.T, pool *pgxpool.Pool, threadID string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM evaluations WHERE thread_id = $1`, threadID).Scan(&n); err != nil {
		t.Fatalf("count thread evaluations: %v", err)
	}
	return n
}

// createThread POSTs /chat/threads and returns the new thread id.
func createThread(t *testing.T, h http.Handler, cookie *http.Cookie) string {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/chat/threads", strings.NewReader(`{"title":""}`)), cookie))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create thread = %d; body=%s", rec.Code, rec.Body)
	}
	var dto struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &dto); err != nil || dto.ID == "" {
		t.Fatalf("decode thread id: %v; body=%s", err, rec.Body)
	}
	return dto.ID
}

// TestGetChatAssessment_EmptyBeforeGenerate — "not yet assessed" is 200 + null,
// never a 404, and ZERO llm_call rows (the read path never calls a model).
func TestGetChatAssessment_EmptyBeforeGenerate(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool}).Handler()
	cookie := signInSeed(t, pool)
	threadID := createThread(t, h, cookie)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/chat/threads/"+threadID+"/assessment", nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	if strings.TrimSpace(rec.Body.String()) != "null" {
		t.Fatalf("GET body = %s, want literal null", rec.Body)
	}
	if n := countChatAssessmentLLMCalls(t, pool); n != 0 {
		t.Fatalf("llm_call rows after GET = %d, want 0", n)
	}
}

// TestGenerateChatAssessment_PersistsAtThreadScope — one flagship call, metered
// surface=chat/purpose=assessment, persisted so GET replays it with no 2nd call.
func TestGenerateChatAssessment_PersistsAtThreadScope(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: assessStubProvider(dualAxisReply), ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)
	threadID := createThread(t, h, cookie)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/chat/threads/"+threadID+"/assessment", strings.NewReader("")), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("POST = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var dto struct {
		DepthAxis struct {
			Subtotal int `json:"subtotal"`
		} `json:"depthAxis"`
		Narrative   string `json:"narrative"`
		GeneratedAt string `json:"generatedAt"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &dto); err != nil {
		t.Fatalf("decode DTO: %v — body=%s", err, rec.Body)
	}
	if dto.DepthAxis.Subtotal != 11 {
		t.Fatalf("depthAxis.subtotal = %d, want 11", dto.DepthAxis.Subtotal)
	}
	if dto.Narrative == "" || dto.GeneratedAt == "" {
		t.Fatalf("dto = %+v, want narrative + generatedAt", dto)
	}

	// surface=chat, purpose=assessment, tier=flagship (评估走旗舰模型绝不降级).
	var surface, purpose, tier string
	if err := pool.QueryRow(context.Background(),
		`SELECT surface, purpose, tier FROM llm_call WHERE surface='chat' AND purpose='assessment'
		 ORDER BY created_at DESC LIMIT 1`).Scan(&surface, &purpose, &tier); err != nil {
		t.Fatalf("read llm_call: %v", err)
	}
	if surface != "chat" || purpose != "assessment" {
		t.Fatalf("llm_call = (%s,%s), want (chat,assessment)", surface, purpose)
	}
	if tier != "flagship" {
		t.Fatalf("assessment ran on tier %q, want flagship — 评估走旗舰模型绝不降级", tier)
	}

	// Persisted at thread scope: GET replays it, still exactly one llm_call.
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, withCookie(httptest.NewRequest("GET", "/api/v1/chat/threads/"+threadID+"/assessment", nil), cookie))
	if strings.TrimSpace(rec2.Body.String()) == "null" {
		t.Fatal("GET after POST returned null — the report was not persisted at thread scope")
	}
	if n := countChatAssessmentLLMCalls(t, pool); n != 1 {
		t.Fatalf("llm_call rows after POST+GET = %d, want exactly 1", n)
	}
}

// TestGenerateChatAssessment_RecordsCostOnRejection — a rejected call still cost
// money, so the llm_call row exists even though nothing is persisted. The reject
// body carries "你应该这样写" — verified to trip enforcement's
// "rewritten-sentence-zh" rule (banned_phrasing.go) — in the minimal valid
// DualAxis shape (mirrors the project surface's reject fixture).
func TestGenerateChatAssessment_RecordsCostOnRejection(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider:     assessStubProvider(`{"depthAxis":{"dims":[{"code":"D1","score":2,"evidence":"你应该这样写：先摆结论"}]},"autonomyAxis":{"observation":"o"},"crossAxis":{"depthLevel":"L2"},"narrative":"n"}`),
		ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)
	threadID := createThread(t, h, cookie)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/chat/threads/"+threadID+"/assessment", strings.NewReader("")), cookie))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("POST (banned phrasing) = %d, want 422; body=%s", rec.Code, rec.Body)
	}
	if n := countChatAssessmentLLMCalls(t, pool); n != 1 {
		t.Fatalf("llm_call rows after rejection = %d, want 1 — a rejected call still cost money", n)
	}
	if n := countThreadEvaluations(t, pool, threadID); n != 0 {
		t.Fatalf("evaluation rows after rejection = %d, want 0 — nothing persisted", n)
	}
}

// TestChatAssessment_UnknownThread404s — an unowned/nonexistent thread id gets
// 404 and leaks nothing (loadOwnedThread resolves by caller).
func TestChatAssessment_UnknownThread404s(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool}).Handler()
	cookie := signInSeed(t, pool)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET",
		"/api/v1/chat/threads/00000000-0000-0000-0000-0000000000ff/assessment", nil), cookie))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET unknown thread = %d, want 404", rec.Code)
	}
}
