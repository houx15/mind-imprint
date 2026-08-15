package api_test

// opencard_recovery_test.go — the anchor-less-zombie recovery path. A summon
// that couldn't ground an AI example mints a `proposed` card with `[]` anchors;
// the project-wide one-active mutex then blocks every new lens, yet the resume
// endpoint used to hide that card (it required an on-material anchor), so the
// student could never see, complete, or skip it — a permanent deadlock. The
// endpoint now surfaces an anchor-less open card as a recovery fallback.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/store/sqlc"
)

// TestGetOpenReadingCard_SurfacesAnchorlessInstance — a proposed card with NO
// anchors (the graceful-degrade summon) must still come back from open-card so
// the room can render it (at the first block, client-side) and release the
// mutex by completing or skipping it.
func TestGetOpenReadingCard_SurfacesAnchorlessInstance(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries:      sqlc.New(pool),
		Pool:         pool,
		Provider:     readingStubProvider(""),
		EvalResolver: fakeEvalResolver(),
		SpecByID:     cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)

	mid := ingestReadingMaterial(t, h, cookie, materialsTestProjectID, "全球变暖正在加速冰川融化。")

	ciID := uuid.New()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO card_instances (id, task_id, project_id, card_id, status, anchors) VALUES ($1, NULL, $2, 'argument-map', 'proposed', '[]'::jsonb)`,
		ciID, uuid.MustParse(materialsTestProjectID)); err != nil {
		t.Fatalf("seed anchor-less proposed card instance: %v", err)
	}

	rec := httptest.NewRecorder()
	req := withCookie(httptest.NewRequest("GET",
		"/api/v1/projects/"+materialsTestProjectID+"/materials/"+mid+"/open-card", nil), cookie)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("open-card: %d — %s", rec.Code, rec.Body.String())
	}
	var out struct {
		CardInstanceID string `json:"card_instance_id"`
		CardID         string `json:"card_id"`
		Status         string `json:"status"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v — body: %s", err, rec.Body.String())
	}
	if out.CardInstanceID != ciID.String() {
		t.Fatalf("card_instance_id = %q, want %q (anchor-less card must be surfaced for recovery)", out.CardInstanceID, ciID.String())
	}
	if out.Status != "proposed" || out.CardID != "argument-map" {
		t.Fatalf("got card_id=%q status=%q, want argument-map/proposed", out.CardID, out.Status)
	}
}

// TestGetOpenReadingCard_PrefersOnMaterialOverAnchorless — when both an
// on-material anchored card and an anchor-less one exist, the anchored one wins
// (the anchor-less fallback is only for genuine recovery).
func TestGetOpenReadingCard_PrefersOnMaterialOverAnchorless(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries:      sqlc.New(pool),
		Pool:         pool,
		Provider:     readingStubProvider(""),
		EvalResolver: fakeEvalResolver(),
		SpecByID:     cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)

	mid := ingestReadingMaterial(t, h, cookie, materialsTestProjectID, "全球变暖正在加速冰川融化。")

	// An anchor-less card (would be the fallback) …
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO card_instances (id, task_id, project_id, card_id, status, anchors) VALUES ($1, NULL, $2, 'argument-map', 'proposed', '[]'::jsonb)`,
		uuid.New(), uuid.MustParse(materialsTestProjectID)); err != nil {
		t.Fatalf("seed anchor-less card: %v", err)
	}
	// … and an on-material anchored card that must win.
	anchors, _ := json.Marshal([]map[string]string{
		{"id": "a0", "material_id": mid, "block_id": "b0", "quote": "x", "dimension": "argument-map", "author": "ai"},
	})
	anchoredID := uuid.New()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO card_instances (id, task_id, project_id, card_id, status, anchors) VALUES ($1, NULL, $2, 'craap', 'proposed', $3)`,
		anchoredID, uuid.MustParse(materialsTestProjectID), anchors); err != nil {
		t.Fatalf("seed anchored card: %v", err)
	}

	rec := httptest.NewRecorder()
	req := withCookie(httptest.NewRequest("GET",
		"/api/v1/projects/"+materialsTestProjectID+"/materials/"+mid+"/open-card", nil), cookie)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("open-card: %d — %s", rec.Code, rec.Body.String())
	}
	var out struct {
		CardInstanceID string `json:"card_instance_id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v — body: %s", err, rec.Body.String())
	}
	if out.CardInstanceID != anchoredID.String() {
		t.Fatalf("card_instance_id = %q, want the on-material anchored card %q", out.CardInstanceID, anchoredID.String())
	}
}
