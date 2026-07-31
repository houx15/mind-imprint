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

	. "mindimprint/api/internal/api"
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

// TestPutReadingBrief_EmptyPhaseStoresNull is the whole-branch-review CRITICAL
// fix: the default path (a student saves a brief without picking a phase) has
// the frontend send `phase_tag: ""`. Before the fix, putReadingBrief passed a
// non-nil pointer to that empty string straight to UpdateReadingBrief, storing
// "" in the column; toReferenceDTO then serialized `"phaseTag": ""`, which
// fails the Zod PhaseTag enum (.nullable().optional() has no "" member) and
// throws in getLibrary's/postFinalizeReading's Reference.parse. Guards that
// putReadingBrief normalizes an empty phase_tag (and reading_reason/focus) to
// NULL — never "" — while still persisting a genuinely non-empty reason.
func TestPutReadingBrief_EmptyPhaseStoresNull(t *testing.T) {
	h, cookie, q := planTestHandler(t)
	projectID := uuid.MustParse(seedProjectID)
	ref := seedReference(t, q, "未分阶段源")

	body := `{"reading_reason":"验证碳排放反例","reading_focus":"","phase_tag":""}`
	rr := doJSON(t, h, cookie, http.MethodPut,
		"/api/v1/projects/"+seedProjectID+"/references/"+ref.ID.String()+"/reading-brief",
		body)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Check the persisted row directly first — this is the shape that flows
	// into toReferenceDTO/getLibrary.
	row, err := q.GetReferenceForProject(context.Background(), sqlc.GetReferenceForProjectParams{
		ID: ref.ID, ProjectID: projectID,
	})
	if err != nil {
		t.Fatalf("GetReferenceForProject: %v", err)
	}
	if row.PhaseTag != nil {
		t.Fatalf("phase_tag should be stored as NULL for an empty phase, got %q", *row.PhaseTag)
	}
	if row.ReadingFocus != nil {
		t.Fatalf("reading_focus should be stored as NULL for an empty focus, got %q", *row.ReadingFocus)
	}
	if row.ReadingReason == nil || *row.ReadingReason != "验证碳排放反例" {
		t.Fatalf("reading_reason should persist the non-empty value, got %v", row.ReadingReason)
	}

	// The PUT response's own echoed reference DTO must reflect the same nil,
	// never "" — the shape that would fail Zod's PhaseTag enum on the TS side
	// (getLibrary/postFinalizeReading both re-parse this exact projection).
	var out struct {
		Reference referenceView `json:"reference"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal response: %v — %s", err, rr.Body.String())
	}
	if out.Reference.PhaseTag != nil {
		t.Fatalf("response reference.phaseTag should be nil, got %v", *out.Reference.PhaseTag)
	}

	// Round-trip through GET /library too — this is the exact path
	// getLibrary's z.array(Reference).parse would throw on before the fix.
	rec := doJSON(t, h, cookie, http.MethodGet, "/api/v1/projects/"+seedProjectID+"/library", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("get library = %d: %s", rec.Code, rec.Body)
	}
	var lib struct {
		References []referenceView `json:"references"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &lib); err != nil {
		t.Fatalf("decode library: %v — %s", err, rec.Body)
	}
	for _, r := range lib.References {
		if r.ID == ref.ID.String() && r.PhaseTag != nil {
			t.Fatalf("library reference.phaseTag should be nil, got %v", *r.PhaseTag)
		}
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

// TestReadingBriefFor_MatchesByMaterialAndLoadsProposal is the coverage-gap
// fix: readingBriefFor (reading_brief.go) is the one new DB-facing function
// in Task 3 and previously had no test exercising it against real Postgres
// rows (only buildReadingRouteUserPrompt, with a hand-built agent.ReadingBrief
// struct, was tested). Drives it directly via the ReadingBriefForTest seam
// (export_test.go) against a persisted reference brief + proposal, and guards
// the match-by-material_id logic: a material with no linked reference must
// not pick up another reference's brief just because it shares the project.
func TestReadingBriefFor_MatchesByMaterialAndLoadsProposal(t *testing.T) {
	pool := newAPITestPool(t)
	a := New(DepsForTest(pool))
	h := a.Handler()
	cookie := signInSeed(t, pool)
	q := sqlc.New(pool)
	ctx := context.Background()
	projectID := uuid.MustParse(seedProjectID)

	// Seed a reference bound to a real material, carrying a persisted brief.
	matID := ingestMaterialForTest(t, h, cookie, seedProjectID, "NASA 报告", craapMaterialText)
	materialID := uuid.MustParse(matID)
	ref := seedReference(t, q, "NASA 报告")
	if _, err := q.SetReferenceMaterial(ctx, sqlc.SetReferenceMaterialParams{
		ID: ref.ID, ProjectID: projectID,
		MaterialID: pgtype.UUID{Bytes: materialID, Valid: true},
	}); err != nil {
		t.Fatalf("link reference to material: %v", err)
	}
	reason, focus, phase := "验证碳排放反例", "看引用来源是否可信", "反例检验"
	if _, err := q.UpdateReadingBrief(ctx, sqlc.UpdateReadingBriefParams{
		ID: ref.ID, ProjectID: projectID,
		ReadingReason: &reason, ReadingFocus: &focus, PhaseTag: &phase,
	}); err != nil {
		t.Fatalf("persist brief: %v", err)
	}
	if _, err := q.UpsertProjectProposal(ctx, sqlc.UpsertProjectProposalParams{
		ProjectID: projectID, Objective: "研究中国是否让地球更可持续",
		Reason: "关心气候变化", Activities: "读 NASA/Nature，写论证", Resources: "Zotero + 学校图书馆",
	}); err != nil {
		t.Fatalf("persist proposal: %v", err)
	}

	// (a) happy path: the persisted brief + proposal snapshot load.
	got := a.ReadingBriefForTest(ctx, projectID, materialID)
	if got.Reason != reason || got.Focus != focus || got.PhaseTag != phase {
		t.Fatalf("readingBriefFor did not load the persisted brief, got %+v", got)
	}
	if got.ProposalSnap != "研究中国是否让地球更可持续" {
		t.Fatalf("readingBriefFor did not load the proposal snapshot, got %+v", got)
	}

	// (b) guard: a DIFFERENT material with no reference pointing at it must
	// not pick up the seeded reference's brief just because it's the same
	// project — this is the "wrong match" bug class the gap review flagged.
	otherMatID := uuid.MustParse(ingestMaterialForTest(t, h, cookie, seedProjectID, "Nature 报告", craapMaterialText))
	unmatched := a.ReadingBriefForTest(ctx, projectID, otherMatID)
	if unmatched.Reason != "" || unmatched.Focus != "" || unmatched.PhaseTag != "" {
		t.Fatalf("readingBriefFor matched an unrelated reference's brief for an unlinked material: %+v", unmatched)
	}
}

// #22: a linked reference with NO persisted reading_reason yet still carries a
// motivation into reading — seeded from the proposal objective — so the
// read-turn agent reads WITH a purpose instead of a blank brief. (An unlinked
// material stays blank; that guard is covered above.)
func TestReadingBriefFor_SeedsMotivationFromProposal(t *testing.T) {
	pool := newAPITestPool(t)
	a := New(DepsForTest(pool))
	h := a.Handler()
	cookie := signInSeed(t, pool)
	q := sqlc.New(pool)
	ctx := context.Background()
	projectID := uuid.MustParse(seedProjectID)

	matID := ingestMaterialForTest(t, h, cookie, seedProjectID, "NASA 报告", craapMaterialText)
	materialID := uuid.MustParse(matID)
	ref := seedReference(t, q, "NASA 报告")
	if _, err := q.SetReferenceMaterial(ctx, sqlc.SetReferenceMaterialParams{
		ID: ref.ID, ProjectID: projectID,
		MaterialID: pgtype.UUID{Bytes: materialID, Valid: true},
	}); err != nil {
		t.Fatalf("link reference to material: %v", err)
	}
	// Deliberately NO UpdateReadingBrief — the student hasn't written her reason.
	if _, err := q.UpsertProjectProposal(ctx, sqlc.UpsertProjectProposalParams{
		ProjectID: projectID, Objective: "研究中国是否让地球更可持续",
		Reason: "关心气候变化", Activities: "读 NASA/Nature", Resources: "Zotero",
	}); err != nil {
		t.Fatalf("persist proposal: %v", err)
	}

	got := a.ReadingBriefForTest(ctx, projectID, materialID)
	if strings.TrimSpace(got.Reason) == "" {
		t.Fatalf("readingBriefFor should seed a motivation from the proposal when reading_reason is unset, got empty")
	}
	if !strings.Contains(got.Reason, "研究中国是否让地球更可持续") {
		t.Fatalf("seeded motivation should carry the proposal objective, got %q", got.Reason)
	}
}
