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
// Emits a non-zero EventUsage so every live gateway.Collect call site
// (coach.go/anchors.go/course.go) has real token counts to meter — the 5d
// review CRITICAL fix this file's usage-persistence tests exercise.
func fakeProvider() gateway.Provider {
	return gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: "先说说你打算怎么把这条证据接上主张？"},
		{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 123, OutputTokens: 45}},
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

// TestProjectTurn_PersistsAnchorsUsage — 5d review CRITICAL fix: surfacing
// the craap card (Task 2's surface seam, same trigger as
// TestProjectTurn_SurfacesCraapCard_GeneratesAnchors) makes a real
// gateway.Collect call (agent/anchors.go's Generate) that must be metered.
// The old task-scoped surface recorded provider/model/tier/tokens/cost on
// the assistant `messages` row (agent/turn.go, deleted in Slice 5d); the new
// project surface has no row every LLM call maps onto, so usage gets its own
// llm_call table (migration 0019). Before this fix, nothing recorded this
// call at all — this test is RED against the pre-fix code.
func TestProjectTurn_PersistsAnchorsUsage(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, Provider: fakeProvider(), ChatResolver: fakeResolver(), SpecByID: cards.ByID}).Handler()
	cookie := signInSeed(t, pool)
	projectID := uuid.MustParse("00000000-0000-0000-0000-000000000101")

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/projects/00000000-0000-0000-0000-000000000101/turn", strings.NewReader(`{"user_input":"这条来源可信吗"}`))
	h.ServeHTTP(rr, withCookie(req, cookie))
	if rr.Code != 200 || !strings.Contains(rr.Body.String(), `"card_id":"craap"`) {
		t.Fatalf("expected a craap card event: %d — %s", rr.Code, rr.Body.String())
	}

	calls, err := sqlc.New(pool).ListLLMCallsByProject(context.Background(), pgUUID(projectID))
	if err != nil {
		t.Fatalf("ListLLMCallsByProject: %v", err)
	}
	var anchorsCall *sqlc.LlmCall
	for i, c := range calls {
		if c.Purpose == "anchors" {
			anchorsCall = &calls[i]
			break
		}
	}
	if anchorsCall == nil {
		t.Fatalf("no purpose=anchors llm_call row persisted, got %d calls: %+v", len(calls), calls)
	}
	if anchorsCall.Surface != "studio" {
		t.Fatalf("surface = %q, want studio", anchorsCall.Surface)
	}
	if anchorsCall.UserID != SeedUserID {
		t.Fatalf("user_id = %v, want %v", anchorsCall.UserID, SeedUserID)
	}
	if !anchorsCall.ProjectID.Valid || anchorsCall.ProjectID.Bytes != projectID {
		t.Fatalf("project_id not correctly recorded: %+v", anchorsCall.ProjectID)
	}
	if anchorsCall.PromptTokens == 0 && anchorsCall.CompletionTokens == 0 {
		t.Fatalf("expected non-zero token counts, got %+v", anchorsCall)
	}
}

// TestProjectTurn_PersistsCoachUsage — same CRITICAL fix, for the other live
// gateway.Collect call site: the coach's own turn (agent/coach.go's
// ProposeIntervention, driven from RunAgentStep when the top candidate is
// post_intervention rather than surface_card). The seeded demo project
// (migration 0018) plants a bare unsupported claim AND two article
// materials, so surface_card outranks post_intervention until every
// material has been surfaced once (SurfaceCard mints the
// card_instance->material "evaluates" edge immediately at surface time) —
// repeat turns until the coach path is reached, bounded well above the
// seed's material count so a regression here fails loudly instead of
// looping forever.
func TestProjectTurn_PersistsCoachUsage(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, Provider: fakeProvider(), ChatResolver: fakeResolver(), SpecByID: cards.ByID}).Handler()
	cookie := signInSeed(t, pool)
	projectID := uuid.MustParse("00000000-0000-0000-0000-000000000101")

	reachedIntervention := false
	for i := 0; i < 6; i++ {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/api/v1/projects/00000000-0000-0000-0000-000000000101/turn", strings.NewReader(`{"user_input":"这条主张要怎么支撑"}`))
		h.ServeHTTP(rr, withCookie(req, cookie))
		body := rr.Body.String()
		if rr.Code != 200 {
			t.Fatalf("turn %d: %d — %s", i, rr.Code, body)
		}
		if strings.Contains(body, "event: intervention") {
			reachedIntervention = true
			break
		}
		if !strings.Contains(body, "event: card") {
			t.Fatalf("turn %d: neither a card nor an intervention event: %s", i, body)
		}
	}
	if !reachedIntervention {
		t.Fatal("never reached the coach path after surfacing every material")
	}

	calls, err := sqlc.New(pool).ListLLMCallsByProject(context.Background(), pgUUID(projectID))
	if err != nil {
		t.Fatalf("ListLLMCallsByProject: %v", err)
	}
	var coachCall *sqlc.LlmCall
	for i, c := range calls {
		if c.Purpose == "coach" {
			coachCall = &calls[i]
			break
		}
	}
	if coachCall == nil {
		t.Fatalf("no purpose=coach llm_call row persisted, got %d calls: %+v", len(calls), calls)
	}
	if coachCall.Surface != "studio" {
		t.Fatalf("surface = %q, want studio", coachCall.Surface)
	}
	if coachCall.UserID != SeedUserID {
		t.Fatalf("user_id = %v, want %v", coachCall.UserID, SeedUserID)
	}
	if coachCall.PromptTokens == 0 && coachCall.CompletionTokens == 0 {
		t.Fatalf("expected non-zero token counts, got %+v", coachCall)
	}
	if coachCall.Provider != "deepseek" || coachCall.Model != "deepseek-chat" || coachCall.Tier != "chaperone" {
		t.Fatalf("routing not carried through from gateway.Resolved: %+v", coachCall)
	}
}

// TestProjectTurn_SchoolUsageAggregateNonEmpty is the test that proves the
// admin console stops lying: before this fix, GetSchoolUsageByTier (the
// query behind GET /api/v1/admin/overview's usage panel) always returned an
// empty slice for every school, because nothing on the live path wrote to
// llm_usage's underlying tables any more. It must fail against the pre-fix
// code and pass once a student turn records a metered llm_call row.
func TestProjectTurn_SchoolUsageAggregateNonEmpty(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, Provider: fakeProvider(), ChatResolver: fakeResolver(), SpecByID: cards.ByID}).Handler()
	cookie := signInSeed(t, pool)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/projects/00000000-0000-0000-0000-000000000101/turn", strings.NewReader(`{"user_input":"这条来源可信吗"}`))
	h.ServeHTTP(rr, withCookie(req, cookie))
	if rr.Code != 200 {
		t.Fatalf("turn: %d — %s", rr.Code, rr.Body.String())
	}

	rows, err := sqlc.New(pool).GetSchoolUsageByTier(context.Background(), SeedSchoolID)
	if err != nil {
		t.Fatalf("GetSchoolUsageByTier: %v", err)
	}
	if len(rows) == 0 {
		t.Fatal("GetSchoolUsageByTier returned no rows after a student turn — the admin usage panel would still show 暂无用量")
	}
	var totalPrompt, totalCompletion int64
	for _, r := range rows {
		totalPrompt += r.PromptTokens
		totalCompletion += r.CompletionTokens
	}
	if totalPrompt == 0 && totalCompletion == 0 {
		t.Fatalf("aggregated usage is all-zero: %+v", rows)
	}
}
