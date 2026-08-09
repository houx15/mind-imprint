package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/store/sqlc"
)

// seedResearchProject seeds the proposal + advances proposal→essay (which seeds
// the warren), then returns the first sub-question lead's id + a paper reference
// attached under it. Uses the given handler/pool.
func seedResearchWithPaper(t *testing.T, h http.Handler, cookie *http.Cookie, q *sqlc.Queries) (subQID string, refID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	if _, err := q.UpsertProjectProposal(ctx, sqlc.UpsertProjectProposalParams{
		ProjectID: mustUUID(seedProjectID), Objective: "主问题", Reason: "r", Activities: "a", Resources: "res", Counterpoints: "c",
	}); err != nil {
		t.Fatalf("seed proposal: %v", err)
	}
	st := agent.DefaultStudioState()
	st.Started = true
	st.Stage = agent.StageProposalReview
	st.ProposalTrack = &agent.WritingTrack{SubQuestions: []agent.SubQuestion{{ID: "sq1", Text: "子问题一"}}}
	b, _ := json.Marshal(st)
	if err := q.SetStudioState(ctx, sqlc.SetStudioStateParams{ID: mustUUID(seedProjectID), StudioState: b}); err != nil {
		t.Fatalf("set state: %v", err)
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+seedProjectID+"/coach/advance", strings.NewReader(`{"to_status":"essay"}`)), cookie))
	if rr.Code != http.StatusOK {
		t.Fatalf("advance = %d — %s", rr.Code, rr.Body)
	}

	leads, _ := q.ListExplorationLeads(ctx, mustUUID(seedProjectID))
	for _, l := range leads {
		if l.Text == "子问题一" {
			subQID = l.ID.String()
		}
	}
	if subQID == "" {
		t.Fatal("sub-question lead not seeded")
	}

	ref, err := q.CreateReference(ctx, sqlc.CreateReferenceParams{
		ProjectID: mustUUID(seedProjectID), Title: "一篇支持论文", Tags: []byte("[]"), SearchHints: []byte("[]"),
	})
	if err != nil {
		t.Fatalf("create reference: %v", err)
	}
	refID = ref.ID
	// A paper lead under the sub-question, connected to the reference.
	sq := mustUUID(subQID)
	if _, err := q.CreateExplorationLead(ctx, sqlc.CreateExplorationLeadParams{
		ProjectID:            mustUUID(seedProjectID),
		Text:                 "一篇支持论文",
		Status:               "connected",
		Origin:               "manual",
		ConnectedReferenceID: pgtype.UUID{Bytes: ref.ID, Valid: true},
		ParentLeadID:         pgtype.UUID{Bytes: sq, Valid: true},
	}); err != nil {
		t.Fatalf("create paper lead: %v", err)
	}
	return subQID, refID
}

func TestEvidenceMap_PatchAndProject(t *testing.T) {
	h, cookie, pool := orchestratorHandler(t, `{"narrate":"好。","tools":[]}`)
	q := sqlc.New(pool)
	subQID, refID := seedResearchWithPaper(t, h, cookie, q)
	base := "/api/v1/projects/" + seedProjectID

	// PATCH the paper's evidence (support) + triage (red).
	rrEv := httptest.NewRecorder()
	h.ServeHTTP(rrEv, withCookie(httptest.NewRequest("PATCH", base+"/references/"+refID.String()+"/evidence",
		strings.NewReader(`{"nature":"support","argument":"投资降低碳排","finding":"装机量上升"}`)), cookie))
	if rrEv.Code != http.StatusOK {
		t.Fatalf("patch evidence = %d — %s", rrEv.Code, rrEv.Body)
	}
	rrTr := httptest.NewRecorder()
	h.ServeHTTP(rrTr, withCookie(httptest.NewRequest("PATCH", base+"/references/"+refID.String()+"/triage", strings.NewReader(`{"triage":"red"}`)), cookie))
	if rrTr.Code != http.StatusOK {
		t.Fatalf("patch triage = %d — %s", rrTr.Code, rrTr.Body)
	}

	// GET evidence-map → the sub-question shows the paper with nature + triage.
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("GET", base+"/evidence-map", nil), cookie))
	if rr.Code != http.StatusOK {
		t.Fatalf("evidence-map = %d — %s", rr.Code, rr.Body)
	}
	var m struct {
		MainQuestion string `json:"mainQuestion"`
		SubQuestions []struct {
			ID     string `json:"id"`
			Text   string `json:"text"`
			Papers []struct {
				Title   string `json:"title"`
				Nature  string `json:"nature"`
				Triage  string `json:"triage"`
				HasNote bool   `json:"hasNote"`
			} `json:"papers"`
		} `json:"subQuestions"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &m); err != nil {
		t.Fatalf("decode: %v — %s", err, rr.Body)
	}
	if m.MainQuestion != "主问题" || len(m.SubQuestions) != 1 || m.SubQuestions[0].ID != subQID {
		t.Fatalf("map wrong: %+v", m)
	}
	if len(m.SubQuestions[0].Papers) != 1 {
		t.Fatalf("want 1 paper under the sub-question, got %d", len(m.SubQuestions[0].Papers))
	}
	p := m.SubQuestions[0].Papers[0]
	if p.Nature != "support" || p.Triage != "red" || !p.HasNote {
		t.Fatalf("paper facets wrong: %+v", p)
	}

	// Archive the paper → it drops off the map.
	rrAr := httptest.NewRecorder()
	h.ServeHTTP(rrAr, withCookie(httptest.NewRequest("POST", base+"/references/"+refID.String()+"/archive", strings.NewReader(`{}`)), cookie))
	if rrAr.Code != http.StatusOK {
		t.Fatalf("archive = %d — %s", rrAr.Code, rrAr.Body)
	}
	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, withCookie(httptest.NewRequest("GET", base+"/evidence-map", nil), cookie))
	_ = json.Unmarshal(rr2.Body.Bytes(), &m)
	if len(m.SubQuestions[0].Papers) != 0 {
		t.Fatalf("archived paper should drop off the map, got %d", len(m.SubQuestions[0].Papers))
	}
}

func TestEvidenceMap_SaturationReview(t *testing.T) {
	// A provider whose reply is a saturation verdict.
	prov := gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: `{"saturated":false,"why":"只有支持材料","gaps":["缺一个反例"]}`},
		{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 20, OutputTokens: 8}},
		{Kind: gateway.EventDone, StopReason: gateway.StopStop},
	})
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: prov, ChatResolver: fakeResolver(), FastChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)
	q := sqlc.New(pool)
	subQID, _ := seedResearchWithPaper(t, h, cookie, q)

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+seedProjectID+"/evidence-map/subquestions/"+subQID+"/review", strings.NewReader("")), cookie))
	if rr.Code != http.StatusOK {
		t.Fatalf("review = %d — %s", rr.Code, rr.Body)
	}
	var v struct {
		SubQuestionID string   `json:"subQuestionId"`
		Saturated     bool     `json:"saturated"`
		Why           string   `json:"why"`
		Gaps          []string `json:"gaps"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode verdict: %v — %s", err, rr.Body)
	}
	if v.SubQuestionID != subQID || v.Saturated || v.Why == "" || len(v.Gaps) != 1 {
		t.Fatalf("verdict = %+v — %s", v, rr.Body)
	}
}
