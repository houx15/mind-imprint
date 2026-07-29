package api_test

// compaction_test.go — S4 · the size-threshold compaction backstop
// (maybeCompactBackstop) and the digest's ride in the spine projection.
// Discipline under test: over-budget folds the oldest turns into a rolling
// conversation_digest (digest write PRECEDES fold); under-budget spends
// nothing; an empty/failed compose folds NOTHING; the projection carries the
// digest as a 会话记忆 block.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/store/sqlc"
)

// countFoldedCoachMsgs counts folded (folded_at IS NOT NULL) chat_message rows
// on the project's thread — the turns the backstop moved out of the window.
func countFoldedCoachMsgs(t *testing.T, pool *pgxpool.Pool, projectID string) int {
	t.Helper()
	q := `SELECT count(*) FROM chat_message cm
	      JOIN chat_thread ct ON cm.thread_id = ct.id
	      WHERE ct.seeded_project_id = $1 AND cm.folded_at IS NOT NULL`
	var n int
	if err := pool.QueryRow(context.Background(), q, mustUUID(projectID)).Scan(&n); err != nil {
		t.Fatalf("countFoldedCoachMsgs: %v", err)
	}
	return n
}

// longTurn is ~760 runes of filler so a handful of turns overflows digestRuneBudget.
var longTurn = strings.Repeat("这是一段较长的思考内容用来撑大上下文窗口。", 40)

func postCoachTurn(t *testing.T, h http.Handler, cookie *http.Cookie, base, scope, text string) {
	t.Helper()
	rr := httptest.NewRecorder()
	body := `{"scope":"` + scope + `","user_input":"` + text + `"}`
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", base+"/coach", strings.NewReader(body)), cookie))
	if rr.Code != http.StatusOK {
		t.Fatalf("coach turn = %d — %s", rr.Code, rr.Body)
	}
}

// TestMaybeCompactBackstop_FoldsOldestIntoDigest — enough long turns to overflow
// the budget fold the oldest turns into a digest row (turns_folded>0), leave the
// newest live, and meter exactly the coach_compact calls that ran.
func TestMaybeCompactBackstop_FoldsOldestIntoDigest(t *testing.T) {
	pool := newAPITestPool(t)
	q := sqlc.New(pool)
	h := New(Deps{Queries: q, Pool: pool, Provider: fakeProvider(), ChatResolver: fakeResolver(), SpecByID: cards.ByID}).Handler()
	cookie := signInSeed(t, pool)
	base := "/api/v1/projects/" + seedProjectID

	for i := 0; i < 12; i++ {
		postCoachTurn(t, h, cookie, base, "writing", longTurn)
	}

	d, err := q.GetConversationDigest(context.Background(), mustUUID(seedProjectID))
	if err != nil {
		t.Fatalf("expected a conversation_digest row after overflow: %v", err)
	}
	if d.TurnsFolded <= 0 {
		t.Fatalf("turns_folded = %d, want > 0", d.TurnsFolded)
	}
	if strings.TrimSpace(d.Prose) == "" {
		t.Fatalf("digest prose is empty")
	}
	if got := countFoldedCoachMsgs(t, pool, seedProjectID); got == 0 {
		t.Fatalf("expected folded messages after overflow, got 0")
	}
	if got := countLLMCallsByPurpose(t, pool, seedProjectID, "coach_compact"); got == 0 {
		t.Fatalf("expected coach_compact to be metered, got 0")
	}
	// The newest turns stay live (keepLastN): not everything folded.
	if got := countCoachMsgs(t, pool, seedProjectID, "writing", true); got == 0 {
		t.Fatalf("expected some active (non-folded) turns to remain, got 0")
	}
}

// TestMaybeCompactBackstop_UnderBudgetNoSpend — one short turn stays under
// budget: no digest, no fold, no compact spend.
func TestMaybeCompactBackstop_UnderBudgetNoSpend(t *testing.T) {
	pool := newAPITestPool(t)
	q := sqlc.New(pool)
	h := New(Deps{Queries: q, Pool: pool, Provider: fakeProvider(), ChatResolver: fakeResolver(), SpecByID: cards.ByID}).Handler()
	cookie := signInSeed(t, pool)
	base := "/api/v1/projects/" + seedProjectID

	postCoachTurn(t, h, cookie, base, "writing", "你好")

	if _, err := q.GetConversationDigest(context.Background(), mustUUID(seedProjectID)); err == nil {
		t.Fatalf("expected NO digest row under budget")
	}
	if got := countFoldedCoachMsgs(t, pool, seedProjectID); got != 0 {
		t.Fatalf("expected 0 folded under budget, got %d", got)
	}
	if got := countLLMCallsByPurpose(t, pool, seedProjectID, "coach_compact"); got != 0 {
		t.Fatalf("expected 0 coach_compact spend under budget, got %d", got)
	}
}

// emptyTextProvider yields no text and no usage — a successful-but-empty
// completion. The coach falls back; the compact compose returns empty prose.
func emptyTextProvider() gateway.Provider {
	return gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventDone, StopReason: gateway.StopStop},
	})
}

// TestMaybeCompactBackstop_EmptyComposeFoldsNothing — when the compose yields no
// durable prose, the backstop folds NOTHING (digest-before-fold invariant) and
// records no digest row, even though the window is over budget.
func TestMaybeCompactBackstop_EmptyComposeFoldsNothing(t *testing.T) {
	pool := newAPITestPool(t)
	q := sqlc.New(pool)
	h := New(Deps{Queries: q, Pool: pool, Provider: emptyTextProvider(), ChatResolver: fakeResolver(), SpecByID: cards.ByID}).Handler()
	cookie := signInSeed(t, pool)
	base := "/api/v1/projects/" + seedProjectID

	for i := 0; i < 12; i++ {
		postCoachTurn(t, h, cookie, base, "writing", longTurn) // 200 with fallback replies
	}

	if _, err := q.GetConversationDigest(context.Background(), mustUUID(seedProjectID)); err == nil {
		t.Fatalf("empty compose must not create a digest row")
	}
	if got := countFoldedCoachMsgs(t, pool, seedProjectID); got != 0 {
		t.Fatalf("empty compose must fold nothing, got %d folded", got)
	}
	if got := countLLMCallsByPurpose(t, pool, seedProjectID, "coach_compact"); got != 0 {
		t.Fatalf("empty (0-token) compose must not meter, got %d", got)
	}
}

// TestSpineProjection_IncludesDigest — a stored digest rides the projection as a
// 会话记忆 block; absent, no such block appears.
func TestSpineProjection_IncludesDigest(t *testing.T) {
	api, _, _, q := projectionTestHandler(t)
	ctx := context.Background()

	proj, err := api.BuildSpineProjectionForTest(ctx, mustUUID(seedProjectID))
	if err != nil {
		t.Fatalf("projection: %v", err)
	}
	if strings.Contains(proj, "会话记忆") {
		t.Fatalf("no digest yet, projection should not carry 会话记忆:\n%s", proj)
	}

	if err := q.UpsertConversationDigest(ctx, sqlc.UpsertConversationDigestParams{
		ProjectID: mustUUID(seedProjectID), Prose: "早前：学生已把公众号线索溯源到 NASA Ames。", TurnsFolded: 4,
	}); err != nil {
		t.Fatalf("upsert digest: %v", err)
	}
	proj2, err := api.BuildSpineProjectionForTest(ctx, mustUUID(seedProjectID))
	if err != nil {
		t.Fatalf("projection 2: %v", err)
	}
	if !strings.Contains(proj2, "会话记忆") || !strings.Contains(proj2, "NASA Ames") {
		t.Fatalf("projection missing digest block:\n%s", proj2)
	}
}
