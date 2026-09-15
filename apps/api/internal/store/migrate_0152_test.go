package store

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

// TestMigration0152ParentReportExportOnly: 0152 drops the publish columns of
// lite_parent_report and adds hidden (default '{}'); its Down puts the four
// columns back as 0151 defined them, CHECK and UNIQUE included. newTestPool
// migrates to head, so this goes down to 151, seeds a published row, applies
// 0152, goes down again and back up to head.
func TestMigration0152ParentReportExportOnly(t *testing.T) {
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

	columns := func() map[string]bool {
		t.Helper()
		rows, err := pool.Query(ctx, `SELECT column_name FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name = 'lite_parent_report'`)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		out := map[string]bool{}
		for rows.Next() {
			var c string
			if err := rows.Scan(&c); err != nil {
				t.Fatal(err)
			}
			out[c] = true
		}
		return out
	}
	publish := []string{"status", "share_token", "published_at", "student_seen_at"}
	wantPre0152 := func(when string) {
		t.Helper()
		cols := columns()
		for _, c := range publish {
			if !cols[c] {
				t.Fatalf("%s: column %s missing", when, c)
			}
		}
		if cols["hidden"] {
			t.Fatalf("%s: hidden must not exist", when)
		}
	}
	want0152 := func(when string) {
		t.Helper()
		cols := columns()
		for _, c := range publish {
			if cols[c] {
				t.Fatalf("%s: column %s must be dropped", when, c)
			}
		}
		if !cols["hidden"] {
			t.Fatalf("%s: hidden missing", when)
		}
	}
	want0152("head")

	if err := goose.DownToContext(ctx, db, "migrations", 151); err != nil {
		t.Fatalf("goose down to v151 (reversing 0152): %v", err)
	}
	wantPre0152("down to 151")

	var classID string
	if err := pool.QueryRow(ctx, `SELECT class_id::text FROM enrollments WHERE user_id = $1 LIMIT 1`, refactor2SeededStudentID).Scan(&classID); err != nil {
		t.Fatalf("seeded class: %v", err)
	}
	var reportID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO lite_parent_report (user_id, class_id, created_by, range_start, range_end, facts, status, share_token, published_at)
		VALUES ($1, $2, $1, '2026-08-01', '2026-08-28', '{}'::jsonb, 'published', 'tok', now())
		RETURNING id::text`, refactor2SeededStudentID, classID).Scan(&reportID); err != nil {
		t.Fatalf("seed published report: %v", err)
	}

	if err := goose.UpToContext(ctx, db, "migrations", 152); err != nil {
		t.Fatalf("apply 0152: %v", err)
	}
	want0152("up to 152")
	var hidden string
	if err := pool.QueryRow(ctx, `SELECT hidden::text FROM lite_parent_report WHERE id = $1`, reportID).Scan(&hidden); err != nil {
		t.Fatalf("read hidden: %v", err)
	}
	if hidden != "{}" {
		t.Fatalf("hidden on an existing row = %s, want {}", hidden)
	}

	if err := goose.DownToContext(ctx, db, "migrations", 151); err != nil {
		t.Fatalf("goose down to v151 again: %v", err)
	}
	wantPre0152("down again")
	var status string
	if err := pool.QueryRow(ctx, `SELECT status FROM lite_parent_report WHERE id = $1`, reportID).Scan(&status); err != nil {
		t.Fatalf("read status: %v", err)
	}
	if status != "draft" {
		t.Fatalf("status after Down = %q, want draft", status)
	}
	if _, err := pool.Exec(ctx, `UPDATE lite_parent_report SET status = 'sent' WHERE id = $1`, reportID); err == nil {
		t.Fatal("Down must restore the status CHECK")
	}
	if _, err := pool.Exec(ctx, `UPDATE lite_parent_report SET share_token = 'same' WHERE id = $1`, reportID); err != nil {
		t.Fatalf("set share_token: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO lite_parent_report (user_id, class_id, created_by, range_start, range_end, facts, share_token)
		VALUES ($1, $2, $1, '2026-08-01', '2026-08-28', '{}'::jsonb, 'same')`, refactor2SeededStudentID, classID); err == nil {
		t.Fatal("Down must restore UNIQUE on share_token")
	}

	if err := goose.UpContext(ctx, db, "migrations"); err != nil {
		t.Fatalf("goose up to head: %v", err)
	}
	want0152("back at head")
}
