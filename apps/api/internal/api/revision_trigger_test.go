package api_test

// revision_trigger_test.go — Task 3 of the revision-recording plan (spec
// 2026-08-11-revision-recording): drives the real HTTP endpoints that carry
// the Mechanism-1 ask_feedback trigger (orderReview, reviewEssayStatement,
// postReflectProjectCard, postCoach scope=writing/proposal_review) and
// asserts a revision_checkpoint row landed. Lives in package api_test (the
// external test package used by every other endpoint test in this
// directory) — recordCheckpoint/recordCheckpoints themselves are covered
// in-package by revision_checkpoint_test.go (Task 2); this file only proves
// the wiring at each trigger call site.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/store/sqlc"
)

// hasCheckpoint reports whether rows contains a checkpoint for the given
// artifact type + trigger.
func hasCheckpoint(rows []sqlc.RevisionCheckpoint, artifactType, trigger string) bool {
	for _, r := range rows {
		if r.ArtifactType == artifactType && r.Trigger == trigger {
			return true
		}
	}
	return false
}

// TestOrderReview_RecordsDraftCheckpoint — ordering a whole-draft review over
// a freshly committed snapshot (spec §12) must record a "draft" /
// "ask_feedback" checkpoint alongside the existing review_item persistence,
// so the revision trajectory captures the moment the student asked for
// feedback on their draft.
func TestOrderReview_RecordsDraftCheckpoint(t *testing.T) {
	pool := newAPITestPool(t)
	reply := `[{"criterion_code":"表E","band":"5–6 段","evidence":"第2段接住反方","missing":"跳步没补","fix":"补上定义"}]`
	h := New(Deps{
		Queries:      sqlc.New(pool),
		Pool:         pool,
		Provider:     reviewStubProvider(reply),
		ChatResolver: fakeResolver(),
	}).Handler()
	cookie := signInSeed(t, pool)
	projectID := materialsTestProjectID

	// Commit an in-band snapshot first (orderReview reviews a committed draft).
	content := strings.Repeat("字", 1600)
	recCommit := httptest.NewRecorder()
	reqCommit := withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+projectID+"/snapshots",
		strings.NewReader(`{"content":`+strconv.Quote(content)+`}`)), cookie)
	h.ServeHTTP(recCommit, reqCommit)
	if recCommit.Code != http.StatusCreated {
		t.Fatalf("commit snapshot = %d, want 201; body=%s", recCommit.Code, recCommit.Body)
	}
	var snap struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(recCommit.Body.Bytes(), &snap); err != nil {
		t.Fatalf("decode snapshot: %v", err)
	}

	recReview := httptest.NewRecorder()
	reqReview := withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+projectID+"/snapshots/"+snap.ID+"/review",
		strings.NewReader("")), cookie)
	h.ServeHTTP(recReview, reqReview)
	if recReview.Code != http.StatusOK {
		t.Fatalf("order review = %d, want 200; body=%s", recReview.Code, recReview.Body)
	}

	rows, err := sqlc.New(pool).ListRevisionCheckpoints(context.Background(), mustUUID(projectID))
	if err != nil {
		t.Fatalf("ListRevisionCheckpoints: %v", err)
	}
	if !hasCheckpoint(rows, "draft", "ask_feedback") {
		t.Fatalf("order-review did not record a draft ask_feedback checkpoint; rows=%+v", rows)
	}
}

// TestWritingCoachTurn_RecordsSnippetsCheckpoint — a coach turn on
// scope=writing must record snippets/draft/outline ask_feedback checkpoints
// (the writing-room artifacts the student is asking 印记 about), alongside
// the existing coach_turn event + reply persistence.
func TestWritingCoachTurn_RecordsSnippetsCheckpoint(t *testing.T) {
	prov := gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: `这段论点还需要一个反例来接住。`},
		{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 10, OutputTokens: 5}},
		{Kind: gateway.EventDone, StopReason: gateway.StopStop},
	})
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: prov, ChatResolver: fakeResolver(), FastChatResolver: fakeResolver(),
	}).Handler()
	cookie := signInSeed(t, pool)
	projectID := seedProjectID

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+projectID+"/coach",
		strings.NewReader(`{"scope":"writing","user_input":"帮我看这段论点够不够有力，别改写"}`)), cookie))
	if rr.Code != http.StatusOK {
		t.Fatalf("coach = %d, want 200; body=%s", rr.Code, rr.Body)
	}

	rows, err := sqlc.New(pool).ListRevisionCheckpoints(context.Background(), mustUUID(projectID))
	if err != nil {
		t.Fatalf("ListRevisionCheckpoints: %v", err)
	}
	if !hasCheckpoint(rows, "snippets", "ask_feedback") {
		t.Fatalf("writing coach turn did not record snippets checkpoint; rows=%+v", rows)
	}
}

// TestProposalReviewCoachTurn_RecordsProposalCheckpoint — a coach turn on
// scope=proposal_review must record a "proposal"/"ask_feedback" checkpoint.
func TestProposalReviewCoachTurn_RecordsProposalCheckpoint(t *testing.T) {
	prov := gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: `你的目标句还可以再收窄一点。`},
		{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 10, OutputTokens: 5}},
		{Kind: gateway.EventDone, StopReason: gateway.StopStop},
	})
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: prov, ChatResolver: fakeResolver(), FastChatResolver: fakeResolver(),
	}).Handler()
	cookie := signInSeed(t, pool)
	projectID := seedProjectID

	// Seed a proposal so checkpointProposal has content to snapshot.
	if _, err := sqlc.New(pool).UpsertProjectProposal(context.Background(), sqlc.UpsertProjectProposalParams{
		ProjectID: mustUUID(projectID), Objective: "bounded yes", Reason: "r", Activities: "a", Resources: "s",
	}); err != nil {
		t.Fatalf("seed proposal: %v", err)
	}

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+projectID+"/coach",
		strings.NewReader(`{"scope":"proposal_review","user_input":"看看我的提案够不够站得住"}`)), cookie))
	if rr.Code != http.StatusOK {
		t.Fatalf("coach = %d, want 200; body=%s", rr.Code, rr.Body)
	}

	rows, err := sqlc.New(pool).ListRevisionCheckpoints(context.Background(), mustUUID(projectID))
	if err != nil {
		t.Fatalf("ListRevisionCheckpoints: %v", err)
	}
	if !hasCheckpoint(rows, "proposal", "ask_feedback") {
		t.Fatalf("proposal_review coach turn did not record proposal checkpoint; rows=%+v", rows)
	}
}

// TestReviewEssayStatement_RecordsClaimAndOutlineCheckpoints — the essay
// statement-step draft-annotation review must record claim + outline
// ask_feedback checkpoints.
func TestReviewEssayStatement_RecordsClaimAndOutlineCheckpoints(t *testing.T) {
	prov := gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: `[]`},
		{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 10, OutputTokens: 5}},
		{Kind: gateway.EventDone, StopReason: gateway.StopStop},
	})
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: prov, ChatResolver: fakeResolver(), FastChatResolver: fakeResolver(),
	}).Handler()
	cookie := signInSeed(t, pool)
	projectID := seedProjectID

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+projectID+"/essay-statement/review",
		strings.NewReader(`{}`)), cookie))
	if rr.Code != http.StatusOK {
		t.Fatalf("review essay statement = %d, want 200; body=%s", rr.Code, rr.Body)
	}

	rows, err := sqlc.New(pool).ListRevisionCheckpoints(context.Background(), mustUUID(projectID))
	if err != nil {
		t.Fatalf("ListRevisionCheckpoints: %v", err)
	}
	if !hasCheckpoint(rows, "claim", "ask_feedback") {
		t.Fatalf("review essay statement did not record claim checkpoint; rows=%+v", rows)
	}
	if !hasCheckpoint(rows, "outline", "ask_feedback") {
		t.Fatalf("review essay statement did not record outline checkpoint; rows=%+v", rows)
	}
}

// TestPostReflectProjectCard_RecordsSnippetsAndClaimCheckpoints — completing
// a project card (card_reflect) must record snippets + claim ask_feedback
// checkpoints.
func TestPostReflectProjectCard_RecordsSnippetsAndClaimCheckpoints(t *testing.T) {
	prov := gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: `谢谢分享，继续想想这个方向。`},
		{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 10, OutputTokens: 5}},
		{Kind: gateway.EventDone, StopReason: gateway.StopStop},
	})
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: prov, ChatResolver: fakeResolver(), FastChatResolver: fakeResolver(),
		EvalResolver: fakeEvalResolver(), SpecByID: cardsByID(),
	}).Handler()
	cookie := signInSeed(t, pool)
	projectID := seedProjectID

	body := `{"card_id":"fact-opinion-value","field_values":{"claim":"这是一个陈述"},"event_trace":[],"surface":"writing"}`
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+projectID+"/cards/reflect",
		strings.NewReader(body)), cookie))
	if rr.Code != http.StatusOK {
		t.Fatalf("card reflect = %d, want 200; body=%s", rr.Code, rr.Body)
	}

	rows, err := sqlc.New(pool).ListRevisionCheckpoints(context.Background(), mustUUID(projectID))
	if err != nil {
		t.Fatalf("ListRevisionCheckpoints: %v", err)
	}
	if !hasCheckpoint(rows, "snippets", "ask_feedback") {
		t.Fatalf("card reflect did not record snippets checkpoint; rows=%+v", rows)
	}
	if !hasCheckpoint(rows, "claim", "ask_feedback") {
		t.Fatalf("card reflect did not record claim checkpoint; rows=%+v", rows)
	}
}
