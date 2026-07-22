package api_test

// perspectives_test.go — Task 7: POST /projects/{id}/perspectives (S2).
//
// THE load-bearing test here is TestSubmitPerspectives_DoesNotDeleteCardMintedPerspectives:
// the perspective-matrix tool card (agent/card_effects.go:~130) mints
// `perspective` nodes of its OWN with body {"text","cells"} and NO "origin"
// key at all — a re-save of the S2 view must never touch them, and they must
// still count toward evaluate_perspectives' node_count_at_least{perspective,2}
// gate. The card-minted fixture below is seeded with that exact production
// shape (not a hand-simplified "origin"-carrying stand-in), matching what
// agent.InsertGraphNode actually marshals for a MintNode{Type:"perspective",
// Author:"student", Body:map[string]any{"text":row.Label,"cells":row.Cells}}.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"mindimprint/api/internal/agent"
	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/skills"
	"mindimprint/api/internal/store/sqlc"
)

func TestSubmitPerspectives_DoesNotDeleteCardMintedPerspectives(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)
	pid := createProjectForTest(t, h, cookie)

	q := sqlc.New(pool)

	// Seed ONE card-minted perspective node with the real production shape:
	// body {"text","cells"}, author "student", NO "origin" key. This is what
	// perspective-matrix's graph_effects (card_effects.go) actually produces —
	// verbatim from MintNode's Body literal, not a simplification.
	cardBody, err := json.Marshal(map[string]any{
		"text":  "本地渔民视角",
		"cells": map[string]string{"impact": "近海过度捕捞", "evidence": "渔获量下降报告"},
	})
	if err != nil {
		t.Fatalf("marshal card-minted body: %v", err)
	}
	pidUUID := mustUUID(pid)
	cardNode, err := q.InsertGraphNode(context.Background(), sqlc.InsertGraphNodeParams{
		ProjectID: pidUUID, Type: "perspective", Body: cardBody, Author: "student", SpanRef: nil,
	})
	if err != nil {
		t.Fatalf("seed card-minted perspective: %v", err)
	}

	// Now the station view saves its own two rows.
	body := `{"perspectives":[
		{"text":"国家发展视角","level":"national"},
		{"text":"全球气候影响的正面看法","level":"global_for"}
	]}`
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+pid+"/perspectives",
		strings.NewReader(body)), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("submit = %d, want 200; body=%s", rec.Code, rec.Body)
	}

	// Re-save the station view (e.g. she edits a row). The card-minted node
	// must still be there afterward.
	body2 := `{"perspectives":[
		{"text":"国家发展视角（修订）","level":"national"},
		{"text":"全球气候影响的正面看法","level":"global_for"}
	]}`
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+pid+"/perspectives",
		strings.NewReader(body2)), cookie))
	if rec2.Code != http.StatusOK {
		t.Fatalf("re-submit = %d, want 200; body=%s", rec2.Code, rec2.Body)
	}

	// Half 1: the card-minted node survives, body untouched.
	var gotBody []byte
	var gotAuthor string
	if err := pool.QueryRow(context.Background(),
		`SELECT body, author FROM graph_node WHERE id = $1`, cardNode.ID).Scan(&gotBody, &gotAuthor); err != nil {
		t.Fatalf("card-minted node missing after station-view re-save: %v", err)
	}
	if gotAuthor != "student" {
		t.Errorf("card-minted node author = %q, want student", gotAuthor)
	}
	var gotCard struct {
		Text   string            `json:"text"`
		Cells  map[string]string `json:"cells"`
		Origin string            `json:"origin"`
	}
	if err := json.Unmarshal(gotBody, &gotCard); err != nil {
		t.Fatalf("unmarshal card-minted body: %v; raw=%s", err, gotBody)
	}
	if gotCard.Text != "本地渔民视角" || gotCard.Origin != "" {
		t.Errorf("card-minted body = %+v, want text=本地渔民视角 and no origin (got origin=%q)", gotCard, gotCard.Origin)
	}

	// Station-view rows: exactly 2 (the re-save replaced, didn't accumulate).
	var stationCount int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM graph_node WHERE project_id=$1 AND type='perspective' AND body->>'origin'='station_view'`,
		pidUUID).Scan(&stationCount); err != nil {
		t.Fatalf("count station-view perspectives: %v", err)
	}
	if stationCount != 2 {
		t.Errorf("station-view perspective nodes = %d, want 2", stationCount)
	}

	// Total perspective nodes = card-minted (1) + station-view (2) = 3.
	var total int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM graph_node WHERE project_id=$1 AND type='perspective'`, pidUUID).Scan(&total); err != nil {
		t.Fatalf("count total perspectives: %v", err)
	}
	if total != 3 {
		t.Errorf("total perspective nodes = %d, want 3 (1 card-minted + 2 station-view)", total)
	}

	// Half 2: the gate's node_count_at_least{perspective,2} item counts the
	// card-minted node too — checked directly against agent.CheckGate, the
	// same evaluator advanceGates drives, rather than re-deriving the rule.
	store := agent.NewSqlcAgentStore(sqlc.New(pool), pool)
	g, err := store.LoadGraph(context.Background(), pidUUID)
	if err != nil {
		t.Fatalf("LoadGraph: %v", err)
	}
	sk, ok := skills.ByID("writing-project")
	if !ok {
		t.Fatal("writing-project skill not found")
	}
	report := agent.CheckGate(sk, "evaluate_perspectives", g, agent.RecordedGate{})
	found := false
	for _, item := range report.Items {
		if item.Name == "node_count_at_least:perspective" {
			found = true
			if !item.Pass {
				t.Errorf("node_count_at_least:perspective item.Pass = false, want true (total perspective nodes=%d)", total)
			}
		}
	}
	if !found {
		t.Fatal("no node_count_at_least:perspective item in evaluate_perspectives gate report")
	}
}

// A plain re-save with no card-minted rows in play: editing a row must
// replace, not accumulate, exactly like framing's own ReplacesNodes test.
func TestSubmitPerspectives_ReplacesOwnRows(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)
	pid := createProjectForTest(t, h, cookie)

	body := `{"perspectives":[
		{"text":"国家发展视角","level":"national"},
		{"text":"全球气候正面影响","level":"global_for"},
		{"text":"全球气候负面影响","level":"global_against"}
	]}`
	submit := func(b string) int {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+pid+"/perspectives",
			strings.NewReader(b)), cookie))
		return rec.Code
	}
	if code := submit(body); code != http.StatusOK {
		t.Fatalf("submit = %d, want 200", code)
	}
	if code := submit(body); code != http.StatusOK {
		t.Fatalf("re-submit = %d, want 200", code)
	}

	var count int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM graph_node WHERE project_id=$1 AND type='perspective'`, mustUUID(pid)).Scan(&count); err != nil {
		t.Fatalf("count perspective nodes: %v", err)
	}
	if count != 3 {
		t.Errorf("perspective nodes after two submits = %d, want 3 (re-save must not inflate)", count)
	}

	// Half-finished save (a blank row dropped, not rejected) must also succeed.
	partial := `{"perspectives":[{"text":"","level":"national"},{"text":"仅一行","level":"global_for"}]}`
	if code := submit(partial); code != http.StatusOK {
		t.Fatalf("partial submit = %d, want 200 (an offer is never a wall)", code)
	}
	var count2 int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM graph_node WHERE project_id=$1 AND type='perspective'`, mustUUID(pid)).Scan(&count2); err != nil {
		t.Fatalf("count perspective nodes after partial save: %v", err)
	}
	if count2 != 1 {
		t.Errorf("perspective nodes after partial save = %d, want 1 (blank row dropped)", count2)
	}
}

// An unknown level is a 400 — the three keys (national/global_for/global_against)
// are the binding design's own closed set, not a client-chosen string.
func TestSubmitPerspectives_RejectsUnknownLevel(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)
	pid := createProjectForTest(t, h, cookie)

	body := `{"perspectives":[{"text":"某视角","level":"regional"}]}`
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+pid+"/perspectives",
		strings.NewReader(body)), cookie))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("submit with unknown level = %d, want 400; body=%s", rec.Code, rec.Body)
	}

	var count int
	_ = pool.QueryRow(context.Background(),
		`SELECT count(*) FROM graph_node WHERE project_id=$1 AND type='perspective'`, mustUUID(pid)).Scan(&count)
	if count != 0 {
		t.Errorf("perspective nodes after rejected submit = %d, want 0", count)
	}
}
