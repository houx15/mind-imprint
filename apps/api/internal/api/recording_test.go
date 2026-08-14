package api_test

// recording_test.go — G2 (annotation-open) + G3 (citation) recording endpoints.

import (
	"context"
	"encoding/json"
	"testing"
)

// G2 · POST /annotations/open appends one annotation_opened event carrying the
// annotation id, and rejects an empty id.
func TestAnnotationOpen_RecordsEvent(t *testing.T) {
	pool := newAPITestPool(t)
	h := libraryTestHandler(pool)
	cookie := signInSeed(t, pool)
	pid := createProjectForTest(t, h, cookie)
	base := "/api/v1/projects/" + pid

	rec := doJSON(t, h, cookie, "POST", base+"/annotations/open", `{"annotationId":"anno-123","doc":"proposal"}`)
	if rec.Code != 204 {
		t.Fatalf("annotation open = %d, want 204: %s", rec.Code, rec.Body)
	}

	var n int
	_ = pool.QueryRow(context.Background(),
		`SELECT count(*) FROM event WHERE project_id=$1 AND type='annotation_opened' AND payload->>'annotation_id'=$2`,
		pid, "anno-123").Scan(&n)
	if n != 1 {
		t.Fatalf("annotation_opened events = %d, want 1", n)
	}

	// A second open of the same annotation accumulates (an event stream, not a
	// flag) — engagement is measured by opens, robust to re-review.
	rec = doJSON(t, h, cookie, "POST", base+"/annotations/open", `{"annotationId":"anno-123","doc":"proposal"}`)
	if rec.Code != 204 {
		t.Fatalf("second annotation open = %d, want 204", rec.Code)
	}
	_ = pool.QueryRow(context.Background(),
		`SELECT count(*) FROM event WHERE project_id=$1 AND type='annotation_opened'`, pid).Scan(&n)
	if n != 2 {
		t.Fatalf("annotation_opened events after two opens = %d, want 2", n)
	}

	// Empty annotationId is a 400.
	rec = doJSON(t, h, cookie, "POST", base+"/annotations/open", `{"annotationId":"","doc":"proposal"}`)
	if rec.Code != 400 {
		t.Fatalf("empty annotationId = %d, want 400: %s", rec.Code, rec.Body)
	}
}

// G3 · POST /citations links a project's reference to an essay section; a
// foreign/unknown reference 404s, a non-uuid 400s.
func TestCitation_RecordsLink(t *testing.T) {
	pool := newAPITestPool(t)
	h := libraryTestHandler(pool)
	cookie := signInSeed(t, pool)
	pid := createProjectForTest(t, h, cookie)
	base := "/api/v1/projects/" + pid

	// Seed a reference on this project.
	rec := doJSON(t, h, cookie, "POST", base+"/references", `{"title":"NASA 报告"}`)
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

	// Happy path → 201, and a row lands with the section.
	rec = doJSON(t, h, cookie, "POST", base+"/citations", `{"referenceId":"`+refID+`","section":"claim:abc-1"}`)
	if rec.Code != 201 {
		t.Fatalf("create citation = %d, want 201: %s", rec.Code, rec.Body)
	}
	var n int
	_ = pool.QueryRow(context.Background(),
		`SELECT count(*) FROM citation WHERE project_id=$1 AND reference_id=$2 AND section='claim:abc-1'`,
		pid, refID).Scan(&n)
	if n != 1 {
		t.Fatalf("citation rows = %d, want 1", n)
	}

	// Unknown reference id (well-formed uuid, not in this project) → 404.
	rec = doJSON(t, h, cookie, "POST", base+"/citations",
		`{"referenceId":"00000000-0000-0000-0000-000000000000","section":"claim:x"}`)
	if rec.Code != 404 {
		t.Fatalf("unknown reference = %d, want 404: %s", rec.Code, rec.Body)
	}

	// Non-uuid reference id → 400.
	rec = doJSON(t, h, cookie, "POST", base+"/citations", `{"referenceId":"not-a-uuid","section":"claim:x"}`)
	if rec.Code != 400 {
		t.Fatalf("non-uuid reference = %d, want 400: %s", rec.Code, rec.Body)
	}

	// Empty section → 400.
	rec = doJSON(t, h, cookie, "POST", base+"/citations", `{"referenceId":"`+refID+`","section":""}`)
	if rec.Code != 400 {
		t.Fatalf("empty section = %d, want 400: %s", rec.Code, rec.Body)
	}
}
