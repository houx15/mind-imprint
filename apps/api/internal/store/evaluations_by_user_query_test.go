package store

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/store/sqlc"
)

// C ability model: ListEvaluationsByUser returns the LATEST evaluation per scope
// the caller owns across all three scopes, oldest-first. The DISTINCT ON hardening
// (this query defends the one-report-per-session invariant itself) means two rows
// on the SAME scope collapse to the latest — so this test seeds a same-scope
// duplicate and asserts it collapses, plus a second owned scope to exercise
// cross-scope ascending order and owner isolation. Mirrors
// growth_history_query_test.go's pool/seed harness.
func TestListEvaluationsByUser_LatestPerScopeOwnerFiltered(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newTestPool(t)
	q := sqlc.New(pool)

	scoresOld := []byte(`{"depthAxis":{"dims":[{"code":"D1","score":3}],"subtotal":3}}`)
	scoresNew := []byte(`{"depthAxis":{"dims":[{"code":"D1","score":5}],"subtotal":5}}`)
	scoresB := []byte(`{"depthAxis":{"dims":[{"code":"D1","score":9}],"subtotal":9}}`)

	// Project A (owned) with TWO evaluations at different created_at — a
	// same-scope duplicate. DISTINCT ON must keep only the latest (subtotal 5).
	var projectA pgtype.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO project (user_id, title) VALUES ($1, '中国可持续') RETURNING id`,
		refactor2SeededStudentID).Scan(&projectA); err != nil {
		t.Fatalf("seed project A: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO evaluations (project_id, scores, narrative, model, tier, status, created_at)
		VALUES ($1, $2, 'A-old', 'deepseek-v4-pro', 'flagship', 'done', now() - interval '3 days')`,
		projectA, scoresOld); err != nil {
		t.Fatalf("seed project A eval old: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO evaluations (project_id, scores, narrative, model, tier, status, created_at)
		VALUES ($1, $2, 'A-new', 'deepseek-v4-pro', 'flagship', 'done', now() - interval '1 day')`,
		projectA, scoresNew); err != nil {
		t.Fatalf("seed project A eval new: %v", err)
	}

	// Project B (owned) with ONE evaluation, older than A's latest — so
	// ascending order puts B (day -2) before A's latest (day -1).
	var projectB pgtype.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO project (user_id, title) VALUES ($1, '第二个任务') RETURNING id`,
		refactor2SeededStudentID).Scan(&projectB); err != nil {
		t.Fatalf("seed project B: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO evaluations (project_id, scores, narrative, model, tier, status, created_at)
		VALUES ($1, $2, 'B', 'deepseek-v4-pro', 'flagship', 'done', now() - interval '2 days')`,
		projectB, scoresB); err != nil {
		t.Fatalf("seed project B eval: %v", err)
	}

	// A DIFFERENT user's project report — must be excluded. users has more
	// NOT NULL columns than email/role/school_id (password_hash, display_name,
	// avatar_color; the column is display_name, not name), so copy those from
	// the seeded student's own row rather than inventing values.
	var otherUser pgtype.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, role, school_id, display_name, avatar_color)
		SELECT 'c2-other@example.com', password_hash, 'student', school_id, 'Other', avatar_color
		FROM users WHERE id = $1
		RETURNING id`, refactor2SeededStudentID).Scan(&otherUser); err != nil {
		t.Fatalf("seed other user: %v", err)
	}
	var otherProject pgtype.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO project (user_id, title) VALUES ($1, 'not mine') RETURNING id`,
		otherUser).Scan(&otherProject); err != nil {
		t.Fatalf("seed other project: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO evaluations (project_id, scores, narrative, model, tier, status)
		VALUES ($1, '[]'::jsonb, 'not mine', 'deepseek-v4-pro', 'flagship', 'done')`,
		otherProject); err != nil {
		t.Fatalf("seed other eval: %v", err)
	}

	rows, err := q.ListEvaluationsByUser(ctx, refactor2SeededStudentID)
	if err != nil {
		t.Fatalf("ListEvaluationsByUser: %v", err)
	}
	// Two OWNED scopes, each collapsed to its latest row; project A's older
	// duplicate is dropped, the other user's row excluded.
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2 (latest-per-scope: A collapses to 1, B is 1, other user excluded)", len(rows))
	}
	if !rows[0].CreatedAt.Before(rows[1].CreatedAt) {
		t.Errorf("rows not ascending by created_at: rows[0]=%v rows[1]=%v", rows[0].CreatedAt, rows[1].CreatedAt)
	}
	// scores round-trips through jsonb, which reformats whitespace, so compare
	// decoded structure rather than raw bytes. Ascending: B (day -2, subtotal 9)
	// then A's latest (day -1, subtotal 5). A's OLD row (subtotal 3) must be gone.
	if subtotal(t, rows[0].Scores) != 9 {
		t.Errorf("rows[0].Scores subtotal = %d, want 9 (project B, oldest of the survivors)", subtotal(t, rows[0].Scores))
	}
	if subtotal(t, rows[1].Scores) != 5 {
		t.Errorf("rows[1].Scores subtotal = %d, want 5 (project A's LATEST, not the dropped 3)", subtotal(t, rows[1].Scores))
	}
}

func subtotal(t *testing.T, scores []byte) int {
	t.Helper()
	var decoded struct {
		DepthAxis struct {
			Subtotal int `json:"subtotal"`
		} `json:"depthAxis"`
	}
	if err := json.Unmarshal(scores, &decoded); err != nil {
		t.Fatalf("unmarshal scores: %v", err)
	}
	return decoded.DepthAxis.Subtotal
}
