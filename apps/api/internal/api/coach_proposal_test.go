package api_test

// coach_proposal_test.go — S4 · cross-phase card proposing. A coach turn on an
// eligible surface whose text hits a semantic moment attaches a proposal to the
// {reply} JSON and records a coach_proposed event; a surface where argument
// cards don't belong (or a none moment) attaches nothing.

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
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/store/sqlc"
)

// momentReplyProvider replays one script per call: the coach reply call and the
// classify call both see `id`. For the classify call, `id` must be a moment id
// (e.g. "fact_opinion") to fire a proposal, or "none" to fire nothing.
func momentReplyProvider(id string) gateway.Provider {
	return gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: id},
		{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 20, OutputTokens: 5}},
		{Kind: gateway.EventDone, StopReason: gateway.StopStop},
	})
}

func countCoachProposedEvents(t *testing.T, pool *pgxpool.Pool, projectID string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM event WHERE project_id=$1 AND type='coach_proposed'`,
		mustUUID(projectID)).Scan(&n); err != nil {
		t.Fatalf("countCoachProposedEvents: %v", err)
	}
	return n
}

func coachProposalHandler(t *testing.T, id string) (http.Handler, *http.Cookie, *pgxpool.Pool) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: momentReplyProvider(id), ChatResolver: fakeResolver(), SpecByID: cards.ByID,
	}).Handler()
	return h, signInSeed(t, pool), pool
}

func TestPostCoach_ReturnsProposalAndRecordsEvent(t *testing.T) {
	h, cookie, pool := coachProposalHandler(t, "fact_opinion")
	base := "/api/v1/projects/" + seedProjectID

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", base+"/coach",
		strings.NewReader(`{"scope":"writing","user_input":"我觉得中国显然让地球更可持续了，这就是事实"}`)), cookie))
	if rr.Code != http.StatusOK {
		t.Fatalf("coach = %d — %s", rr.Code, rr.Body)
	}
	var resp struct {
		Reply    string `json:"reply"`
		Proposal *struct {
			CardID    string `json:"cardId"`
			NudgeText string `json:"nudgeText"`
		} `json:"proposal"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v — %s", err, rr.Body)
	}
	if resp.Proposal == nil || resp.Proposal.CardID != "fact-opinion-value" {
		t.Fatalf("want fact-opinion-value proposal, got %+v — %s", resp.Proposal, rr.Body)
	}
	if resp.Proposal.NudgeText == "" {
		t.Fatalf("proposal must carry a student-facing nudge")
	}
	if got := countCoachProposedEvents(t, pool, seedProjectID); got != 1 {
		t.Fatalf("coach_proposed events = %d, want 1", got)
	}
}

func TestPostCoach_NoProposalOnReadingSurface(t *testing.T) {
	// The reading surface has its own summon ladder — the coach must not also
	// offer argument cards there (surface gate → no classify, no proposal).
	h, cookie, pool := coachProposalHandler(t, "fact_opinion")
	base := "/api/v1/projects/" + seedProjectID

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", base+"/coach",
		strings.NewReader(`{"scope":"reading","user_input":"我觉得中国显然让地球更可持续了，这就是事实"}`)), cookie))
	if rr.Code != http.StatusOK {
		t.Fatalf("coach = %d — %s", rr.Code, rr.Body)
	}
	if strings.Contains(rr.Body.String(), `"proposal"`) {
		t.Fatalf("reading surface must not carry a proposal: %s", rr.Body)
	}
	if got := countCoachProposedEvents(t, pool, seedProjectID); got != 0 {
		t.Fatalf("coach_proposed events on reading = %d, want 0", got)
	}
	// The surface gate fires BEFORE the classifier: no classify spend either.
	if got := countLLMCallsByPurpose(t, pool, seedProjectID, "classify"); got != 0 {
		t.Fatalf("reading surface must not classify, got %d classify calls", got)
	}
}
