package api_test

// studioturn_test.go — Task 6: POST /api/v1/projects/{id}/turn, the SSE loop
// driver that runs one RunAgentStep with surface-card production on (Slice
// 5c-2's tool-card transport; SkipSurfaceCards: false). Uses the real
// testcontainers Postgres + the seeded demo project
// (00000000-0000-0000-0000-000000000101, owned by Phoebe) so RunAgentStep
// exercises a real graph, not a fixture.

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/agent"
	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/store/sqlc"
)

// fakeProvider is a gateway.Provider whose Stream emits a short text reply
// then closes — the coach may or may not be invoked for the seeded graph
// (RunAgentStep can legitimately stay silent), but if it is, this is enough
// for ProposeIntervention/gateway.Collect to complete without a live model.
func fakeProvider() gateway.Provider {
	return gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: "先说说你打算怎么把这条证据接上主张？"},
		{Kind: gateway.EventDone, StopReason: gateway.StopStop},
	})
}

// fakeResolver is a gateway.KeyResolver returning a dummy Resolved — no real
// key, no network; the fakeProvider never reads it.
func fakeResolver() gateway.KeyResolver {
	return func(context.Context) (gateway.Resolved, error) {
		return gateway.Resolved{Provider: "deepseek", Model: "deepseek-chat", Tier: "chaperone"}, nil
	}
}

// pgUUID converts a uuid.UUID to the pgtype.UUID sqlc expects.
func pgUUID(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}

func TestProjectTurn(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries:      sqlc.New(pool),
		Pool:         pool,
		Provider:     fakeProvider(),
		ChatResolver: fakeResolver(),
		SpecByID:     cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool) // Phoebe
	projectID := "00000000-0000-0000-0000-000000000101"

	// 401 unauth.
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("POST", "/api/v1/projects/"+projectID+"/turn", strings.NewReader(`{"user_input":"hi there enough"}`)))
	if rr.Code != 401 {
		t.Fatalf("unauth: want 401, got %d", rr.Code)
	}

	// happy path — SSE with a done event, and a persisted chat_message.
	rr = httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/projects/"+projectID+"/turn", strings.NewReader(`{"user_input":"它想证明中国在认真转型呢"}`))
	h.ServeHTTP(rr, withCookie(req, cookie))
	if rr.Code != 200 {
		t.Fatalf("turn: %d — %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "event: done") {
		t.Fatalf("stream missing done:\n%s", rr.Body.String())
	}
	msgs, err := sqlc.New(pool).ListChatMessagesByProject(context.Background(), pgUUID(uuid.MustParse(projectID)))
	if err != nil {
		t.Fatalf("ListChatMessagesByProject: %v", err)
	}
	if len(msgs) < 1 {
		t.Fatalf("student message not persisted")
	}

	// 404 non-owned.
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", "/api/v1/projects/00000000-0000-0000-0000-0000000009ff/turn", strings.NewReader(`{"user_input":"whatever long enough"}`)), cookie))
	if rr.Code != 404 {
		t.Fatalf("foreign: want 404, got %d", rr.Code)
	}
}

// TestProjectTurn_SurfacesCraapCard — the seeded project's article materials
// are un-evaluated (no "evaluated-as" edge in migration 0018), so with
// SkipSurfaceCards off, a turn surfaces the CRAAP card instead of staying
// silent or emitting an intervention.
func TestProjectTurn_SurfacesCraapCard(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, Provider: fakeProvider(), ChatResolver: fakeResolver(), SpecByID: cards.ByID}).Handler()
	cookie := signInSeed(t, pool)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/projects/00000000-0000-0000-0000-000000000101/turn", strings.NewReader(`{"user_input":"这条来源可信吗"}`))
	h.ServeHTTP(rr, withCookie(req, cookie))
	body := rr.Body.String()
	if !strings.Contains(body, "event: card") || !strings.Contains(body, `"card_id":"craap"`) {
		t.Fatalf("expected a craap card event:\n%s", body)
	}
	// a card_instance (proposed) was persisted for the project
	cis, _ := sqlc.New(pool).ListCardInstancesByProject(context.Background(), pgUUID(uuid.MustParse("00000000-0000-0000-0000-000000000101")))
	if len(cis) == 0 {
		t.Fatalf("no card_instance persisted")
	}
}

// TestProjectTurn_SurfacesCraapCard_GeneratesAnchors — Task 2: the Studio
// surface seam (streamAction -> surfaceAnchors) must generate + persist AI
// anchors on the craap card_instance it just proposed, not emit an empty
// `[]` anchors payload. Drives the real HTTP turn endpoint (same trigger as
// TestProjectTurn_SurfacesCraapCard) with the fake provider/resolver: the
// stub's scripted reply is not valid anchor-gen JSON, so
// AnchorGenerator.Generate falls back to its deterministic per-tag path —
// exercising the surface seam end to end with no live model/key required.
func TestProjectTurn_SurfacesCraapCard_GeneratesAnchors(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, Provider: fakeProvider(), ChatResolver: fakeResolver(), SpecByID: cards.ByID}).Handler()
	cookie := signInSeed(t, pool)
	projectID := uuid.MustParse("00000000-0000-0000-0000-000000000101")

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/projects/00000000-0000-0000-0000-000000000101/turn", strings.NewReader(`{"user_input":"这条来源可信吗"}`))
	h.ServeHTTP(rr, withCookie(req, cookie))
	body := rr.Body.String()
	if !strings.Contains(body, "event: card") || !strings.Contains(body, `"card_id":"craap"`) {
		t.Fatalf("expected a craap card event:\n%s", body)
	}
	// The SSE `card` frame itself must no longer carry the hardcoded `[]`.
	if strings.Contains(body, `"anchors":[]`) {
		t.Fatalf("card frame still emits empty anchors:\n%s", body)
	}

	q := sqlc.New(pool)
	cis, err := q.ListCardInstancesByProject(context.Background(), pgUUID(projectID))
	if err != nil {
		t.Fatalf("ListCardInstancesByProject: %v", err)
	}
	var cid uuid.UUID
	found := false
	for _, ci := range cis {
		if ci.CardID == "craap" {
			cid = ci.ID
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("no craap card_instance persisted")
	}

	row, err := q.GetCardInstance(context.Background(), cid)
	if err != nil {
		t.Fatalf("GetCardInstance: %v", err)
	}
	var anchors []agent.Anchor
	if err := json.Unmarshal(row.Anchors, &anchors); err != nil {
		t.Fatalf("unmarshal persisted anchors: %v — raw: %s", err, row.Anchors)
	}
	if len(anchors) == 0 {
		t.Fatalf("expected generated anchors on annotate-card surface, got none")
	}
	tags := map[string]bool{"currency": true, "relevance": true, "authority": true, "accuracy": true, "purpose": true}
	for _, a := range anchors {
		if !tags[a.Dimension] {
			t.Fatalf("persisted anchor dimension %q is not a completion tag", a.Dimension)
		}
	}
}

// TestProjectTurn_TouchesLastActiveAt — Slice 5d: a turn is activity — the
// roster's 最近活跃 depends on project.last_active_at, which is otherwise
// only set once, at project creation. A successful turn must strictly
// advance it.
func TestProjectTurn_TouchesLastActiveAt(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, Provider: fakeProvider(), ChatResolver: fakeResolver(), SpecByID: cards.ByID}).Handler()
	cookie := signInSeed(t, pool)
	q := sqlc.New(pool)
	projectID := uuid.MustParse("00000000-0000-0000-0000-000000000101")

	before, err := q.GetProject(context.Background(), projectID)
	if err != nil {
		t.Fatalf("GetProject before: %v", err)
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/projects/00000000-0000-0000-0000-000000000101/turn", strings.NewReader(`{"user_input":"这条来源可信吗"}`))
	h.ServeHTTP(rr, withCookie(req, cookie))
	if rr.Code != 200 {
		t.Fatalf("turn: %d — %s", rr.Code, rr.Body.String())
	}

	after, err := q.GetProject(context.Background(), projectID)
	if err != nil {
		t.Fatalf("GetProject after: %v", err)
	}
	if !after.LastActiveAt.After(before.LastActiveAt) {
		t.Fatalf("last_active_at did not advance: before=%v after=%v", before.LastActiveAt, after.LastActiveAt)
	}
}
