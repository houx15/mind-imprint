package api_test

import (
	"context"
	"testing"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/store/sqlc"
)

func TestCourseSeedAndProgressRoundTrip(t *testing.T) {
	pool := newAPITestPool(t)
	q := sqlc.New(pool)
	ctx := context.Background()

	courses, err := q.ListCourses(ctx)
	if err != nil || len(courses) == 0 {
		t.Fatalf("list courses: %v len=%d", err, len(courses))
	}
	co := courses[0]
	if co.Title == "" || co.StepCount == 0 {
		t.Fatalf("seed course missing title/steps: %+v", co)
	}

	steps, err := q.ListCourseSteps(ctx, co.ID)
	if err != nil || len(steps) == 0 {
		t.Fatalf("list steps: %v len=%d", err, len(steps))
	}

	prog, err := q.UpsertCourseProgress(ctx, sqlc.UpsertCourseProgressParams{
		UserID: mustUUID(SeedUserID.String()), CourseID: co.ID, CurrentOrdinal: 1, CompletedOrdinals: []int32{0},
	})
	if err != nil || prog.CurrentOrdinal != 1 {
		t.Fatalf("upsert progress: %v %+v", err, prog)
	}
	// upsert again → update path
	prog2, err := q.UpsertCourseProgress(ctx, sqlc.UpsertCourseProgressParams{
		UserID: mustUUID(SeedUserID.String()), CourseID: co.ID, CurrentOrdinal: 2, CompletedOrdinals: []int32{0, 1},
	})
	if err != nil || prog2.CurrentOrdinal != 2 || len(prog2.CompletedOrdinals) != 2 {
		t.Fatalf("upsert update: %v %+v", err, prog2)
	}
}
