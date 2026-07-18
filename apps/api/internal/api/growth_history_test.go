package api_test

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/store/sqlc"
)

// seedGrowthHistoryFixture seeds a project eval + a chat-thread eval for the
// seeded student (2 owned reports), plus one project eval for a DIFFERENT
// user (must be excluded). Mirrors internal/store/growth_history_query_test.go
// (Task 2's store test) for the seed shape.
func seedGrowthHistoryFixture(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()

	// Owned project + its report.
	var projectID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO project (user_id, title) VALUES ($1, '中国可持续') RETURNING id`,
		SeedUserID).Scan(&projectID); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO evaluations (project_id, scores, narrative, model, tier, status)
		VALUES ($1, '[]'::jsonb, 'proj narrative', 'deepseek-v4-pro', 'flagship', 'done')`,
		projectID); err != nil {
		t.Fatalf("seed project eval: %v", err)
	}

	// Owned chat thread + its report.
	var threadID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO chat_thread (user_id, title) VALUES ($1, 'CRAAP 溯源') RETURNING id`,
		SeedUserID).Scan(&threadID); err != nil {
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
	var otherUser string
	if err := pool.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, role, school_id, display_name, avatar_color)
		SELECT 'a3-other-history@example.com', password_hash, 'student', school_id, 'Other', avatar_color
		FROM users WHERE id = $1
		RETURNING id`, SeedUserID).Scan(&otherUser); err != nil {
		t.Fatalf("seed other user: %v", err)
	}
	var otherProject string
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
}

// TestGetGrowthHistory_CrossSurfaceOwnerFiltered — GET /growth/history returns
// every report the caller owns across surfaces, newest-first, each with the
// full report embedded, and NO top-level score/level (RL-5) — plus proves
// another user's report is excluded.
func TestGetGrowthHistory_CrossSurfaceOwnerFiltered(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool}).Handler()
	cookie := signInSeed(t, pool)
	seedGrowthHistoryFixture(t, pool)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/growth/history", nil), cookie))
	if rec.Code != 200 {
		t.Fatalf("GET /growth/history = %d, want 200; body=%s", rec.Code, rec.Body)
	}

	var out struct {
		Entries []map[string]any `json:"entries"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v; body=%s", err, rec.Body)
	}
	if len(out.Entries) != 2 {
		t.Fatalf("entries len = %d, want 2 (own project+chat; other user excluded)", len(out.Entries))
	}

	// Newest-first: the chat report was seeded after the project report.
	if surface, _ := out.Entries[0]["surface"].(string); surface != "chat" {
		t.Errorf("entries[0].surface = %v, want chat (newest-first)", out.Entries[0]["surface"])
	}
	if surface, _ := out.Entries[1]["surface"].(string); surface != "project" {
		t.Errorf("entries[1].surface = %v, want project", out.Entries[1]["surface"])
	}

	for _, e := range out.Entries {
		// RL-5: no top-level score/level/rank — only surface/label/date + the
		// nested report.
		for _, banned := range []string{"score", "level", "rank"} {
			if _, present := e[banned]; present {
				t.Errorf("entry has top-level %q — RL-5 violation: %+v", banned, e)
			}
		}
		report, ok := e["report"].(map[string]any)
		if !ok {
			t.Fatalf("entry.report missing or wrong shape: %+v", e)
		}
		if _, ok := report["dimensions"]; !ok {
			t.Errorf("report.dimensions missing: %+v", report)
		}
		if narrative, _ := report["narrative"].(string); narrative == "" {
			t.Errorf("report.narrative empty: %+v", report)
		}
	}
}

// TestGetGrowthHistory_EmptyWhenNoReports — no reports yet is 200 + empty
// list, never a 404 or null.
func TestGetGrowthHistory_EmptyWhenNoReports(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool}).Handler()
	cookie := signInSeed(t, pool)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/growth/history", nil), cookie))
	if rec.Code != 200 {
		t.Fatalf("GET /growth/history = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var out struct {
		Entries []map[string]any `json:"entries"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v; body=%s", err, rec.Body)
	}
	if out.Entries == nil || len(out.Entries) != 0 {
		t.Fatalf("entries = %v, want empty (non-nil) list", out.Entries)
	}
}
