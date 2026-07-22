package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/store/sqlc"
)

// TestSignDeclaration_PersistsNodeAndSolidifiesGate proves signDeclaration
// persists a `declaration` graph_node (author "student") and flips the S6
// reflect_archive gate's human `declaration_signed` item to solid, in one
// transaction — and that a second sign is idempotent (no second node).
func TestSignDeclaration_PersistsNodeAndSolidifiesGate(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID}).Handler()
	cookie := signInSeed(t, pool)
	pid := createProjectForTest(t, h, cookie)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+pid+"/declaration/sign", strings.NewReader("")), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("sign declaration = %d, want 200; body=%s", rec.Code, rec.Body)
	}

	var nodeCount int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM graph_node WHERE project_id=$1 AND type='declaration' AND author='student'`, pid).
		Scan(&nodeCount); err != nil {
		t.Fatalf("count declaration nodes: %v", err)
	}
	if nodeCount != 1 {
		t.Fatalf("declaration nodes = %d, want 1", nodeCount)
	}

	var nodeBody []byte
	if err := pool.QueryRow(context.Background(),
		`SELECT body FROM graph_node WHERE project_id=$1 AND type='declaration' AND author='student'`, pid).
		Scan(&nodeBody); err != nil {
		t.Fatalf("select declaration body: %v", err)
	}
	var got struct {
		Asks             int `json:"asks"`
		Dispositions     int `json:"dispositions"`
		CardsSpontaneous int `json:"cardsSpontaneous"`
		CardsPrompted    int `json:"cardsPrompted"`
		AiWrittenProse   int `json:"aiWrittenProse"`
	}
	if err := json.Unmarshal(nodeBody, &got); err != nil {
		t.Fatalf("unmarshal declaration body: %v; raw=%s", err, nodeBody)
	}
	if got.AiWrittenProse != 0 {
		t.Errorf("declaration body aiWrittenProse = %d, want 0 (RL-1: no AI text ever reaches the draft)", got.AiWrittenProse)
	}
	// A freshly created project has done none of this yet — all-zero is the
	// honest reading, not a fabricated placeholder.
	if got.Asks != 0 || got.Dispositions != 0 || got.CardsSpontaneous != 0 || got.CardsPrompted != 0 {
		t.Errorf("declaration body on a fresh project = %+v, want all zero", got)
	}

	// The gate checker (agent.CheckGate) reads gate_state, not the
	// declaration node itself — assert the human item is recorded solid.
	var gateBody []byte
	if err := pool.QueryRow(context.Background(),
		`SELECT body FROM graph_node WHERE project_id=$1 AND type='gate_state' AND body->>'contract'='reflect_archive'`, pid).
		Scan(&gateBody); err != nil {
		t.Fatalf("select reflect_archive gate_state: %v", err)
	}
	var gotGate struct {
		Items map[string]string `json:"items"`
	}
	if err := json.Unmarshal(gateBody, &gotGate); err != nil {
		t.Fatalf("unmarshal gate_state body: %v; raw=%s", err, gateBody)
	}
	if gotGate.Items["declaration_signed"] != "solid" {
		t.Errorf("reflect_archive gate items[declaration_signed] = %q, want %q", gotGate.Items["declaration_signed"], "solid")
	}

	// Second sign must be idempotent: no second node minted.
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+pid+"/declaration/sign", strings.NewReader("")), cookie))
	if rec2.Code != http.StatusOK {
		t.Fatalf("second sign declaration = %d, want 200; body=%s", rec2.Code, rec2.Body)
	}
	var nodeCount2 int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM graph_node WHERE project_id=$1 AND type='declaration'`, pid).Scan(&nodeCount2); err != nil {
		t.Fatalf("count declaration nodes after 2nd sign: %v", err)
	}
	if nodeCount2 != 1 {
		t.Errorf("declaration nodes after 2nd sign = %d, want 1 (idempotent, no duplicate mint)", nodeCount2)
	}
}

func TestSignDeclaration_OtherUsersProject404(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID}).Handler()
	cookie := signInSeed(t, pool)
	pid := createProjectForTest(t, h, cookie)

	other := createStudent(t, pool, SeedSchoolID, "declaration-other@demo.local")
	otherCookie := signInAs(t, pool, other)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+pid+"/declaration/sign", strings.NewReader("")), otherCookie))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("other user's declaration sign = %d, want 404; body=%s", rec.Code, rec.Body)
	}
}
