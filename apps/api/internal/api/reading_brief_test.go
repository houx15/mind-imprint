package api_test

// reading_brief_test.go — S2 brief-in: PUT .../reading-brief persists why-read-
// THIS-source onto the reference row. The templating function itself
// (suggestReadingReason, unexported/pure) is tested in
// reading_brief_internal_test.go (package api).

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"

	"mindimprint/api/internal/store/sqlc"
)

// seedReference inserts a bare reference row under the seeded demo project
// and returns it.
func seedReference(t *testing.T, q *sqlc.Queries, title string) sqlc.Reference {
	t.Helper()
	ref, err := q.CreateReference(context.Background(), sqlc.CreateReferenceParams{
		ProjectID:   uuid.MustParse(seedProjectID),
		Title:       title,
		Tags:        []byte("[]"),
		SearchHints: []byte("[]"),
	})
	if err != nil {
		t.Fatalf("seedReference: %v", err)
	}
	return ref
}

func TestPutReadingBrief_Persists(t *testing.T) {
	h, cookie, q := planTestHandler(t)
	ref := seedReference(t, q, "NASA 报告")

	body := `{"reading_reason":"验证碳排放反例","reading_focus":"看引用来源","phase_tag":"反例检验"}`
	rr := doJSON(t, h, cookie, http.MethodPut,
		"/api/v1/projects/"+seedProjectID+"/references/"+ref.ID.String()+"/reading-brief",
		body)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "反例检验") {
		t.Fatalf("response should echo the brief: %s", rr.Body.String())
	}
}
