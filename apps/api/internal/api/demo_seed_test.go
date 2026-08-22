package api_test

// demo_seed_test.go — asserts migration 0082 seeds the guided-tour demo
// project (00000000-0000-0000-0000-000000000200) with substantive content for
// rooms 立题 / 管理 / 阅读. Tasks 4-5 extend this test (写作/回顾 + eval report).
// TestDemoSeed checks the seed at the DB level; TestDemoSeedReadEndpoints
// checks the same content renders through the real HTTP read endpoints the
// guided tour will hit, signed in as a NON-owner (proving world-readability).

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/store/sqlc"
)

const demoProjectID = "00000000-0000-0000-0000-000000000200"
const demoOwnerID = "00000000-0000-0000-0000-000000000003" // Phoebe

func TestDemoSeed(t *testing.T) {
	pool := newAPITestPool(t)
	ctx := context.Background()

	// Core: the demo project exists, is world-readable demo, owned by Phoebe.
	var owner string
	var isDemo bool
	var status, title string
	var started bool
	if err := pool.QueryRow(ctx,
		`SELECT user_id, is_demo, status, title, (studio_state->>'started')::bool
		   FROM project WHERE id = $1`, demoProjectID,
	).Scan(&owner, &isDemo, &status, &title, &started); err != nil {
		t.Fatalf("demo project row: %v", err)
	}
	if owner != demoOwnerID {
		t.Errorf("demo project owner = %s, want %s (Phoebe)", owner, demoOwnerID)
	}
	if !isDemo {
		t.Error("demo project is_demo = false, want true")
	}
	if !started {
		t.Error("demo project studio_state.started = false, want true")
	}
	if title == "" {
		t.Error("demo project title is empty")
	}

	// 立题: proposal has all four dimensions filled with non-trivial prose.
	var obj, reason, acts, res string
	if err := pool.QueryRow(ctx,
		`SELECT objective, reason, activities, resources
		   FROM project_proposal WHERE project_id = $1`, demoProjectID,
	).Scan(&obj, &reason, &acts, &res); err != nil {
		t.Fatalf("project_proposal row: %v", err)
	}
	for _, d := range []struct {
		name, val string
	}{{"objective", obj}, {"reason", reason}, {"activities", acts}, {"resources", res}} {
		if len([]rune(d.val)) < 20 {
			t.Errorf("proposal.%s too short (%q) — want a real paragraph", d.name, d.val)
		}
	}

	// 管理: ≥3 plan items.
	if n := countDemoRows(t, pool, `SELECT count(*) FROM plan_item WHERE project_id = $1`, demoProjectID); n < 3 {
		t.Errorf("plan_item count = %d, want ≥3", n)
	}

	// 管理: ≥3 activity log entries.
	if n := countDemoRows(t, pool, `SELECT count(*) FROM activity_log_entry WHERE project_id = $1`, demoProjectID); n < 3 {
		t.Errorf("activity_log_entry count = %d, want ≥3", n)
	}

	// 管理/过程: the 立题-done milestone event exists.
	if n := countDemoRows(t, pool,
		`SELECT count(*) FROM event WHERE project_id = $1 AND type = 'milestone:framework_finished'`,
		demoProjectID); n < 1 {
		t.Error("milestone:framework_finished event missing")
	}

	// 论证图: ≥2 graph nodes (a claim + evidence).
	if n := countDemoRows(t, pool, `SELECT count(*) FROM graph_node WHERE project_id = $1`, demoProjectID); n < 2 {
		t.Errorf("graph_node count = %d, want ≥2", n)
	}

	// 阅读: ≥4 references.
	if n := countDemoRows(t, pool, `SELECT count(*) FROM reference WHERE project_id = $1`, demoProjectID); n < 4 {
		t.Errorf("reference count = %d, want ≥4", n)
	}

	// 阅读: ≥1 material with a non-empty blocks array (the reading room renders
	// only when blocks are present).
	if n := countDemoRows(t, pool,
		`SELECT count(*) FROM material WHERE project_id = $1 AND jsonb_array_length(blocks) > 0`,
		demoProjectID); n < 1 {
		t.Error("no material with non-empty blocks for demo project")
	}

	// 阅读: exploration leads + at least one question edge.
	if n := countDemoRows(t, pool, `SELECT count(*) FROM exploration_lead WHERE project_id = $1`, demoProjectID); n < 3 {
		t.Errorf("exploration_lead count = %d, want ≥3", n)
	}
	if n := countDemoRows(t, pool, `SELECT count(*) FROM question_edge WHERE project_id = $1`, demoProjectID); n < 1 {
		t.Error("no question_edge for demo project")
	}

	// 阅读: ≥1 citation.
	if n := countDemoRows(t, pool, `SELECT count(*) FROM citation WHERE project_id = $1`, demoProjectID); n < 1 {
		t.Error("no citation for demo project")
	}

	// studio thread + a studio-surface message.
	if n := countDemoRows(t, pool,
		`SELECT count(*) FROM chat_message m
		   JOIN chat_thread t ON t.id = m.thread_id
		  WHERE t.seeded_project_id = $1 AND m.surface = 'studio'`,
		demoProjectID); n < 1 {
		t.Error("no studio-surface chat_message for demo project")
	}

	// ── 写作 / 回顾 / finished (Task 4) ──────────────────────────────────────

	// Finished: the project has been flipped to status='finished'.
	if status != "finished" {
		t.Errorf("demo project status = %s, want finished", status)
	}

	// 写作: ≥4 outline nodes (depth/position tree).
	if n := countDemoRows(t, pool, `SELECT count(*) FROM outline_node WHERE project_id = $1`, demoProjectID); n < 4 {
		t.Errorf("outline_node count = %d, want ≥4", n)
	}

	// 写作: ≥3 snippets, at least one carrying a section.
	if n := countDemoRows(t, pool, `SELECT count(*) FROM snippet WHERE project_id = $1`, demoProjectID); n < 3 {
		t.Errorf("snippet count = %d, want ≥3", n)
	}
	if n := countDemoRows(t, pool,
		`SELECT count(*) FROM snippet WHERE project_id = $1 AND section IS NOT NULL`, demoProjectID); n < 1 {
		t.Error("no snippet carries a section for demo project")
	}

	// 写作: the essay edit_buffer is non-empty full prose that includes a
	// 反思 / Reflection heading (the report's D6 detection scans for this).
	var essay string
	if err := pool.QueryRow(ctx,
		`SELECT content FROM edit_buffer WHERE project_id = $1 AND doc_kind = 'essay'`, demoProjectID,
	).Scan(&essay); err != nil {
		t.Fatalf("essay edit_buffer row: %v", err)
	}
	if len([]rune(strings.TrimSpace(essay))) < 200 {
		t.Errorf("essay edit_buffer too short (%d runes) — want a full-length essay", len([]rune(essay)))
	}
	if !strings.Contains(essay, "## 反思") && !strings.Contains(essay, "## Reflection") {
		t.Error("essay edit_buffer missing a 反思/Reflection heading (D6 detection depends on it)")
	}

	// 写作: an essay draft snapshot (seq 1) exists.
	if n := countDemoRows(t, pool,
		`SELECT count(*) FROM draft_snapshot WHERE project_id = $1 AND doc_kind = 'essay'`, demoProjectID); n < 1 {
		t.Error("no essay draft_snapshot for demo project")
	}

	// 写作: the essay 完成写作 milestone row exists (REQUIRED for finished).
	if n := countDemoRows(t, pool,
		`SELECT count(*) FROM writing_finish WHERE project_id = $1 AND doc_kind = 'essay'`, demoProjectID); n < 1 {
		t.Error("no writing_finish (doc_kind='essay') for demo project")
	}

	// 回顾: reflection done=true with exactly 5 answers.
	var reflectionDone bool
	var answerCount int
	if err := pool.QueryRow(ctx,
		`SELECT done, jsonb_array_length(answers) FROM project_reflection WHERE project_id = $1`, demoProjectID,
	).Scan(&reflectionDone, &answerCount); err != nil {
		t.Fatalf("project_reflection row: %v", err)
	}
	if !reflectionDone {
		t.Error("project_reflection.done = false, want true")
	}
	if answerCount != 5 {
		t.Errorf("project_reflection answers count = %d, want 5", answerCount)
	}

	// (回顾 mirror-prose was retired in migration 0065; the evaluation report,
	// seeded in Task 5, is the review room's narrative now — nothing to assert.)

	// 过程: the project_finished event exists.
	if n := countDemoRows(t, pool,
		`SELECT count(*) FROM event WHERE project_id = $1 AND type = 'project_finished'`, demoProjectID); n < 1 {
		t.Error("project_finished event missing")
	}
}

// TestDemoSeedReadEndpoints verifies rooms 立题/管理/阅读 render non-empty through
// the actual read endpoints the guided tour hits — decoding real HTTP JSON, not
// re-querying the DB. Signed in as SeedAdminID (…005), a NON-owner: a 200 here
// simultaneously proves the demo project is world-readable to any authenticated
// user regardless of ownership (loadOwnedProjectRow's is_demo bypass).
func TestDemoSeedReadEndpoints(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, CookieSecure: false}).Handler()

	nonOwner := signInAdmin(t, pool) // SeedAdminID …005 ≠ Phoebe …003
	if SeedAdminID.String() == demoOwnerID {
		t.Fatalf("test precondition: signed-in user must NOT be the demo owner")
	}

	get := func(path string) *httptest.ResponseRecorder {
		t.Helper()
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, withCookie(httptest.NewRequest(http.MethodGet, path, nil), nonOwner))
		return rec
	}
	base := "/api/v1/projects/" + demoProjectID

	// 立题: GET /projects/{id} → workspace projection, isDemo + 4-dim proposal.
	{
		rec := get(base)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s (non-owner) = %d, want 200; body=%s", base, rec.Code, rec.Body)
		}
		var proj struct {
			IsDemo   bool   `json:"isDemo"`
			Status   string `json:"status"`
			Proposal struct {
				Objective  string `json:"objective"`
				Reason     string `json:"reason"`
				Activities string `json:"activities"`
				Resources  string `json:"resources"`
			} `json:"proposal"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &proj); err != nil {
			t.Fatalf("decode workspace projection: %v — body=%s", err, rec.Body)
		}
		if !proj.IsDemo {
			t.Error("workspace projection isDemo = false, want true")
		}
		// Task 4: finished project → workspace display status "done".
		if proj.Status != "done" {
			t.Errorf("workspace projection status = %q, want \"done\" (finished)", proj.Status)
		}
		for _, d := range []struct{ name, val string }{
			{"objective", proj.Proposal.Objective}, {"reason", proj.Proposal.Reason},
			{"activities", proj.Proposal.Activities}, {"resources", proj.Proposal.Resources},
		} {
			if strings.TrimSpace(d.val) == "" {
				t.Errorf("workspace proposal.%s is empty over HTTP", d.name)
			}
		}
	}

	// 管理: GET /projects/{id}/plan → non-empty plan items.
	{
		rec := get(base + "/plan")
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s/plan = %d, want 200; body=%s", base, rec.Code, rec.Body)
		}
		var plan struct {
			Items []json.RawMessage `json:"items"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &plan); err != nil {
			t.Fatalf("decode plan: %v — body=%s", err, rec.Body)
		}
		if len(plan.Items) == 0 {
			t.Error("GET /plan returned no items over HTTP")
		}
	}

	// 阅读: GET /projects/{id}/library → ≥4 references, and at least one linking
	// to a seeded material (materialId set) carrying non-empty reading content.
	// (Raw material blocks are not exposed via any GET — they feed the coach
	// turn — so the library reference's material link + content is the
	// HTTP-observable proxy for "the reading room has real material to render".)
	{
		rec := get(base + "/library")
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s/library = %d, want 200; body=%s", base, rec.Code, rec.Body)
		}
		var lib struct {
			References []struct {
				MaterialID  *string `json:"materialId"`
				Abstract    string  `json:"abstract"`
				ReadingNote string  `json:"readingNote"`
			} `json:"references"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &lib); err != nil {
			t.Fatalf("decode library: %v — body=%s", err, rec.Body)
		}
		if len(lib.References) < 4 {
			t.Errorf("GET /library returned %d references over HTTP, want ≥4", len(lib.References))
		}
		materialLinked := false
		for _, ref := range lib.References {
			if ref.MaterialID != nil && *ref.MaterialID != "" &&
				(strings.TrimSpace(ref.Abstract) != "" || strings.TrimSpace(ref.ReadingNote) != "") {
				materialLinked = true
				break
			}
		}
		if !materialLinked {
			t.Error("GET /library: no reference links to a material with non-empty content")
		}
	}
}

func countDemoRows(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		t.Fatalf("count query %q: %v", sql, err)
	}
	return n
}
