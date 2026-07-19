package store

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/store/sqlc"
)

// C ability model: ListEvaluationsByUser returns EVERY evaluation the caller
// owns across all three scopes (not latest-per-scope like ListGrowthHistory),
// oldest-first. Mirrors growth_history_query_test.go's pool/seed harness;
// only the assertions differ (two project evaluations for the owner at
// different created_at, one for a different user, ascending order, owner
// isolation).
func TestListEvaluationsByUser_OwnerFilteredAllRows(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newTestPool(t)
	q := sqlc.New(pool)

	scoresDay1 := []byte(`{"depthAxis":{"dims":[{"code":"D1","score":3}],"subtotal":3}}`)
	scoresDay2 := []byte(`{"depthAxis":{"dims":[{"code":"D1","score":4}],"subtotal":4}}`)

	// Owned project + two evaluations at different created_at.
	var projectID pgtype.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO project (user_id, title) VALUES ($1, '中国可持续') RETURNING id`,
		refactor2SeededStudentID).Scan(&projectID); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO evaluations (project_id, scores, narrative, model, tier, status, created_at)
		VALUES ($1, $2, 'day1', 'deepseek-v4-pro', 'flagship', 'done', now() - interval '2 days')`,
		projectID, scoresDay1); err != nil {
		t.Fatalf("seed project eval day1: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO evaluations (project_id, scores, narrative, model, tier, status, created_at)
		VALUES ($1, $2, 'day2', 'deepseek-v4-pro', 'flagship', 'done', now() - interval '1 day')`,
		projectID, scoresDay2); err != nil {
		t.Fatalf("seed project eval day2: %v", err)
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
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2 (owner's two project evals, other user excluded)", len(rows))
	}
	if !rows[0].CreatedAt.Before(rows[1].CreatedAt) {
		t.Errorf("rows not ascending by created_at: rows[0]=%v rows[1]=%v", rows[0].CreatedAt, rows[1].CreatedAt)
	}
	// scores round-trips through jsonb, which reformats whitespace, so compare
	// decoded structure rather than raw bytes.
	if subtotal(t, rows[0].Scores) != 3 {
		t.Errorf("rows[0].Scores subtotal = %d, want 3 (oldest first)", subtotal(t, rows[0].Scores))
	}
	if subtotal(t, rows[1].Scores) != 4 {
		t.Errorf("rows[1].Scores subtotal = %d, want 4", subtotal(t, rows[1].Scores))
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
