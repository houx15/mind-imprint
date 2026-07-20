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

func TestSubmitReflection_PersistsNodeAndEvent(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID}).Handler()
	cookie := signInSeed(t, pool)
	pid := createProjectForTest(t, h, cookie)

	const reflectionText = "我一开始以为证据够了，被追问后才发现来源单一，于是补了两个反方来源。"

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+pid+"/reflection",
		strings.NewReader(`{"text":"`+reflectionText+`"}`)), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("reflection = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var nodes, events int
	_ = pool.QueryRow(context.Background(), `SELECT count(*) FROM graph_node WHERE project_id=$1 AND type='reflection' AND author='student'`, pid).Scan(&nodes)
	if nodes != 1 {
		t.Errorf("reflection nodes = %d, want 1", nodes)
	}
	_ = pool.QueryRow(context.Background(), `SELECT count(*) FROM event WHERE project_id=$1 AND type='reflection_written'`, pid).Scan(&events)
	if events != 1 {
		t.Errorf("reflection_written events = %d, want 1", events)
	}

	// Assert body content, not just row counts — catches field-name drift
	// (e.g. a text/body mismatch) that a count-only assertion can't see.
	var nodeBody []byte
	if err := pool.QueryRow(context.Background(),
		`SELECT body FROM graph_node WHERE project_id=$1 AND type='reflection' AND author='student'`, pid).
		Scan(&nodeBody); err != nil {
		t.Fatalf("select node body: %v", err)
	}
	var gotNode struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(nodeBody, &gotNode); err != nil {
		t.Fatalf("unmarshal node body: %v; raw=%s", err, nodeBody)
	}
	if gotNode.Text != reflectionText {
		t.Errorf("node body text = %q, want %q", gotNode.Text, reflectionText)
	}

	// Writing the retro must advance the S6 reflect_archive gate's
	// student_written "reflection" item — the gate checker reads gate_state,
	// not the reflection node itself.
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
	if gotGate.Items["reflection"] != "solid" {
		t.Errorf("reflect_archive gate items[reflection] = %q, want %q", gotGate.Items["reflection"], "solid")
	}
}

func TestSubmitReflection_RejectsTooShort(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID}).Handler()
	cookie := signInSeed(t, pool)
	pid := createProjectForTest(t, h, cookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+pid+"/reflection", strings.NewReader(`{"text":"太短了"}`)), cookie))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("short reflection = %d, want 400", rec.Code)
	}
}

func TestSubmitReflection_OtherUsersProject404(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID}).Handler()
	cookie := signInSeed(t, pool)
	pid := createProjectForTest(t, h, cookie)

	other := createStudent(t, pool, SeedSchoolID, "reflection-other@demo.local")
	otherCookie := signInAs(t, pool, other)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+pid+"/reflection",
		strings.NewReader(`{"text":"我一开始以为证据够了，被追问后才发现来源单一，于是补了两个反方来源。"}`)), otherCookie))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("other user's reflection submit = %d, want 404; body=%s", rec.Code, rec.Body)
	}
}
