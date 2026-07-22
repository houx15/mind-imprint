package api_test

// advance_test.go — Task 4: the first live callers of agent.AdvanceAll
// (Task 3): every gate-affecting write endpoint now re-derives gate state
// from the graph, and logSourceOpen additionally attests
// evaluate_perspectives' recon_logged item (opening/logging a source IS the
// recon, spec §6.2).

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/store/sqlc"
)

// TestLogSourceOpen_AttestsReconLogged — opening a source records the
// evaluate_perspectives gate's recon_logged item as solid. The seeded demo
// project's materialBlogID source-log entry already exists (migration
// 0020), so GetSourceLogByMaterial's own lookup inside logSourceOpen is the
// proof the condition ("the project's source log has >=1 entry") already
// holds by construction before attestReconLogged ever runs.
func TestLogSourceOpen_AttestsReconLogged(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, SpecByID: cards.ByID}).Handler()
	cookie := signInSeed(t, pool)

	rec := openMaterial(t, h, cookie, materialsTestProjectID, materialBlogID, `{"time_spent_s":15}`)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("open material = %d, want 204: %s", rec.Code, rec.Body)
	}

	var body []byte
	if err := pool.QueryRow(context.Background(),
		`SELECT body FROM graph_node WHERE project_id = $1 AND type = 'gate_state' AND body->>'contract' = 'evaluate_perspectives'`,
		materialsTestProjectID).Scan(&body); err != nil {
		t.Fatalf("no evaluate_perspectives gate_state node: %v", err)
	}
	var got struct {
		Items map[string]string `json:"items"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal gate_state body: %v — %s", err, body)
	}
	if got.Items["recon_logged"] != "solid" {
		t.Errorf("evaluate_perspectives items = %+v, want recon_logged=solid", got.Items)
	}
}

// failLoadGraphDB wraps a real sqlc.DBTX (the test pool) and injects a
// failure into exactly the ONE query agent.AdvanceAll's LoadGraph issues
// first (ListGraphNodesByProject) — a query no other codepath in these
// handlers ever runs (the handler's own gate-merge block uses
// ListGateStateNodes/GetGateStateNode, a different query entirely). So every
// other query on the same pool, including the handler's own read/write
// before advanceGates runs, still succeeds untouched: this genuinely fails
// agent.AdvanceAll for THIS request without stubbing it out, rather than
// asserting nothing.
type failLoadGraphDB struct {
	sqlc.DBTX
}

func (f failLoadGraphDB) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if strings.Contains(sql, "ListGraphNodesByProject") {
		return nil, errors.New("advance_test: injected LoadGraph failure")
	}
	return f.DBTX.Query(ctx, sql, args...)
}

// TestAdvanceGates_IsBestEffort — with a store that genuinely fails
// mid-AdvanceAll (LoadGraph errors), attestGate must still return 204: gate
// state is derived and recomputable, so losing one recompute must never
// fail the student's write.
func TestAdvanceGates_IsBestEffort(t *testing.T) {
	pool := newAPITestPool(t)
	cookie := signInSeed(t, pool)

	h := New(Deps{
		Queries:  sqlc.New(failLoadGraphDB{DBTX: pool}),
		Pool:     pool,
		SpecByID: cards.ByID,
	}).Handler()

	rec := httptest.NewRecorder()
	req := withCookie(httptest.NewRequest("POST",
		"/api/v1/projects/"+materialsTestProjectID+"/gate/decode_task/attest",
		strings.NewReader(`{"item":"milestone_plan","confirmed":true}`)), cookie)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("attestGate with a failing AdvanceAll = %d, want 204 (best-effort): %s", rec.Code, rec.Body)
	}

	// The attest itself (the handler's OWN write, via a different query)
	// still landed — proof advanceGates' injected failure didn't corrupt or
	// skip the request it was attached to.
	var itemsBody []byte
	if err := pool.QueryRow(context.Background(),
		`SELECT body FROM graph_node WHERE project_id = $1 AND type = 'gate_state' AND body->>'contract' = 'decode_task'`,
		materialsTestProjectID).Scan(&itemsBody); err != nil {
		t.Fatalf("no decode_task gate_state node: %v", err)
	}
	var got struct {
		Items map[string]string `json:"items"`
	}
	if err := json.Unmarshal(itemsBody, &got); err != nil {
		t.Fatalf("unmarshal gate_state body: %v — %s", err, itemsBody)
	}
	if got.Items["milestone_plan"] != "solid" {
		t.Errorf("decode_task items = %+v, want milestone_plan=solid", got.Items)
	}
}
