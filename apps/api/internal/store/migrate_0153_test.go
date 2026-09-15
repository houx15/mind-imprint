package store

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

// TestMigration0153WritingVersions: 0153 adds writing_version, writing.revising_at
// and the three return columns on lite_assignment_recipient, and backfills
// version 1 for every finished writing from its current draft. The expected
// word counts are agent.CountWords on the same strings; the agent package
// pins them in wordcount_migration_fixture_test.go.
func TestMigration0153WritingVersions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newTestPool(t)

	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()
	goose.SetBaseFS(migrationFS)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("dialect: %v", err)
	}
	if err := goose.DownToContext(ctx, db, "migrations", 152); err != nil {
		t.Fatalf("goose down to v152: %v", err)
	}

	seedWriting := func(status, body string, finishedAt *time.Time) string {
		t.Helper()
		var atomID string
		if err := pool.QueryRow(ctx, `INSERT INTO atom (kind, user_id) VALUES ('writing', $1) RETURNING id::text`,
			refactor2SeededStudentID).Scan(&atomID); err != nil {
			t.Fatalf("seed atom: %v", err)
		}
		if _, err := pool.Exec(ctx, `INSERT INTO writing (atom_id, title, lang, status, finished_at) VALUES ($1, '雨', 'zh', $2, $3)`,
			atomID, status, finishedAt); err != nil {
			t.Fatalf("seed writing: %v", err)
		}
		if _, err := pool.Exec(ctx, `INSERT INTO writing_draft (atom_id, body) VALUES ($1, $2)`, atomID, body); err != nil {
			t.Fatalf("seed draft: %v", err)
		}
		return atomID
	}
	finishedAt := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	mixed := seedWriting("finished", "我读了 NASA 的报告，数据是 2024 年的。", &finishedAt)
	english := seedWriting("finished", "It rained all day.\n\n雨停了。", &finishedAt)
	active := seedWriting("active", "还没写完。", nil)

	if err := goose.UpToContext(ctx, db, "migrations", 153); err != nil {
		t.Fatalf("apply 0153: %v", err)
	}

	type version struct {
		number    int
		title     string
		body      string
		words     int
		submitted time.Time
	}
	read := func(atomID string) []version {
		t.Helper()
		rows, err := pool.Query(ctx, `SELECT number, title, body, word_count, submitted_at FROM writing_version WHERE atom_id = $1 ORDER BY number`, atomID)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var out []version
		for rows.Next() {
			var v version
			if err := rows.Scan(&v.number, &v.title, &v.body, &v.words, &v.submitted); err != nil {
				t.Fatal(err)
			}
			out = append(out, v)
		}
		return out
	}
	if got := read(mixed); len(got) != 1 || got[0].number != 1 || got[0].title != "雨" ||
		got[0].body != "我读了 NASA 的报告，数据是 2024 年的。" || got[0].words != 15 || !got[0].submitted.Equal(finishedAt) {
		t.Fatalf("mixed backfill = %+v", got)
	}
	if got := read(english); len(got) != 1 || got[0].words != 8 {
		t.Fatalf("english backfill = %+v", got)
	}
	if got := read(active); len(got) != 0 {
		t.Fatalf("active writing must get no version, got %+v", got)
	}

	if _, err := pool.Exec(ctx, `INSERT INTO writing_version (atom_id, number, title, body, word_count) VALUES ($1, 1, 'x', 'x', 1)`, mixed); err == nil {
		t.Fatal("UNIQUE(atom_id, number) must refuse a second version 1")
	}
	var revising *time.Time
	if err := pool.QueryRow(ctx, `SELECT revising_at FROM writing WHERE atom_id = $1`, mixed).Scan(&revising); err != nil || revising != nil {
		t.Fatalf("revising_at = %v err=%v, want NULL", revising, err)
	}

	if err := goose.DownToContext(ctx, db, "migrations", 152); err != nil {
		t.Fatalf("goose down to v152 again: %v", err)
	}
	var exists bool
	if err := pool.QueryRow(ctx, `SELECT to_regclass('public.writing_version') IS NOT NULL`).Scan(&exists); err != nil || exists {
		t.Fatalf("writing_version after Down exists=%v err=%v", exists, err)
	}
	if err := goose.UpContext(ctx, db, "migrations"); err != nil {
		t.Fatalf("goose up to head: %v", err)
	}
}
