package store_test

// seed_courses_test.go — Task 12's real-DB pass for the course example seed:
// after SeedCourses runs against a fresh (migrated, empty) database,
// ListCourses must return both a-mid and b-mid with the card_ids/step_count/
// title the brief specifies. Mirrors internal/agent/coursestore_test.go's
// pattern (real embedded JSON, real store, testcontainers), but exercises
// the seed ENTRY POINT (store.SeedCourses) rather than UpsertCourse directly.

import (
	"context"
	"testing"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/store"
	"mindimprint/api/internal/store/sqlc"
)

func TestSeedCourses(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newStoreTestPool(t)

	n, err := store.SeedCourses(ctx, pool)
	if err != nil {
		t.Fatalf("SeedCourses: %v", err)
	}
	if n != 2 {
		t.Fatalf("SeedCourses returned %d, want 2", n)
	}

	q := sqlc.New(pool)
	agentStore := agent.NewSqlcAgentStore(q, pool)
	courses, err := agentStore.ListCourses(ctx)
	if err != nil {
		t.Fatalf("ListCourses: %v", err)
	}
	if len(courses) != 2 {
		t.Fatalf("ListCourses len = %d, want 2", len(courses))
	}

	bySlug := map[string]agent.CourseSummaryRow{}
	for _, c := range courses {
		bySlug[c.Slug] = c
	}

	aMid, ok := bySlug["a-mid"]
	if !ok {
		t.Fatalf("a-mid not seeded; got slugs %v", bySlug)
	}
	if aMid.Branch != "A" {
		t.Fatalf("a-mid.Branch = %q, want \"A\"", aMid.Branch)
	}
	if aMid.Title == "" {
		t.Fatalf("a-mid.Title is empty")
	}
	if len(aMid.CardIDs) != 1 || aMid.CardIDs[0] != "craap" {
		t.Fatalf("a-mid.CardIDs = %v, want [craap]", aMid.CardIDs)
	}
	if aMid.StepCount <= 0 {
		t.Fatalf("a-mid.StepCount = %d, want > 0", aMid.StepCount)
	}

	bMid, ok := bySlug["b-mid"]
	if !ok {
		t.Fatalf("b-mid not seeded; got slugs %v", bySlug)
	}
	if bMid.Branch != "B" {
		t.Fatalf("b-mid.Branch = %q, want \"B\"", bMid.Branch)
	}
	if bMid.Title == "" {
		t.Fatalf("b-mid.Title is empty")
	}
	wantCards := []string{"sift", "craap", "argument-map", "concession", "metacognition"}
	if len(bMid.CardIDs) != len(wantCards) {
		t.Fatalf("b-mid.CardIDs = %v, want %v", bMid.CardIDs, wantCards)
	}
	for i, id := range wantCards {
		if bMid.CardIDs[i] != id {
			t.Fatalf("b-mid.CardIDs = %v, want %v", bMid.CardIDs, wantCards)
		}
	}
	if bMid.StepCount <= 0 {
		t.Fatalf("b-mid.StepCount = %d, want > 0", bMid.StepCount)
	}

	// Idempotent: calling SeedCourses again does not error and does not
	// duplicate rows (upsert-by-slug).
	n2, err := store.SeedCourses(ctx, pool)
	if err != nil {
		t.Fatalf("SeedCourses (second call): %v", err)
	}
	if n2 != 2 {
		t.Fatalf("SeedCourses (second call) returned %d, want 2", n2)
	}
	coursesAgain, err := agentStore.ListCourses(ctx)
	if err != nil {
		t.Fatalf("ListCourses (second call): %v", err)
	}
	if len(coursesAgain) != 2 {
		t.Fatalf("ListCourses (second call) len = %d, want 2 (upsert should not duplicate)", len(coursesAgain))
	}
}
