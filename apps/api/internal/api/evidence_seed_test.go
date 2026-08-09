package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/store/sqlc"
)

// TestProposalToResearch_SeedsMapAndOpensReading — slice 4a: advancing
// proposal→essay seeds the 证据地图 backbone from the proposal (main RQ + one
// 子问题 lead per sub-question, confirmed edges), sets the essay track to the
// research stage, and opens the reading room (not the writing surface). Idempotent.
func TestProposalToResearch_SeedsMapAndOpensReading(t *testing.T) {
	h, cookie, pool := orchestratorHandler(t, `{"narrate":"好，我们去做研究。","tools":[]}`)
	q := sqlc.New(pool)
	ctx := context.Background()

	// Seed a proposal objective + a proposal track with 2 sub-questions.
	if _, err := q.UpsertProjectProposal(ctx, sqlc.UpsertProjectProposalParams{
		ProjectID: mustUUID(seedProjectID), Objective: "中国是否让地球更可持续", Reason: "r", Activities: "a", Resources: "res", Counterpoints: "c",
	}); err != nil {
		t.Fatalf("seed proposal: %v", err)
	}
	st := agent.DefaultStudioState()
	st.Started = true
	st.Stage = agent.StageProposalReview
	st.ProposalTrack = &agent.WritingTrack{
		Mode: agent.ModeGuided, SubQuestions: []agent.SubQuestion{{ID: "sq1", Text: "新能源投资的净效应"}, {ID: "sq2", Text: "碳排放总量的影响"}},
	}
	b, _ := json.Marshal(st)
	if err := q.SetStudioState(ctx, sqlc.SetStudioStateParams{ID: mustUUID(seedProjectID), StudioState: b}); err != nil {
		t.Fatalf("set studio state: %v", err)
	}

	base := "/api/v1/projects/" + seedProjectID
	advance := func() {
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", base+"/coach/advance", strings.NewReader(`{"to_status":"essay"}`)), cookie))
		if rr.Code != http.StatusOK {
			t.Fatalf("advance = %d — %s", rr.Code, rr.Body)
		}
	}
	advance()

	// The essay track is in research, and the state opens the reading room.
	raw, err := q.GetStudioState(ctx, mustUUID(seedProjectID))
	if err != nil {
		t.Fatalf("get studio state: %v", err)
	}
	var out agent.StudioState
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.EssayTrack == nil || out.EssayTrack.Stage != agent.EssayResearch {
		t.Fatalf("essay track should be research, got %+v", out.EssayTrack)
	}
	if out.OpenTool != agent.ToolReading {
		t.Fatalf("should open reading, got openTool=%v", out.OpenTool)
	}

	// The warren was seeded: main RQ + 2 sub-question leads + 2 子问题 edges.
	leads, err := q.ListExplorationLeads(ctx, mustUUID(seedProjectID))
	if err != nil {
		t.Fatalf("list leads: %v", err)
	}
	texts := map[string]bool{}
	for _, l := range leads {
		texts[l.Text] = true
	}
	if !texts["中国是否让地球更可持续"] || !texts["新能源投资的净效应"] || !texts["碳排放总量的影响"] {
		t.Fatalf("seed leads missing: %+v", texts)
	}
	edges, err := q.ListQuestionEdgesByProject(ctx, mustUUID(seedProjectID))
	if err != nil {
		t.Fatalf("list edges: %v", err)
	}
	subqEdges := 0
	for _, e := range edges {
		if e.Label == "子问题" && e.Status == "confirmed" {
			subqEdges++
		}
	}
	if subqEdges != 2 {
		t.Fatalf("want 2 confirmed 子问题 edges, got %d", subqEdges)
	}

	// Idempotent: a second advance does not duplicate the seed.
	leadCountBefore := len(leads)
	advance()
	leads2, _ := q.ListExplorationLeads(ctx, mustUUID(seedProjectID))
	if len(leads2) != leadCountBefore {
		t.Fatalf("re-advance duplicated the seed: %d → %d leads", leadCountBefore, len(leads2))
	}
}
