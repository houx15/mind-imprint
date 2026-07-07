package api_test

import (
	"bytes"
	"context"
	"testing"

	"mindimprint/api/internal/store/sqlc"
)

func TestCourseStepRenderCacheRoundTrip(t *testing.T) {
	pool := newAPITestPool(t)
	q := sqlc.New(pool)
	ctx := context.Background()

	courses, _ := q.ListCourses(ctx)
	step, err := q.GetCourseStepByOrdinal(ctx, sqlc.GetCourseStepByOrdinalParams{CourseID: courses[0].ID, Ordinal: 0})
	if err != nil {
		t.Fatalf("get step: %v", err)
	}
	if _, err := q.GetCourseStepRender(ctx, step.ID); err == nil {
		t.Fatal("expected no cache row yet")
	}
	r, err := q.UpsertCourseStepRender(ctx, sqlc.UpsertCourseStepRenderParams{CourseStepID: step.ID, Content: []byte(`{"subtitle":"x"}`), Source: "generated"})
	if err != nil || !bytes.Contains(r.Content, []byte("subtitle")) {
		t.Fatalf("upsert render: %v %s", err, r.Content)
	}
	got, err := q.GetCourseStepRender(ctx, step.ID)
	if err != nil || got.Source != "generated" {
		t.Fatalf("get render: %v %+v", err, got)
	}
}
