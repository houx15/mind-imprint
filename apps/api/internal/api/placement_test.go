package api_test

// placement_test.go — S3 rabbit-hole Task 4: POST .../exploration/suggest-placement.
// Mirrors exploration_test.go's harness/seed conventions. Only the no-questions
// (no-spend) and IDOR branches are pinned here — neither touches a.d.Provider,
// which is nil in these harnesses; the happy path (with a stub provider) is
// covered by Task 3's agent-level test (agent.SuggestBestQuestion).

import (
	"encoding/json"
	"testing"
)

// TestSuggestPlacement_NoQuestions_ReturnsNullNoSpend — a project with a
// reference but no open question leads has nothing to place it under, so the
// handler must degrade to { leadId: null, reason: "" } WITHOUT ever touching
// a.d.Provider (nil in this harness — a call would panic/error otherwise).
func TestSuggestPlacement_NoQuestions_ReturnsNullNoSpend(t *testing.T) {
	pool := newAPITestPool(t)
	h := libraryTestHandler(pool)
	cookie := signInSeed(t, pool)
	pid := createProjectForTest(t, h, cookie)
	base := "/api/v1/projects/" + pid

	rec := doJSON(t, h, cookie, "POST", base+"/references", `{"title":"孤零零的来源"}`)
	if rec.Code != 201 {
		t.Fatalf("create reference = %d, want 201: %s", rec.Code, rec.Body)
	}
	var refWrap struct {
		Reference struct {
			ID string `json:"id"`
		} `json:"reference"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &refWrap); err != nil {
		t.Fatalf("decode reference: %v — %s", err, rec.Body)
	}
	refID := refWrap.Reference.ID

	rec = doJSON(t, h, cookie, "POST", base+"/exploration/suggest-placement", `{"referenceId":"`+refID+`"}`)
	if rec.Code != 200 {
		t.Fatalf("suggest-placement = %d, want 200: %s", rec.Code, rec.Body)
	}
	// No questions yet → leadId null, empty reason, and (implicitly) no model call.
	var body struct {
		LeadID *string `json:"leadId"`
		Reason string  `json:"reason"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode suggest-placement response: %v — %s", err, rec.Body)
	}
	if body.LeadID != nil {
		t.Fatalf("leadId = %v, want null when there are no questions", *body.LeadID)
	}
	if body.Reason != "" {
		t.Fatalf("reason = %q, want empty when there are no questions", body.Reason)
	}
}

// TestSuggestPlacement_ForeignReference_400 — a referenceId that's real but
// lives in a different project must be rejected (IDOR guard), mirroring the
// convention exploration_test.go already follows for parentLeadId/sourceReferenceId.
func TestSuggestPlacement_ForeignReference_400(t *testing.T) {
	pool := newAPITestPool(t)
	h := libraryTestHandler(pool)
	cookie := signInSeed(t, pool)
	pid := createProjectForTest(t, h, cookie)
	other := createProjectForTest(t, h, cookie)

	rec := doJSON(t, h, cookie, "POST", "/api/v1/projects/"+other+"/references", `{"title":"别处"}`)
	if rec.Code != 201 {
		t.Fatalf("create foreign reference = %d, want 201: %s", rec.Code, rec.Body)
	}
	var refWrap struct {
		Reference struct {
			ID string `json:"id"`
		} `json:"reference"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &refWrap); err != nil {
		t.Fatalf("decode foreign reference: %v — %s", err, rec.Body)
	}
	refID := refWrap.Reference.ID

	rec = doJSON(t, h, cookie, "POST", "/api/v1/projects/"+pid+"/exploration/suggest-placement", `{"referenceId":"`+refID+`"}`)
	if rec.Code != 400 {
		t.Fatalf("suggest-placement with foreign referenceId = %d, want 400: %s", rec.Code, rec.Body)
	}
}
