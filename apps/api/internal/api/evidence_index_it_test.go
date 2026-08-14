package api_test

// evidence_index_it_test.go — G5 · BuildEvidenceIndex against a real DB.

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/evalreport"
	"mindimprint/api/internal/store/sqlc"
)

// TestBuildEvidenceIndex_SpansKinds seeds a reference (reference kind) and an
// event (event kind), then asserts the index spans both and that every returned
// id resolves.
func TestBuildEvidenceIndex_SpansKinds(t *testing.T) {
	pool := newAPITestPool(t)
	h := libraryTestHandler(pool)
	cookie := signInSeed(t, pool)
	pid := createProjectForTest(t, h, cookie)
	base := "/api/v1/projects/" + pid
	ctx := context.Background()

	// reference kind
	rec := doJSON(t, h, cookie, "POST", base+"/references", `{"title":"Nature Sustainability"}`)
	if rec.Code != 201 {
		t.Fatalf("create reference = %d: %s", rec.Code, rec.Body)
	}
	var refWrap struct {
		Reference struct {
			ID string `json:"id"`
		} `json:"reference"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &refWrap)
	refID := refWrap.Reference.ID

	// event kind (annotation_opened)
	if rec = doJSON(t, h, cookie, "POST", base+"/annotations/open", `{"annotationId":"a1","doc":"proposal"}`); rec.Code != 204 {
		t.Fatalf("annotation open = %d: %s", rec.Code, rec.Body)
	}
	var eventID string
	_ = pool.QueryRow(ctx,
		`SELECT id::text FROM event WHERE project_id=$1 AND type='annotation_opened' LIMIT 1`, pid).Scan(&eventID)
	if eventID == "" {
		t.Fatal("expected an annotation_opened event id")
	}

	idx, err := BuildEvidenceIndex(ctx, sqlc.New(pool), uuid.MustParse(pid))
	if err != nil {
		t.Fatalf("BuildEvidenceIndex: %v", err)
	}
	if idx.Len() < 2 {
		t.Fatalf("index Len = %d, want >= 2", idx.Len())
	}
	if !idx.Has(refID) {
		t.Errorf("index missing reference id %s", refID)
	}
	if !idx.Has(eventID) {
		t.Errorf("index missing event id %s", eventID)
	}
	// The two kinds are distinct.
	if c, _ := idx.Get(refID); c.Kind != evalreport.KindReference {
		t.Errorf("reference candidate kind = %q, want %q", c.Kind, evalreport.KindReference)
	}
	if c, _ := idx.Get(eventID); c.Kind != evalreport.KindEvent {
		t.Errorf("event candidate kind = %q, want %q", c.Kind, evalreport.KindEvent)
	}
}
