package store

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/store/sqlc"
)

func TestListGrowthHistory(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newTestPool(t)
	q := sqlc.New(pool)

	// Owned project + its report.
	var projectID pgtype.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO project (user_id, title) VALUES ($1, '中国可持续') RETURNING id`,
		refactor2SeededStudentID).Scan(&projectID); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO evaluations (project_id, scores, narrative, model, tier, status)
		VALUES ($1, '[]'::jsonb, 'proj narrative', 'deepseek-v4-pro', 'flagship', 'done')`,
		projectID); err != nil {
		t.Fatalf("seed project eval: %v", err)
	}

	// Owned course session + its report (course c1 is seeded by 0012).
	var sessionID pgtype.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO course_session (user_id, course_id, skill_id, phase)
		VALUES ($1, '00000000-0000-0000-0000-0000000000c1', 'info-literacy-course', 'reflect')
		RETURNING id`, refactor2SeededStudentID).Scan(&sessionID); err != nil {
		t.Fatalf("seed course_session: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO evaluations (session_id, scores, narrative, model, tier, status)
		VALUES ($1, '[]'::jsonb, 'course narrative', 'deepseek-v4-pro', 'flagship', 'done')`,
		sessionID); err != nil {
		t.Fatalf("seed session eval: %v", err)
	}

	// Owned chat thread + its report.
	var threadID pgtype.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO chat_thread (user_id, title) VALUES ($1, 'CRAAP 溯源') RETURNING id`,
		refactor2SeededStudentID).Scan(&threadID); err != nil {
		t.Fatalf("seed chat_thread: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO evaluations (thread_id, scores, narrative, model, tier, status)
		VALUES ($1, '[]'::jsonb, 'chat narrative', 'deepseek-v4-pro', 'flagship', 'done')`,
		threadID); err != nil {
		t.Fatalf("seed thread eval: %v", err)
	}

	// A DIFFERENT user's project report — must be excluded. users has more
	// NOT NULL columns than email/role/school_id (password_hash, display_name,
	// avatar_color; the column is display_name, not name), so copy those from
	// the seeded student's own row rather than inventing values.
	var otherUser pgtype.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, role, school_id, display_name, avatar_color)
		SELECT 'a3-other@example.com', password_hash, 'student', school_id, 'Other', avatar_color
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

	rows, err := q.ListGrowthHistory(ctx, refactor2SeededStudentID)
	if err != nil {
		t.Fatalf("ListGrowthHistory: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("got %d rows, want 3 (own project+course+chat, other user excluded)", len(rows))
	}
	surfaces := map[string]string{}
	for _, r := range rows {
		surfaces[r.Surface] = r.Label
	}
	if surfaces["project"] != "中国可持续" {
		t.Errorf("project label = %q, want 中国可持续", surfaces["project"])
	}
	if surfaces["course"] != "信息素养" && surfaces["course"] == "" {
		t.Errorf("course row missing; got labels %v", surfaces)
	}
	if surfaces["chat"] != "CRAAP 溯源" {
		t.Errorf("chat label = %q, want CRAAP 溯源", surfaces["chat"])
	}
}
