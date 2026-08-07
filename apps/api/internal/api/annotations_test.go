package api_test

// annotations_test.go — Task 1 (annotation entity): GET
// /projects/{id}/annotations projects the project's persisted review_item
// interventions (written by orderReview, writing.go) into addressable
// {id, criterion, band, text} rows. No new persistence — read-only over
// intervention.body (agent.ReviewItem JSON).

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/store/sqlc"
)

// TestAnnotationsEndpoint_ListsPersistedReviewItems seeds a review_item
// intervention the same way TestOrderReview_PersistsWorkOrderAndIsIdempotent
// does (commit a snapshot, then order a review against a stub provider), then
// asserts GET /annotations surfaces it with a non-empty actionable text.
func TestAnnotationsEndpoint_ListsPersistedReviewItems(t *testing.T) {
	pool := newAPITestPool(t)
	reply := `[{"criterion_code":"表E","band":"5–6 段","evidence":"第2段接住反方","missing":"跳步没补","fix":"补上定义"}]`
	h := New(Deps{
		Queries:      sqlc.New(pool),
		Pool:         pool,
		Provider:     reviewStubProvider(reply),
		ChatResolver: fakeResolver(),
	}).Handler()
	cookie := signInSeed(t, pool) // Phoebe, owns materialsTestProjectID
	projectID := materialsTestProjectID

	// Commit a snapshot first — an in-band draft, same shape as writing_test.go.
	content := strings.Repeat("字", 1600)
	rec := httptest.NewRecorder()
	req := withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+projectID+"/snapshots",
		strings.NewReader(`{"content":`+strconv.Quote(content)+`}`)), cookie)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("commit snapshot = %d, want 201; body=%s", rec.Code, rec.Body)
	}
	var snap struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &snap); err != nil {
		t.Fatalf("decode snapshot: %v", err)
	}

	// Order the review — persists one review_item intervention.
	rec2 := httptest.NewRecorder()
	req2 := withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+projectID+"/snapshots/"+snap.ID+"/review",
		strings.NewReader("")), cookie)
	h.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("order review = %d, want 200; body=%s", rec2.Code, rec2.Body)
	}

	// GET /annotations surfaces it.
	rec3 := httptest.NewRecorder()
	req3 := withCookie(httptest.NewRequest("GET", "/api/v1/projects/"+projectID+"/annotations", nil), cookie)
	h.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusOK {
		t.Fatalf("GET annotations = %d, want 200; body=%s", rec3.Code, rec3.Body)
	}
	var out struct {
		Annotations []struct {
			ID        string `json:"id"`
			Criterion string `json:"criterion"`
			Band      string `json:"band"`
			Text      string `json:"text"`
		} `json:"annotations"`
	}
	if err := json.Unmarshal(rec3.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode annotations: %v — %s", err, rec3.Body.String())
	}
	if len(out.Annotations) != 1 {
		t.Fatalf("annotations = %d, want 1; body=%s", len(out.Annotations), rec3.Body.String())
	}
	a := out.Annotations[0]
	if a.ID == "" {
		t.Errorf("annotation id is empty")
	}
	if a.Criterion == "" {
		t.Errorf("annotation criterion is empty")
	}
	if a.Band == "" {
		t.Errorf("annotation band is empty")
	}
	if strings.TrimSpace(a.Text) == "" {
		t.Errorf("annotation text is empty, want missing+fix combined")
	}
	if !strings.Contains(a.Text, "跳步没补") || !strings.Contains(a.Text, "补上定义") {
		t.Errorf("annotation text = %q, want it to combine missing (跳步没补) and fix (补上定义)", a.Text)
	}
}

// TestAnnotationsEndpoint_EmptyWhenNoReview asserts a project with no
// review_item interventions returns an empty array (not null/omitted).
func TestAnnotationsEndpoint_EmptyWhenNoReview(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool}).Handler()
	cookie := signInSeed(t, pool)

	rec := httptest.NewRecorder()
	req := withCookie(httptest.NewRequest("GET", "/api/v1/projects/"+materialsTestProjectID+"/annotations", nil), cookie)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET annotations = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	if strings.TrimSpace(rec.Body.String()) != `{"annotations":[]}` {
		t.Fatalf("body = %s, want empty annotations array", rec.Body.String())
	}
}
