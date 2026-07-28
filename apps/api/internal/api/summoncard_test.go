package api_test

// summoncard_test.go — POST .../materials/{mid}/summon-card (SSE): the lens
// library summon path (the student picks a card herself, rather than waiting
// for the read-together router to propose one). Mirrors
// reading_endpoints_test.go's fixtures (readingStubProvider,
// ingestReadingMaterial, materialsTestProjectID).

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/store/sqlc"
)

// TestSummonProjectCard_ValidCardEmitsCardFrame — a valid card_id from the
// reading deck, stubbed to a well-formed ProposeCardExample reply, mints a
// real card_instance and streams an SSE `card` frame carrying a non-empty
// anchors array.
func TestSummonProjectCard_ValidCardEmitsCardFrame(t *testing.T) {
	pool := newAPITestPool(t)
	articleText := "全球变暖正在加速冰川融化，科学家在南极观测到前所未有的冰架断裂。"
	exampleReply := `{"block_id":"b0","quote":"科学家在南极观测到前所未有的冰架断裂","why":"这是文章给出的具体证据。"}`
	h := New(Deps{
		Queries:      sqlc.New(pool),
		Pool:         pool,
		Provider:     readingStubProvider(exampleReply),
		EvalResolver: fakeEvalResolver(),
		SpecByID:     cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)

	mid := ingestReadingMaterial(t, h, cookie, materialsTestProjectID, articleText)

	rec := httptest.NewRecorder()
	req := withCookie(httptest.NewRequest("POST",
		"/api/v1/projects/"+materialsTestProjectID+"/materials/"+mid+"/summon-card",
		strings.NewReader(`{"card_id":"argument-map"}`)), cookie)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("summon-card: %d — %s", rec.Code, rec.Body.String())
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

// TestSummonProjectCard_AlreadyOpenCardRejects — while a card is already
// proposed/active anywhere in the project (the one-active mutex), summoning a
// different card must be rejected: no card frame, just a coach intervention.
func TestSummonProjectCard_AlreadyOpenCardRejects(t *testing.T) {
	pool := newAPITestPool(t)
	articleText := "全球变暖正在加速冰川融化，科学家在南极观测到前所未有的冰架断裂。"
	h := New(Deps{
		Queries:      sqlc.New(pool),
		Pool:         pool,
		Provider:     readingStubProvider(`{"block_id":"b0","quote":"科学家在南极观测到前所未有的冰架断裂","why":"..."}`),
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
		"/api/v1/projects/"+materialsTestProjectID+"/materials/"+mid+"/summon-card",
		strings.NewReader(`{"card_id":"argument-map"}`)), cookie)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("summon-card: %d — %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if strings.Contains(body, "event: card") {
		t.Fatalf("expected no card event while a card is already open:\n%s", body)
	}
	if !strings.Contains(body, "event: intervention") {
		t.Fatalf("expected a coach intervention explaining the rejection:\n%s", body)
	}
	if !strings.Contains(body, "event: done") {
		t.Fatalf("stream missing done:\n%s", body)
	}
}

// TestSummonProjectCard_OutOfDeckIDRejects — a card_id outside
// agent.ReadingDeckIDs (e.g. a Studio-only card) must be rejected without
// ever calling ProposeCardExample or minting a card_instance.
func TestSummonProjectCard_OutOfDeckIDRejects(t *testing.T) {
	pool := newAPITestPool(t)
	articleText := "全球变暖正在加速冰川融化，科学家在南极观测到前所未有的冰架断裂。"
	h := New(Deps{
		Queries:      sqlc.New(pool),
		Pool:         pool,
		Provider:     readingStubProvider(`should never be called`),
		EvalResolver: fakeEvalResolver(),
		SpecByID:     cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)

	mid := ingestReadingMaterial(t, h, cookie, materialsTestProjectID, articleText)

	rec := httptest.NewRecorder()
	req := withCookie(httptest.NewRequest("POST",
		"/api/v1/projects/"+materialsTestProjectID+"/materials/"+mid+"/summon-card",
		strings.NewReader(`{"card_id":"not-a-reading-card"}`)), cookie)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("summon-card: %d — %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if strings.Contains(body, "event: card") {
		t.Fatalf("expected no card event for an out-of-deck id:\n%s", body)
	}
	if !strings.Contains(body, "event: intervention") {
		t.Fatalf("expected a coach intervention explaining the rejection:\n%s", body)
	}
	if !strings.Contains(body, "event: done") {
		t.Fatalf("stream missing done:\n%s", body)
	}

	cis, err := sqlc.New(pool).ListCardInstancesByProject(context.Background(), pgUUID(uuid.MustParse(materialsTestProjectID)))
	if err != nil {
		t.Fatalf("ListCardInstancesByProject: %v", err)
	}
	for _, ci := range cis {
		if ci.CardID == "not-a-reading-card" {
			t.Fatalf("an out-of-deck card_instance was minted")
		}
	}
}

// TestSummonProjectCard_SiftBeforeCraapRejects — SIFT cannot be summoned via
// the lens library before CRAAP has been completed on this material, mirroring
// readturn.go's OrderingGuard.
func TestSummonProjectCard_SiftBeforeCraapRejects(t *testing.T) {
	pool := newAPITestPool(t)
	articleText := "全球变暖正在加速冰川融化，科学家在南极观测到前所未有的冰架断裂。"
	h := New(Deps{
		Queries:      sqlc.New(pool),
		Pool:         pool,
		Provider:     readingStubProvider(`should never be called`),
		EvalResolver: fakeEvalResolver(),
		SpecByID:     cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)

	mid := ingestReadingMaterial(t, h, cookie, materialsTestProjectID, articleText)

	rec := httptest.NewRecorder()
	req := withCookie(httptest.NewRequest("POST",
		"/api/v1/projects/"+materialsTestProjectID+"/materials/"+mid+"/summon-card",
		strings.NewReader(`{"card_id":"sift"}`)), cookie)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("summon-card: %d — %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if strings.Contains(body, "event: card") {
		t.Fatalf("expected no card event — SIFT before CRAAP:\n%s", body)
	}
	if !strings.Contains(body, "event: intervention") {
		t.Fatalf("expected a coach intervention explaining the rejection:\n%s", body)
	}
}

// TestSummonProjectCard_ForeignMaterial404s — a material id that does not
// belong to the owned project is hidden as 404.
func TestSummonProjectCard_ForeignMaterial404s(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries:      sqlc.New(pool),
		Pool:         pool,
		Provider:     readingStubProvider(`should never be called`),
		EvalResolver: fakeEvalResolver(),
		SpecByID:     cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)

	rec := httptest.NewRecorder()
	req := withCookie(httptest.NewRequest("POST",
		"/api/v1/projects/"+materialsTestProjectID+"/materials/00000000-0000-0000-0000-0000000009ff/summon-card",
		strings.NewReader(`{"card_id":"argument-map"}`)), cookie)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("foreign material: want 404, got %d — %s", rec.Code, rec.Body.String())
	}
}
