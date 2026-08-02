package store

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/store/sqlc"
)

func TestListGrowthHistory(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newTestPool(t)
	q := sqlc.New(pool)

	// scores is now the marshalled canonical agent.Report (the DualAxis rows
	// cleared by migration 0027 were themselves cleared again by migration
	// 0028; the product is not in use, so nothing needs to stay compatible
	// with either retired shape).
	reportJSON, err := json.Marshal(agent.Report{
		DepthAxis: []agent.DepthDim{
			{Code: "D1", Name: "任务理解与问题表述", Level: "L3", Evidence: "e"},
		},
		Narrative: "n",
		Axiom:     "两轴永不合成总分；单次会话为事件级证据，不构成人级档位判定",
	})
	if err != nil {
		t.Fatalf("marshal report fixture: %v", err)
	}

	// Owned project + its report.
	var projectID pgtype.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO project (user_id, title) VALUES ($1, '中国可持续') RETURNING id`,
		refactor2SeededStudentID).Scan(&projectID); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO evaluations (project_id, scores, narrative, model, tier, status)
		VALUES ($1, $2, 'proj narrative', 'deepseek-v4-pro', 'flagship', 'done')`,
		projectID, reportJSON); err != nil {
		t.Fatalf("seed project eval: %v", err)
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
		VALUES ($1, $2, 'chat narrative', 'deepseek-v4-pro', 'flagship', 'done')`,
		threadID, reportJSON); err != nil {
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
	// The course-session scope is retired along with course_session itself
	// (migration 0050, course v2, no back-compat) — only project+chat remain.
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2 (own project+chat, other user excluded)", len(rows))
	}
	surfaces := map[string]string{}
	scoresBySurface := map[string][]byte{}
	for _, r := range rows {
		surfaces[r.Surface] = r.Label
		scoresBySurface[r.Surface] = r.Scores
	}
	if surfaces["project"] != "中国可持续" {
		t.Errorf("project label = %q, want 中国可持续", surfaces["project"])
	}
	if surfaces["chat"] != "CRAAP 溯源" {
		t.Errorf("chat label = %q, want CRAAP 溯源", surfaces["chat"])
	}

	// scores is canonical-shaped: the raw column decodes straight into
	// agent.Report, and the depth axis survives the round-trip.
	var report agent.Report
	if err := json.Unmarshal(scoresBySurface["project"], &report); err != nil {
		t.Fatalf("unmarshal project scores into agent.Report: %v", err)
	}
	if len(report.DepthAxis) != 1 || report.DepthAxis[0].Level != "L3" {
		t.Errorf("report.DepthAxis = %+v, want one L3 dim", report.DepthAxis)
	}
}
