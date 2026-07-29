package api_test

// reading_brief_test.go — S2 brief-in: PUT .../reading-brief persists why-read-
// THIS-source onto the reference row. The templating function itself
// (suggestReadingReason, unexported/pure) is tested in
// reading_brief_internal_test.go (package api).

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

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

// TestPutReadingBrief_PreservesNotes guards against toReferenceDTO(row, nil)
// wiping a reference's already-projected reading notes: seed a material with
// a SUBMITTED (completed) reading card whose anchor carries a finding for it
// (the exact shape notesByMaterial projects, per
// TestGetLibraryProjectsReadingNotes in workspace_library_test.go), link the
// reference to that material, then confirm PUT reading-brief's response still
// carries the note instead of an empty array.
func TestPutReadingBrief_PreservesNotes(t *testing.T) {
	h, cookie, q := planTestHandler(t)
	projectID := uuid.MustParse(seedProjectID)

	matID := ingestMaterialForTest(t, h, cookie, seedProjectID, "NASA 报告", craapMaterialText)
	ref := seedReference(t, q, "NASA 报告")
	if _, err := q.SetReferenceMaterial(context.Background(), sqlc.SetReferenceMaterialParams{
		ID:         ref.ID,
		ProjectID:  projectID,
		MaterialID: pgtype.UUID{Bytes: uuid.MustParse(matID), Valid: true},
	}); err != nil {
		t.Fatalf("link reference to material: %v", err)
	}

	ci, err := q.CreateProjectCardInstance(context.Background(), sqlc.CreateProjectCardInstanceParams{
		ProjectID: pgtype.UUID{Bytes: projectID, Valid: true}, CardID: "craap", Status: "active",
	})
	if err != nil {
		t.Fatalf("create card instance: %v", err)
	}
	anchors := fmt.Sprintf(`[{"id":"a0","material_id":%q,"quote":"根据 NASA 卫星数据","answer":"溯源到 Chen et al. (2019) 才是一手"}]`, matID)
	if _, err := q.SetCardInstanceAnchors(context.Background(), sqlc.SetCardInstanceAnchorsParams{
		ID: ci.ID, ProjectID: pgtype.UUID{Bytes: projectID, Valid: true}, Anchors: []byte(anchors),
	}); err != nil {
		t.Fatalf("set anchors: %v", err)
	}
	if _, err := q.SetCardInstanceStatus(context.Background(), sqlc.SetCardInstanceStatusParams{
		ID: ci.ID, ProjectID: pgtype.UUID{Bytes: projectID, Valid: true}, Status: "completed",
	}); err != nil {
		t.Fatalf("complete card: %v", err)
	}

	body := `{"reading_reason":"验证碳排放反例","reading_focus":"看引用来源","phase_tag":"反例检验"}`
	rr := doJSON(t, h, cookie, http.MethodPut,
		"/api/v1/projects/"+seedProjectID+"/references/"+ref.ID.String()+"/reading-brief",
		body)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var out struct {
		Reference referenceView `json:"reference"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal response: %v — %s", err, rr.Body.String())
	}
	if len(out.Reference.Notes) != 1 {
		t.Fatalf("reading-brief response wiped notes, want 1: %+v", out.Reference)
	}
	note := out.Reference.Notes[0]
	if note.Quote != "根据 NASA 卫星数据" || !strings.Contains(note.Finding, "Chen et al.") {
		t.Fatalf("note projection wrong: %+v", note)
	}
}
