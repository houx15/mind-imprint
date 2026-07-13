package store

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/store/sqlc"
)

// jsonEqual compares two jsonb byte slices by decoded value, not raw bytes —
// Postgres re-serializes jsonb (key order, whitespace), so a byte-for-byte
// comparison against the literal we sent would be brittle.
func jsonEqual(t *testing.T, got, want []byte) bool {
	t.Helper()
	var g, w any
	if err := json.Unmarshal(got, &g); err != nil {
		t.Fatalf("unmarshal got: %v (%s)", err, got)
	}
	if err := json.Unmarshal(want, &w); err != nil {
		t.Fatalf("unmarshal want: %v (%s)", err, want)
	}
	return reflect.DeepEqual(g, w)
}

// TestRefactor2CardsSqlcLifecycle exercises the Task 4 project-scoped
// card_instance queries: seed a project (+ a task, since a legacy row can
// still carry a task_id even though the column went nullable in migration
// 0020), CreateProjectCardInstance (status proposed) -> SetCardInstanceAnchors
// -> SetCardInstanceStatus (active) -> SetCardInstanceFramework ->
// GetCardInstance reflects each step -> ListCardInstancesByProject includes
// it.
func TestRefactor2CardsSqlcLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newTestPool(t)
	q := sqlc.New(pool)
	seededStudentID := refactor2SeededStudentID

	task, err := q.CreateTask(ctx, sqlc.CreateTaskParams{
		UserID: seededStudentID,
		Title:  "refactor2-cards-sqlc",
	})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	project, err := q.CreateProject(ctx, sqlc.CreateProjectParams{
		UserID:        seededStudentID,
		Qualification: "EE",
		Title:         "中国是否让地球变得更可持续？",
		BoardCfgVer:   1,
	})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	projectID := pgtype.UUID{Bytes: project.ID, Valid: true}

	contractRef := "craap"
	created, err := q.CreateProjectCardInstance(ctx, sqlc.CreateProjectCardInstanceParams{
		TaskID:      pgtype.UUID{Bytes: task.ID, Valid: true},
		ProjectID:   projectID,
		CardID:      "craap",
		ContractRef: &contractRef,
		Status:      "proposed",
	})
	if err != nil {
		t.Fatalf("CreateProjectCardInstance: %v", err)
	}
	if created.Status != "proposed" {
		t.Fatalf("Status = %q, want proposed", created.Status)
	}
	if created.ContractRef == nil || *created.ContractRef != "craap" {
		t.Fatalf("ContractRef = %v, want craap", created.ContractRef)
	}
	if len(created.FrameworkFill) == 0 || string(created.FrameworkFill) != "{}" {
		t.Fatalf("FrameworkFill = %s, want {} (default)", created.FrameworkFill)
	}

	anchorsJSON := []byte(`[{"id":"a0","material_id":"` + project.ID.String() + `","dimension":"authority","author":"ai","answer":"NASA地球观测团队发布"}]`)
	withAnchors, err := q.SetCardInstanceAnchors(ctx, sqlc.SetCardInstanceAnchorsParams{
		ID:        created.ID,
		ProjectID: projectID,
		Anchors:   anchorsJSON,
	})
	if err != nil {
		t.Fatalf("SetCardInstanceAnchors: %v", err)
	}
	if !jsonEqual(t, withAnchors.Anchors, anchorsJSON) {
		t.Fatalf("Anchors = %s, want %s", withAnchors.Anchors, anchorsJSON)
	}

	active, err := q.SetCardInstanceStatus(ctx, sqlc.SetCardInstanceStatusParams{
		ID:        created.ID,
		ProjectID: projectID,
		Status:    "active",
	})
	if err != nil {
		t.Fatalf("SetCardInstanceStatus: %v", err)
	}
	if active.Status != "active" {
		t.Fatalf("Status = %q, want active", active.Status)
	}

	frameworkJSON := []byte(`{"strategy":"reveal_framework_after_completion","framework":"craap"}`)
	withFramework, err := q.SetCardInstanceFramework(ctx, sqlc.SetCardInstanceFrameworkParams{
		ID:            created.ID,
		ProjectID:     projectID,
		FrameworkFill: frameworkJSON,
	})
	if err != nil {
		t.Fatalf("SetCardInstanceFramework: %v", err)
	}
	if !jsonEqual(t, withFramework.FrameworkFill, frameworkJSON) {
		t.Fatalf("FrameworkFill = %s, want %s", withFramework.FrameworkFill, frameworkJSON)
	}

	got, err := q.GetCardInstance(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetCardInstance: %v", err)
	}
	if got.Status != "active" {
		t.Fatalf("GetCardInstance Status = %q, want active", got.Status)
	}
	if !jsonEqual(t, got.Anchors, anchorsJSON) {
		t.Fatalf("GetCardInstance Anchors = %s, want %s", got.Anchors, anchorsJSON)
	}
	if !jsonEqual(t, got.FrameworkFill, frameworkJSON) {
		t.Fatalf("GetCardInstance FrameworkFill = %s, want %s", got.FrameworkFill, frameworkJSON)
	}

	list, err := q.ListCardInstancesByProject(ctx, projectID)
	if err != nil {
		t.Fatalf("ListCardInstancesByProject: %v", err)
	}
	if len(list) != 1 || list[0].ID != created.ID {
		t.Fatalf("ListCardInstancesByProject = %d rows, want 1 matching", len(list))
	}
}

// TestRefactor2CardsDispositionRoundTrip exercises InsertDisposition: a
// reject with a >=15-char reason round-trips against a real intervention row.
func TestRefactor2CardsDispositionRoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newTestPool(t)
	q := sqlc.New(pool)
	seededStudentID := refactor2SeededStudentID

	project, err := q.CreateProject(ctx, sqlc.CreateProjectParams{
		UserID:        seededStudentID,
		Qualification: "EE",
		Title:         "中国是否让地球变得更可持续？",
		BoardCfgVer:   1,
	})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	criterion := "D5"
	ivn, err := q.InsertIntervention(ctx, sqlc.InsertInterventionParams{
		ProjectID: project.ID,
		Type:      "question",
		Anchor:    []byte(`{"kind":"graph_node","id":"` + project.ID.String() + `"}`),
		Criterion: &criterion,
		Body:      "这条主张现在还没有素材支撑——它的证据是什么？",
	})
	if err != nil {
		t.Fatalf("InsertIntervention: %v", err)
	}

	reason := "这条追问和我原本的方向不一致，我想先按自己的思路推进"
	if len(reason) < 15 {
		t.Fatalf("test fixture reason too short: %d bytes", len(reason))
	}
	disp, err := q.InsertDisposition(ctx, sqlc.InsertDispositionParams{
		InterventionID: ivn.ID,
		Action:         "reject",
		Reason:         reason,
	})
	if err != nil {
		t.Fatalf("InsertDisposition: %v", err)
	}
	if disp.InterventionID != ivn.ID {
		t.Fatalf("InterventionID = %s, want %s", disp.InterventionID, ivn.ID)
	}
	if disp.Action != "reject" {
		t.Fatalf("Action = %q, want reject", disp.Action)
	}
	if disp.Reason != reason {
		t.Fatalf("Reason = %q, want %q", disp.Reason, reason)
	}
}
