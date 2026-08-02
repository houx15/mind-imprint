package api_test

// card_reflect_test.go — Slice 2 · the card-reflect turn. An EMPTY card is a
// no-op (no persist, no spend, empty reply); a filled card persists a completed
// card_instance, appends the compiled student turn + the coach reply to the
// project thread, meters the coach call, and returns the reply.

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

func reflectHandler(t *testing.T) (http.Handler, *http.Cookie, *pgxpool.Pool) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: fakeProvider(), ChatResolver: fakeResolver(), SpecByID: cards.ByID,
	}).Handler()
	return h, signInSeed(t, pool), pool
}

func countChatMessages(t *testing.T, pool *pgxpool.Pool, projectID, role, surface string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM chat_message cm JOIN chat_thread ct ON cm.thread_id = ct.id
		 WHERE ct.seeded_project_id=$1 AND cm.role=$2 AND cm.surface=$3`,
		mustUUID(projectID), role, surface).Scan(&n); err != nil {
		t.Fatalf("countChatMessages: %v", err)
	}
	return n
}

// TestPostReflectProjectCard_EmptyIsNoOp — an empty card neither persists nor
// spends (铁律 · 不操纵 / 过程即数据 without fake work).
func TestPostReflectProjectCard_EmptyIsNoOp(t *testing.T) {
	h, cookie, pool := reflectHandler(t)
	base := "/api/v1/projects/" + seedProjectID

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", base+"/cards/reflect",
		strings.NewReader(`{"card_id":"question-card","field_values":{"entry_direction":"   "},"event_trace":[],"surface":"forming"}`)), cookie))
	if rr.Code != http.StatusOK {
		t.Fatalf("empty reflect = %d — %s", rr.Code, rr.Body)
	}
	var out struct {
		CardInstanceID string `json:"cardInstanceId"`
		Reply          string `json:"reply"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v — %s", err, rr.Body)
	}
	if out.CardInstanceID != "" || out.Reply != "" {
		t.Fatalf("empty card should return empty {cardInstanceId,reply}, got %+v", out)
	}
	if got := countCompletedCard(t, pool, seedProjectID, "question-card"); got != 0 {
		t.Fatalf("empty card must persist nothing, got %d", got)
	}
	if got := countLLMCallsByPurpose(t, pool, seedProjectID, "coach"); got != 0 {
		t.Fatalf("empty card must not spend, coach calls = %d", got)
	}
}

// TestPostReflectProjectCard_PersistsAndReplies — a filled card persists, leaves
// a student + assistant turn on the surface, meters one coach call, and returns
// a non-empty reply that responds to the card (the fakeProvider's reply).
func TestPostReflectProjectCard_PersistsAndReplies(t *testing.T) {
	h, cookie, pool := reflectHandler(t)
	base := "/api/v1/projects/" + seedProjectID

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", base+"/cards/reflect",
		strings.NewReader(`{"card_id":"question-card","field_values":{"entry_direction":"中国是否让地球更可持续？"},"event_trace":[],"surface":"forming"}`)), cookie))
	if rr.Code != http.StatusOK {
		t.Fatalf("reflect = %d — %s", rr.Code, rr.Body)
	}
	var out struct {
		CardInstanceID string `json:"cardInstanceId"`
		Reply          string `json:"reply"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v — %s", err, rr.Body)
	}
	if out.CardInstanceID == "" || out.Reply == "" {
		t.Fatalf("filled card should return a card id + reply, got %+v", out)
	}
	if got := countCompletedCard(t, pool, seedProjectID, "question-card"); got != 1 {
		t.Fatalf("completed question-card = %d, want 1", got)
	}
	if got := countEventsByType(t, pool, seedProjectID, "card_logged"); got != 1 {
		t.Fatalf("card_logged events = %d, want 1", got)
	}
	// The compiled card is persisted as a STUDENT turn on the surface, and the
	// coach reply as an ASSISTANT turn — both on "forming".
	if got := countChatMessages(t, pool, seedProjectID, "user", "forming"); got < 1 {
		t.Fatalf("student card turn = %d, want >=1", got)
	}
	if got := countChatMessages(t, pool, seedProjectID, "assistant", "forming"); got < 1 {
		t.Fatalf("assistant reply turn = %d, want >=1", got)
	}
	if got := countLLMCallsByPurpose(t, pool, seedProjectID, "coach"); got != 1 {
		t.Fatalf("coach llm_calls = %d, want 1", got)
	}
}

// TestPostReflectProjectCard_RejectsNonPersistable — a card in no allowlist is
// refused (400), so the endpoint can't be used to forge arbitrary card state.
func TestPostReflectProjectCard_RejectsNonPersistable(t *testing.T) {
	h, cookie, pool := reflectHandler(t)
	base := "/api/v1/projects/" + seedProjectID

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", base+"/cards/reflect",
		strings.NewReader(`{"card_id":"craap","field_values":{"x":"y"},"event_trace":[],"surface":"reading"}`)), cookie))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("non-persistable reflect = %d, want 400 — %s", rr.Code, rr.Body)
	}
	if got := countLLMCallsByPurpose(t, pool, seedProjectID, "coach"); got != 0 {
		t.Fatalf("rejected reflect must not spend, coach calls = %d", got)
	}
}
