package api_test

// reading_endpoints_test.go — Task 5: POST .../materials/{mid}/read-turn (SSE)
// and POST .../cards/{cid}/evaluate (JSON), the read-together router turn and
// selection-evaluate endpoints wiring Tasks 1-4's pure cores
// (RouteReading/ApplyReadingGate/ResolveExampleAnchor/EvaluateSelection) to
// HTTP. Uses the real testcontainers Postgres + the seeded demo project
// (00000000-0000-0000-0000-000000000101, owned by Phoebe) — same fixture
// materials_test.go/studioturn_test.go use.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/store/sqlc"
)

// readingStubProvider replays reply as one text delta then done — mirrors
// fakeProvider/assessStubProvider's shape (studioturn_test.go, assessment_test.go).
func readingStubProvider(reply string) gateway.Provider {
	return gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: reply},
		{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 50, OutputTokens: 20}},
		{Kind: gateway.EventDone, StopReason: gateway.StopStop},
	})
}

// ingestReadingMaterial pastes a single-paragraph article into projectID and
// returns its material id — a fresh material with one known block (id "b0")
// so a router stub reply can name a verbatim example quote from it.
func ingestReadingMaterial(t *testing.T, h http.Handler, cookie *http.Cookie, projectID, text string) string {
	t.Helper()
	raw, err := json.Marshal(map[string]string{
		"title": "阅读材料", "text": text, "takeaway": "t", "tier": "二手",
	})
	if err != nil {
		t.Fatalf("marshal ingest body: %v", err)
	}
	rec := httptest.NewRecorder()
	req := withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+projectID+"/materials", strings.NewReader(string(raw))), cookie)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("ingest material: %d — %s", rec.Code, rec.Body.String())
	}
	var out struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode ingest response: %v", err)
	}
	return out.ID
}

// TestPostReadingTurn_SummonEmitsCardFrame — the router proposes a real
// summon (argument-map) with a verbatim example quote from the material's own
// text; the handler must resolve it into a real anchor and stream an SSE
// `card` frame carrying a non-empty anchors array (never the hardcoded `[]`
// the "lights up nothing" bug produces).
func TestPostReadingTurn_SummonEmitsCardFrame(t *testing.T) {
	pool := newAPITestPool(t)
	articleText := "全球变暖正在加速冰川融化，科学家在南极观测到前所未有的冰架断裂。"
	routerReply := `{"decision":"summon","card_id":"argument-map","reason":"这句话适合用论证地图梳理证据和主张的关系。","example_block_id":"b0","example_quote":"科学家在南极观测到前所未有的冰架断裂","example_why":"这是文章给出的具体证据。","followup_plan":[]}`
	h := New(Deps{
		Queries:      sqlc.New(pool),
		Pool:         pool,
		Provider:     readingStubProvider(routerReply),
		EvalResolver: fakeEvalResolver(),
		SpecByID:     cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool) // Phoebe

	mid := ingestReadingMaterial(t, h, cookie, materialsTestProjectID, articleText)

	rec := httptest.NewRecorder()
	req := withCookie(httptest.NewRequest("POST",
		"/api/v1/projects/"+materialsTestProjectID+"/materials/"+mid+"/read-turn",
		strings.NewReader(`{"student_text":"这条证据能支持什么结论？","focused_spans":[]}`)), cookie)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("read-turn: %d — %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "event: card") || !strings.Contains(body, `"card_id":"argument-map"`) {
		t.Fatalf("expected an argument-map card event:\n%s", body)
	}
	if strings.Contains(body, `"anchors":[]`) {
		t.Fatalf("card frame carries empty anchors:\n%s", body)
	}
	if !strings.Contains(body, "event: done") {
		t.Fatalf("stream missing done:\n%s", body)
	}

	// A proposed card_instance was actually persisted for the material.
	cis, err := sqlc.New(pool).ListCardInstancesByProject(context.Background(), pgUUID(uuid.MustParse(materialsTestProjectID)))
	if err != nil {
		t.Fatalf("ListCardInstancesByProject: %v", err)
	}
	found := false
	for _, ci := range cis {
		if ci.CardID == "argument-map" && ci.Status == "proposed" {
			found = true
		}
	}
	if !found {
		t.Fatalf("no proposed argument-map card_instance persisted")
	}
}

// TestPostReadingTurn_OpenCardSuppresses — while a card is already
// proposed/active anywhere in the project (the one-active mutex), the router
// is still consulted but ApplyReadingGate must downgrade any summon it
// proposes to a plain respond: no new `card` event, done only.
func TestPostReadingTurn_OpenCardSuppresses(t *testing.T) {
	pool := newAPITestPool(t)
	articleText := "全球变暖正在加速冰川融化，科学家在南极观测到前所未有的冰架断裂。"
	routerReply := `{"decision":"summon","card_id":"argument-map","reason":"...","example_block_id":"b0","example_quote":"科学家在南极观测到前所未有的冰架断裂","example_why":"...","followup_plan":[]}`
	h := New(Deps{
		Queries:      sqlc.New(pool),
		Pool:         pool,
		Provider:     readingStubProvider(routerReply),
		EvalResolver: fakeEvalResolver(),
		SpecByID:     cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)

	mid := ingestReadingMaterial(t, h, cookie, materialsTestProjectID, articleText)

	// Seed an in-flight (proposed) card_instance on the project — anywhere,
	// not necessarily on this material; the mutex is project-wide.
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO card_instances (id, task_id, project_id, card_id, status) VALUES ($1, NULL, $2, 'toulmin', 'proposed')`,
		uuid.New(), uuid.MustParse(materialsTestProjectID)); err != nil {
		t.Fatalf("seed open card: %v", err)
	}

	rec := httptest.NewRecorder()
	req := withCookie(httptest.NewRequest("POST",
		"/api/v1/projects/"+materialsTestProjectID+"/materials/"+mid+"/read-turn",
		strings.NewReader(`{"student_text":"这条证据能支持什么结论？","focused_spans":[]}`)), cookie)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("read-turn: %d — %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if strings.Contains(body, "event: card") {
		t.Fatalf("expected no card event while a card is already open:\n%s", body)
	}
	if !strings.Contains(body, "event: done") {
		t.Fatalf("stream missing done:\n%s", body)
	}
}

// TestPostReadingTurn_SkippedInstanceSuppressesResummon — a card the student
// already skipped on THIS material must not be re-summoned: ApplyReadingGate's
// skip-cooldown reads Pacing.RecentlySkipped, which readturn.go derives from
// card_instance status + the anchors persisted on each instance (NOT the
// card_instance--evaluates-->material edge, which store.CreateCardInstance —
// what THIS endpoint calls to summon — never mints; only agent.SurfaceCard's
// graph effects do). Seeds a status='skipped' argument-map card_instance whose
// anchors carry this material's id directly via SQL (mirrors how
// TestPostReadingTurn_OpenCardSuppresses seeds a 'proposed' row for the
// project-wide mutex), scripts the router to summon that SAME card again, and
// asserts no `card` SSE frame is emitted. This test fails against the old
// edge-based derivation (skippedOnMaterial stays empty, nothing suppresses
// the re-summon) and passes once pacing is derived from card_instance status.
func TestPostReadingTurn_SkippedInstanceSuppressesResummon(t *testing.T) {
	pool := newAPITestPool(t)
	articleText := "全球变暖正在加速冰川融化，科学家在南极观测到前所未有的冰架断裂。"
	routerReply := `{"decision":"summon","card_id":"argument-map","reason":"这句话适合用论证地图梳理证据和主张的关系。","example_block_id":"b0","example_quote":"科学家在南极观测到前所未有的冰架断裂","example_why":"这是文章给出的具体证据。","followup_plan":[]}`
	h := New(Deps{
		Queries:      sqlc.New(pool),
		Pool:         pool,
		Provider:     readingStubProvider(routerReply),
		EvalResolver: fakeEvalResolver(),
		SpecByID:     cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)

	mid := ingestReadingMaterial(t, h, cookie, materialsTestProjectID, articleText)

	// Seed a SKIPPED argument-map card_instance already anchored to THIS
	// material (anchors is the only place a card_instance carries a material
	// id when it was minted by store.CreateCardInstance rather than
	// agent.SurfaceCard — there is no material_id column on card_instances).
	anchors, err := json.Marshal([]map[string]string{
		{"id": "a0", "material_id": mid, "block_id": "b0", "quote": "x", "dimension": "argument-map", "author": "ai"},
	})
	if err != nil {
		t.Fatalf("marshal seed anchors: %v", err)
	}
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO card_instances (id, task_id, project_id, card_id, status, anchors) VALUES ($1, NULL, $2, 'argument-map', 'skipped', $3)`,
		uuid.New(), uuid.MustParse(materialsTestProjectID), anchors); err != nil {
		t.Fatalf("seed skipped card instance: %v", err)
	}

	rec := httptest.NewRecorder()
	req := withCookie(httptest.NewRequest("POST",
		"/api/v1/projects/"+materialsTestProjectID+"/materials/"+mid+"/read-turn",
		strings.NewReader(`{"student_text":"这条证据能支持什么结论？","focused_spans":[]}`)), cookie)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("read-turn: %d — %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if strings.Contains(body, "event: card") {
		t.Fatalf("expected no card event — argument-map was already skipped on this material:\n%s", body)
	}
	if !strings.Contains(body, "event: done") {
		t.Fatalf("stream missing done:\n%s", body)
	}
}

// TestEvaluateProjectCard_ReturnsProgramVerdict — the verdict is PROGRAM-owned
// (agent.SelectionEval's verdictFromChecks), never trusted from the model: a
// stub reply whose checks fail `target` must come back "rethink" over HTTP
// even though nothing in the reply literally says "rethink".
func TestEvaluateProjectCard_ReturnsProgramVerdict(t *testing.T) {
	pool := newAPITestPool(t)
	evalReply := `{"checks":[` +
		`{"key":"target","status":"miss","evidence":"","explanation":"没找对对象"},` +
		`{"key":"evidence","status":"pass","evidence":"","explanation":"有线索"},` +
		`{"key":"centrality","status":"pass","evidence":"","explanation":"关键"}` +
		`],"finding":"她选的句子偏题了","judgment":"","support":"","caveat":"","next_step":"回到文章重新找一句"}`
	h := New(Deps{
		Queries:      sqlc.New(pool),
		Pool:         pool,
		Provider:     readingStubProvider(evalReply),
		EvalResolver: fakeEvalResolver(),
		SpecByID:     cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)

	// The seeded demo project's own steelman card_instance (completed,
	// project-scoped) — evaluateProjectCard reads/writes it without caring
	// about its lifecycle status, so no fresh card needs to be minted first.
	cid := "00000000-0000-0000-0000-000000000130"

	rec := httptest.NewRecorder()
	req := withCookie(httptest.NewRequest("POST",
		"/api/v1/projects/"+materialsTestProjectID+"/cards/"+cid+"/evaluate",
		strings.NewReader(`{"block_id":"b1","start":0,"end":6,"quote":"这是学生选的句子","dimension":"claim"}`)), cookie)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("evaluate: %d — %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("content-type = %q, want application/json", ct)
	}
	var out struct {
		Verdict string `json:"verdict"`
		Checks  []struct {
			Key    string `json:"key"`
			Status string `json:"status"`
		} `json:"checks"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v — body: %s", err, rec.Body.String())
	}
	if out.Verdict != "rethink" {
		t.Fatalf("verdict = %q, want rethink (target miss overrides the model's own framing)", out.Verdict)
	}
	if len(out.Checks) != 3 {
		t.Fatalf("checks length = %d, want 3", len(out.Checks))
	}

	// The eval was persisted onto the card_instance's framework_fill.
	row, err := sqlc.New(pool).GetCardInstance(context.Background(), uuid.MustParse(cid))
	if err != nil {
		t.Fatalf("GetCardInstance: %v", err)
	}
	var persisted struct {
		Verdict string `json:"verdict"`
	}
	if err := json.Unmarshal(row.FrameworkFill, &persisted); err != nil {
		t.Fatalf("unmarshal persisted framework_fill: %v — raw: %s", err, row.FrameworkFill)
	}
	if persisted.Verdict != "rethink" {
		t.Fatalf("persisted framework_fill verdict = %q, want rethink", persisted.Verdict)
	}
}

// TestPostReadingTurn_ForeignMaterial404s — a material id that does not
// belong to the owned project is hidden as 404, same convention as
// prepareSourceAnnotation/loadOwnedProject.
func TestPostReadingTurn_ForeignMaterial404s(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries:      sqlc.New(pool),
		Pool:         pool,
		Provider:     readingStubProvider(`{"decision":"respond","card_id":"","reason":"","example_block_id":"","example_quote":"","example_why":"","followup_plan":[]}`),
		EvalResolver: fakeEvalResolver(),
		SpecByID:     cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)

	rec := httptest.NewRecorder()
	req := withCookie(httptest.NewRequest("POST",
		"/api/v1/projects/"+materialsTestProjectID+"/materials/00000000-0000-0000-0000-0000000009ff/read-turn",
		strings.NewReader(`{"student_text":"x","focused_spans":[]}`)), cookie)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("foreign material: want 404, got %d — %s", rec.Code, rec.Body.String())
	}
}
