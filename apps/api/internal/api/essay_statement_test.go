package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/store/sqlc"
)

func seedStatementProject(t *testing.T, q *sqlc.Queries) {
	t.Helper()
	ctx := context.Background()
	st := agent.DefaultStudioState()
	st.Started = true
	st.Stage = agent.StageBodyWriting
	st.EssayTrack = &agent.EssayTrack{Stage: agent.EssayStatement}
	st.ProposalTrack = &agent.WritingTrack{SubQuestions: []agent.SubQuestion{{ID: "a", Text: "论点甲"}, {ID: "b", Text: "论点乙"}}}
	b, _ := json.Marshal(st)
	if err := q.SetStudioState(ctx, sqlc.SetStudioStateParams{ID: mustUUID(seedProjectID), StudioState: b}); err != nil {
		t.Fatalf("set state: %v", err)
	}
}

func TestEssayStatement_TrackWalk(t *testing.T) {
	h, cookie, pool := proposalTrackHandler(t, guideCardProvider()) // reuses the guide-card stub
	q := sqlc.New(pool)
	seedStatementProject(t, q)
	base := "/api/v1/projects/" + seedProjectID

	req := func(method, path, body string) map[string]any {
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, withCookie(httptest.NewRequest(method, base+path, strings.NewReader(body)), cookie))
		if rr.Code != http.StatusOK {
			t.Fatalf("%s %s = %d — %s", method, path, rr.Code, rr.Body)
		}
		var out map[string]any
		_ = json.Unmarshal(rr.Body.Bytes(), &out)
		return out
	}

	// Fresh GET: not started, total = outline + 2 claims + 4 tail = 7, current = outline.
	got := req("GET", "/essay-statement", "")
	if got["started"] != false || got["total"].(float64) != 7 || got["key"] != "outline" {
		t.Fatalf("fresh essay statement wrong: %+v", got)
	}
	if got["card"] != nil {
		t.Fatalf("no card before started")
	}

	// Start (ready gate) → started, index 0, card generated.
	got = req("POST", "/essay-statement/start", "")
	if got["started"] != true || got["card"] == nil {
		t.Fatalf("after start expected started + card: %+v", got)
	}

	// Advance to the first claim step.
	got = req("POST", "/essay-statement/advance", `{"dir":"next"}`)
	if got["kind"] != "subq" || got["key"] != "claim:a" {
		t.Fatalf("after next expected claim:a, got %+v", got)
	}
}

func TestEssayStatement_ReviewProducesEssayAnnotations(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: annotationProvider(), ChatResolver: fakeResolver(), FastChatResolver: fakeResolver(),
		EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)
	q := sqlc.New(pool)
	seedStatementProject(t, q)
	base := "/api/v1/projects/" + seedProjectID

	// Seed the essay buffer.
	rrBuf := httptest.NewRecorder()
	h.ServeHTTP(rrBuf, withCookie(httptest.NewRequest("PUT", base+"/buffer?doc=essay", strings.NewReader(`{"content":"正文。中国一定会成功。"}`)), cookie))
	if rrBuf.Code != http.StatusNoContent && rrBuf.Code != http.StatusOK {
		t.Fatalf("PUT buffer = %d — %s", rrBuf.Code, rrBuf.Body)
	}

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", base+"/essay-statement/review", strings.NewReader(`{}`)), cookie))
	if rr.Code != http.StatusOK {
		t.Fatalf("review = %d — %s", rr.Code, rr.Body)
	}
	if len(decodeAnnotations(t, rr.Body.Bytes())) == 0 {
		t.Fatalf("essay statement review should produce 批注 — %s", rr.Body)
	}
}
