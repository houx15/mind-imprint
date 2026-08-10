package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/store/sqlc"
)

func explorationReviewProvider() gateway.Provider {
	return gateway.NewSequenceStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: "《A》最贴近核心问题；《B》关联较弱，可归档；子问题一缺反例。"},
		{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 20, OutputTokens: 15}},
		{Kind: gateway.EventDone, StopReason: gateway.StopStop},
	})
}

func TestExplorationReview(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: explorationReviewProvider(), ChatResolver: fakeResolver(), FastChatResolver: fakeResolver(),
		EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)
	base := "/api/v1/projects/" + seedProjectID

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", base+"/exploration/review", nil), cookie))
	if rr.Code != http.StatusOK {
		t.Fatalf("exploration review = %d — %s", rr.Code, rr.Body)
	}
	var out struct {
		Review string `json:"review"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &out)
	if out.Review == "" {
		t.Fatalf("expected a review, got empty — %s", rr.Body)
	}
}
