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
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

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

// fakeEvalResolver is the flagship-tier counterpart to fakeResolver, mirroring
// its shape — no real key, no network; the fake provider never reads it. Use
// this for Deps.EvalResolver in tests that exercise the growth assessor
// (assessment.go / course_assessment.go), which must resolve flagship, never
// chaperone (评估走旗舰模型绝不降级).
func fakeEvalResolver() gateway.KeyResolver {
	return func(context.Context) (gateway.Resolved, error) {
		return gateway.Resolved{Provider: "deepseek", Model: "deepseek-reasoner", Tier: "flagship", APIKey: "sk-test"}, nil
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

// TestProjectTurn_SurfacesSiftCard_AfterCraapCompleted is the whole-branch
// review's end-to-end proof for findings [1] + [3] + [5]: SIFT is UNREACHABLE
// before this fix — SurfaceCardCandidates only ever named "craap", so no
// material could ever get a SIFT card_instance no matter how the rest of the
// runtime evolved. This test drives the REAL surface -> fill -> submit path
// (mirrors projectcards_test.go's TestProjectCardSubmit_SurfaceFillMintE2E)
// to genuinely complete CRAAP on a material.
//
// N3b Seam B (Task 5) made the "card_refeed" trigger a REAL coaching
// question about the card the student just completed, and that refeed
// candidate deliberately outranks every other candidate including
// surface_card (agent/loop.go's RunAgentStep) — so the craap submit's OWN
// response is now that coaching question, not the sift surface. SIFT's
// reachability (finding [1]) still holds, just one hop later: the student's
// NEXT turn (postProjectTurn) is asserted below to surface SIFT on the same
// checked material, with:
//   - the SSE card frame's material_id naming the checked material, so the
//     client never has to guess it (finding [5]);
//   - AI-authored anchors scoped to that checked material only, and never
//     inventing a "find" (lateral) anchor — no lateral source has been
//     chosen yet at surface time (finding [3]).
func TestProjectTurn_SurfacesSiftCard_AfterCraapCompleted(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, Provider: fakeProvider(), ChatResolver: fakeResolver(), SpecByID: cards.ByID}).Handler()
	cookie := signInSeed(t, pool)
	projectID := uuid.MustParse("00000000-0000-0000-0000-000000000101")
	q := sqlc.New(pool)

	// Complete CRAAP for real over the surface->fill->submit path so the
	// graph carries a genuine evaluated-as edge, not a hand-planted fixture.
	cid, anchors := surfaceCraapAndReadAnchors(t, h, pool, cookie, projectID)
	checkedMaterialID := anchors[0].MaterialID
	for i := range anchors {
		anchors[i].Answer = "学生的判断与理由，足够长以通过校验"
	}
	anchors = append(anchors, agent.Anchor{
		ID: "risk_note", MaterialID: checkedMaterialID, Dimension: "risk_note", Author: "student",
		Answer: "它支撑我的核心数据，但只有单一来源，需交叉验证。",
	})
	anchorsJSON, err := json.Marshal(anchors)
	if err != nil {
		t.Fatalf("marshal filled anchors: %v", err)
	}
	body := `{"field_values":{},"event_trace":[{"kind":"submit","at":"2026-07-12T00:00:00Z"}],"anchors":` + string(anchorsJSON) + `}`
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+projectID.String()+"/cards/"+cid+"/submit", strings.NewReader(body)), cookie))
	submitBody := rr.Body.String()
	if rr.Code != 200 || !strings.Contains(submitBody, "event: done") {
		t.Fatalf("craap submit: %d — %s", rr.Code, submitBody)
	}
	if got, _ := q.GetCardInstance(context.Background(), uuid.MustParse(cid)); got.Status != "completed" {
		t.Fatalf("craap card status = %q, want completed (test setup invalid)", got.Status)
	}

	// The craap submit's OWN refeed (N3b Seam B, Task 5) is now a real
	// coaching question about the card the student just completed — it
	// outranks surface_card by construction, so this same response carries
	// an intervention, never a card frame.
	if !strings.Contains(submitBody, "event: intervention") {
		t.Fatalf("expected the craap submit's own refeed to be a coaching question about the just-completed card:\n%s", submitBody)
	}
	if strings.Contains(submitBody, "event: card") {
		t.Fatalf("the refeed candidate must outrank surface_card on this same submit — got a card frame too:\n%s", submitBody)
	}

	// The summon hop: the student's NEXT turn (the refeed question having
	// already been answered/is a separate moment) must surface SIFT on the
	// material that was just evaluated — the exact reachability the
	// whole-branch review found missing (finding [1]), now proved one hop
	// after the refeed instead of on the same submit.
	turnRR := httptest.NewRecorder()
	turnReq := httptest.NewRequest("POST", "/api/v1/projects/"+projectID.String()+"/turn", strings.NewReader(`{"user_input":"这条来源核查完了，接下来该怎么办？"}`))
	h.ServeHTTP(turnRR, withCookie(turnReq, cookie))
	turnBody := turnRR.Body.String()
	if turnRR.Code != 200 || !strings.Contains(turnBody, "event: card") || !strings.Contains(turnBody, `"card_id":"sift"`) {
		t.Fatalf("expected the next turn to surface a sift card: %d — %s", turnRR.Code, turnBody)
	}
	if !strings.Contains(turnBody, `"material_id":"`+checkedMaterialID+`"`) {
		t.Fatalf("card frame material_id must name the checked material %s:\n%s", checkedMaterialID, turnBody)
	}

	cis, err := q.ListCardInstancesByProject(context.Background(), pgUUID(projectID))
	if err != nil {
		t.Fatalf("ListCardInstancesByProject: %v", err)
	}
	var siftCID uuid.UUID
	found := false
	for _, ci := range cis {
		if ci.CardID == "sift" {
			siftCID = ci.ID
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("no sift card_instance persisted")
	}

	// The evaluates edge SurfaceCard mints for the sift card_instance must
	// point at the SAME checked material — the server's own source of truth
	// for "which material is this card about" (finding [5]).
	edges, err := q.ListGraphEdgesByProject(context.Background(), projectID)
	if err != nil {
		t.Fatalf("ListGraphEdgesByProject: %v", err)
	}
	edgeMaterialFound := false
	for _, e := range edges {
		if e.Type == "evaluates" && e.FromKind == "card_instance" && e.FromID == siftCID {
			if e.ToID.String() != checkedMaterialID {
				t.Fatalf("sift card_instance evaluates material %s, want the checked material %s", e.ToID, checkedMaterialID)
			}
			edgeMaterialFound = true
			break
		}
	}
	if !edgeMaterialFound {
		t.Fatalf("no evaluates edge minted for the sift card_instance")
	}

	row, err := q.GetCardInstance(context.Background(), siftCID)
	if err != nil {
		t.Fatalf("GetCardInstance(sift): %v", err)
	}
	var siftAnchors []agent.Anchor
	if err := json.Unmarshal(row.Anchors, &siftAnchors); err != nil {
		t.Fatalf("unmarshal sift anchors: %v — %s", err, row.Anchors)
	}
	if len(siftAnchors) == 0 {
		t.Fatalf("expected generated anchors on the surfaced sift card, got none")
	}
	for _, a := range siftAnchors {
		if a.Dimension == "find" {
			t.Fatalf("sift surface anchors must never author the lateral (find) dimension — no lateral source exists yet: %+v", a)
		}
		if a.MaterialID != checkedMaterialID {
			t.Fatalf("sift surface anchor material_id = %q, want the checked material %q: %+v", a.MaterialID, checkedMaterialID, a)
		}
	}
}

// craapQualifiedBlockIDPattern matches the material-qualified block label
// agent.BuildMaterialContext renders ("[<matID>:b0] text…") in the user
// message the real L1 request sends. craapAnchorGenStubProvider reads the
// live material id out of it rather than hardcoding a bare "b0": since
// Task 10, blockLookup is qualified-only (no bare-id fallback), and this
// fixture's project+material are created fresh per round with a real UUID
// unknowable ahead of time, so the reply must discover the id from the
// request instead of guessing it.
var craapQualifiedBlockIDPattern = regexp.MustCompile(`\[([^\]:]+:b0)\]`)

// craapAnchorGenStubProvider is the fixed valid-anchor-gen JSON reply used by
// TestSurfaceAnchors_FadesWithCompletedUses. It is deliberately L1-shaped
// (real block_id + quote for all five craap tags) and reused unchanged across
// all three rounds: parseAnchorGen's L1 branch resolves block_id/quote against
// the real material, while its L2 branch (agent/anchors.go) ignores
// block_id/quote entirely and reads only dimension+question — so this one
// reply is valid at both levels, and L3 never calls the model at all
// (agent/anchors.go's Generate returns before resolving a key). The shape at
// each level is therefore proven by the level argument threaded through
// Generate/parseAnchorGen, not by anything this stub says.
func craapAnchorGenStubProvider() gateway.Provider {
	return &craapAnchorGenProvider{}
}

// craapAnchorGenProvider implements gateway.Provider directly (rather than
// gateway.NewStubProvider's fixed script) so it can inspect the outgoing
// request and echo back the qualified block id the L1 prompt actually asked
// the model for.
type craapAnchorGenProvider struct{}

func (p *craapAnchorGenProvider) Stream(ctx context.Context, r gateway.Resolved, req gateway.ChatRequest) (<-chan gateway.StreamEvent, error) {
	blockID := "b0" // L2/L3 requests ignore block_id entirely; harmless default.
	for _, m := range req.Messages {
		if match := craapQualifiedBlockIDPattern.FindStringSubmatch(m.Content); match != nil {
			blockID = match[1]
			break
		}
	}
	const quote = "全球变暖导致极端天气增加，这需要认真研究其影响。"
	reply := `[
		{"block_id":"` + blockID + `","quote":"` + quote + `","dimension":"currency","question":"这段话是什么时候写的？"},
		{"block_id":"` + blockID + `","quote":"` + quote + `","dimension":"relevance","question":"这段话跟你的论点有什么关系？"},
		{"block_id":"` + blockID + `","quote":"` + quote + `","dimension":"authority","question":"这段话的作者是谁？"},
		{"block_id":"` + blockID + `","quote":"` + quote + `","dimension":"accuracy","question":"这段话准确吗？"},
		{"block_id":"` + blockID + `","quote":"` + quote + `","dimension":"purpose","question":"作者写这段话的目的是什么？"}
	]`
	return gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: reply},
		{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 123, OutputTokens: 45}},
		{Kind: gateway.EventDone, StopReason: gateway.StopStop},
	}).Stream(ctx, r, req)
}

// craapStubQuestionByDimension mirrors craapAnchorGenStubProvider's scripted
// reply, dimension -> question text. Used to pin TestSurfaceAnchors_
// FadesWithCompletedUses's L2 round to the REAL model/parse path: craap.json's
// own tag_prompts (e.g. currency's "这条信息是什么时候发布/更新的？有没有更新版？")
// are worded differently from these, so a parse or resolver failure at L2
// (which falls back to fallbackAnchors, reading spec.Params.TagPrompts
// instead) would produce a DIFFERENT question text here and the assertion
// would catch it — the identical author/blank-span shape alone cannot.
var craapStubQuestionByDimension = map[string]string{
	"currency":  "这段话是什么时候写的？",
	"relevance": "这段话跟你的论点有什么关系？",
	"authority": "这段话的作者是谁？",
	"accuracy":  "这段话准确吗？",
	"purpose":   "作者写这段话的目的是什么？",
}

// craapMaterialText is the single-paragraph pasted material ingested into
// every round's fresh project — kept identical across rounds so
// craapAnchorGenStubProvider's quote resolves against whichever project's
// material Generate is called with (the block id itself is now discovered
// from the live request, not hardcoded — see craapAnchorGenProvider).
const craapMaterialText = "全球变暖导致极端天气增加，这需要认真研究其影响。"

// ingestCraapMaterial pastes craapMaterialText into projectID, giving it one
// un-evaluated article material (Segment produces exactly block "b0").
func ingestCraapMaterial(t *testing.T, h http.Handler, cookie *http.Cookie, projectID string) {
	t.Helper()
	rec := httptest.NewRecorder()
	req := withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+projectID+"/materials",
		strings.NewReader(`{"title":"一段材料","text":"`+craapMaterialText+`","takeaway":"t","tier":"二手"}`)), cookie)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("ingestCraapMaterial: %d — %s", rec.Code, rec.Body.String())
	}
}

// seedCompletedCraapUse directly inserts one status='completed' craap
// card_instance scoped to projectID — the guidance fade's producer
// (CountCompletedCardUsesByUser, agentstore.go) counts these across every
// project the user owns, so this is the "prior completed instances" the task
// brief asks the test to seed, without the expense of driving a real
// surface->fill->submit round trip three times over.
func seedCompletedCraapUse(t *testing.T, pool *pgxpool.Pool, projectID uuid.UUID) {
	t.Helper()
	if _, err := sqlc.New(pool).CreateProjectCardInstance(context.Background(), sqlc.CreateProjectCardInstanceParams{
		ProjectID: pgUUID(projectID), CardID: "craap", Status: "completed",
	}); err != nil {
		t.Fatalf("seedCompletedCraapUse: %v", err)
	}
}

// craapAnchorsFor surfaces craap on a freshly-created, freshly-materialed
// project and returns the persisted anchors it generated.
func craapAnchorsFor(t *testing.T, h http.Handler, pool *pgxpool.Pool, cookie *http.Cookie) []agent.Anchor {
	t.Helper()
	pid := createProjectForTest(t, h, cookie)
	ingestCraapMaterial(t, h, cookie, pid)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/projects/"+pid+"/turn", strings.NewReader(`{"user_input":"这条来源可信吗"}`))
	h.ServeHTTP(rr, withCookie(req, cookie))
	body := rr.Body.String()
	if rr.Code != 200 || !strings.Contains(body, "event: card") || !strings.Contains(body, `"card_id":"craap"`) {
		t.Fatalf("expected a craap card event: %d — %s", rr.Code, body)
	}

	q := sqlc.New(pool)
	cis, err := q.ListCardInstancesByProject(context.Background(), pgUUID(uuid.MustParse(pid)))
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
	return anchors
}

// TestSurfaceAnchors_FadesWithCompletedUses is N3c Task 5's keystone test:
// the guidance fade (spec §3) must have a LIVE producer, not merely exist as
// dead code behind Task 3/4's plumbing. It drives the SAME user surfacing the
// SAME card (craap, an annotate-primitive card) through the real
// turn->surface_card->surfaceAnchors handler path three times, with 0, then
// 1, then 2 prior status='completed' craap card_instances seeded for that
// user beforehand, and asserts the PERSISTED card_instance.anchors shift from
// L1-shaped to L2-shaped to L3-shaped (design §2's table) — proving
// surfaceAnchors itself now counts uses and threads the resulting level into
// AnchorGenerator.Generate, rather than the level being reachable only by
// calling agent.GuidanceFor directly.
func TestSurfaceAnchors_FadesWithCompletedUses(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: craapAnchorGenStubProvider(), ChatResolver: fakeResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)

	// Round 1: 0 prior completed uses -> L1 (author "ai", question present,
	// span filled: block_id non-empty).
	l1 := craapAnchorsFor(t, h, pool, cookie)
	if len(l1) == 0 {
		t.Fatalf("round 1: expected generated anchors, got none")
	}
	for _, a := range l1 {
		if a.Author != "ai" {
			t.Errorf("round 1 (0 uses): anchor %+v author = %q, want \"ai\" (L1)", a, a.Author)
		}
		if a.Question == "" {
			t.Errorf("round 1 (0 uses): anchor %+v has blank question, want L1's AI-elicited question", a)
		}
		// Require BOTH a resolved block AND a non-zero-length span: a quote
		// miss makes computeOffsets return (0,0) even with BlockID set, which
		// the old "a.BlockID == '' && a.End == a.Start" check let slip through
		// as a false-valid L1 span.
		if a.BlockID == "" || a.End <= a.Start {
			t.Errorf("round 1 (0 uses): anchor %+v span is not a resolved, non-empty L1 span", a)
		}
	}

	// Seed 1 prior completed use for this same user.
	seedCompletedCraapUse(t, pool, uuid.MustParse(createProjectForTest(t, h, cookie)))

	// Round 2: 1 prior completed use -> L2 (author "student", question
	// present but span blank — she must locate it herself).
	l2 := craapAnchorsFor(t, h, pool, cookie)
	if len(l2) == 0 {
		t.Fatalf("round 2: expected generated anchors, got none")
	}
	for _, a := range l2 {
		if a.Author != "student" {
			t.Errorf("round 2 (1 use): anchor %+v author = %q, want \"student\" (L2)", a, a.Author)
		}
		if a.Question == "" {
			t.Errorf("round 2 (1 use): anchor %+v has blank question, want L2's still-AI-elicited question", a)
		}
		if !(a.BlockID == "" && a.End == a.Start) {
			t.Errorf("round 2 (1 use): anchor %+v span is filled, want L2's blank span (she locates it)", a)
		}
		// Pin the REAL model/parse path: fallbackAnchors' L2 question comes
		// from craap.json's own tag_prompts, worded differently from the stub's
		// scripted reply — a parse/resolver failure that silently fell back
		// would still pass every check above (identical author/blank-span
		// shape) but would fail THIS one.
		if want := craapStubQuestionByDimension[a.Dimension]; want != "" && a.Question != want {
			t.Errorf("round 2 (1 use): anchor %+v question = %q, want the stub's scripted L2 question %q (fell back to fallbackAnchors?)", a, a.Question, want)
		}
	}

	// Seed a 2nd prior completed use.
	seedCompletedCraapUse(t, pool, uuid.MustParse(createProjectForTest(t, h, cookie)))

	// Round 3: 2 prior completed uses -> L3 (author "student", question
	// blank — she elicits it herself too — span blank).
	l3 := craapAnchorsFor(t, h, pool, cookie)
	if len(l3) == 0 {
		t.Fatalf("round 3: expected generated anchors, got none")
	}
	for _, a := range l3 {
		if a.Author != "student" {
			t.Errorf("round 3 (2 uses): anchor %+v author = %q, want \"student\" (L3)", a, a.Author)
		}
		if a.Question != "" {
			t.Errorf("round 3 (2 uses): anchor %+v has a question %q, want L3's blank question (she elicits it)", a, a.Question)
		}
		if !(a.BlockID == "" && a.End == a.Start) {
			t.Errorf("round 3 (2 uses): anchor %+v span is filled, want L3's blank span", a)
		}
	}
}

// TestE2E_GuidanceFadeAcrossThreeCompletions is Task 10's keystone: unlike
// TestSurfaceAnchors_FadesWithCompletedUses above (which hand-seeds prior
// status='completed' card_instance rows to prove the SHAPE the fade produces
// at each level), this test proves the COUNTER genuinely moves as a
// consequence of the student's own actions. It drives the same seeded
// student's craap card through the real surface->fill->submit->mint path
// (mirroring TestProjectCardSubmit_SurfaceFillMintE2E in
// projectcards_test.go) twice in a row, on two fresh projects, and asserts
// the THIRD surfacing lands on L3 — the fade advancing purely because
// CountCompletedCardUsesByUser now counts two real completed submits, not
// because a row was planted directly.
//
// It also carries spec §5's regression fence as load-bearing: an L2 submit —
// author "student", span blank because she was never required to locate the
// sentence — must still COMPLETE (mint an evidence node). If locating were
// ever silently required by a completion predicate, this is what would catch
// it; that would be a real defect, not a test to relax.
func TestE2E_GuidanceFadeAcrossThreeCompletions(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: craapAnchorGenStubProvider(), ChatResolver: fakeResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)
	q := sqlc.New(pool)

	// surfaceCraapRound creates a fresh project + material and drives the
	// real turn endpoint so surfaceAnchors generates + persists anchors for
	// THIS student's current completed-use count (mirrors craapAnchorsFor,
	// but also returns the card_instance/project ids so the caller can
	// complete the card for real).
	surfaceCraapRound := func() (string, uuid.UUID, []agent.Anchor) {
		t.Helper()
		pid := createProjectForTest(t, h, cookie)
		ingestCraapMaterial(t, h, cookie, pid)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/api/v1/projects/"+pid+"/turn", strings.NewReader(`{"user_input":"这条来源可信吗"}`))
		h.ServeHTTP(rr, withCookie(req, cookie))
		body := rr.Body.String()
		if rr.Code != 200 || !strings.Contains(body, "event: card") || !strings.Contains(body, `"card_id":"craap"`) {
			t.Fatalf("expected a craap card event: %d — %s", rr.Code, body)
		}

		cis, err := q.ListCardInstancesByProject(context.Background(), pgUUID(uuid.MustParse(pid)))
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
			t.Fatalf("expected generated anchors, got none")
		}
		return pid, cid, anchors
	}

	// completeCraapRound fills every generated anchor's Answer — leaving
	// whatever BlockID/Quote/Question the surface produced untouched, so at
	// L2/L3 the span (and at L3 the question) stay exactly as blank as the
	// student would leave them — plus a student risk_note, then submits over
	// the real /submit endpoint. Asserts the submit completes the card and
	// mints exactly one new evidence node: the "never a wall" regression
	// fence when called on an L2 round.
	completeCraapRound := func(pid string, cid uuid.UUID, anchors []agent.Anchor) {
		t.Helper()
		for i := range anchors {
			anchors[i].Answer = "学生的判断与理由，足够长以通过校验"
		}
		anchors = append(anchors, agent.Anchor{
			ID: "risk_note", MaterialID: anchors[0].MaterialID, Dimension: "risk_note", Author: "student",
			Answer: "它支撑我的核心数据，但只有单一来源，需交叉验证。",
		})
		anchorsJSON, err := json.Marshal(anchors)
		if err != nil {
			t.Fatalf("marshal filled anchors: %v", err)
		}

		projectID := uuid.MustParse(pid)
		before, err := q.ListGraphNodesByProject(context.Background(), projectID)
		if err != nil {
			t.Fatalf("ListGraphNodesByProject (before): %v", err)
		}
		beforeIDs := make(map[uuid.UUID]bool, len(before))
		for _, n := range before {
			beforeIDs[n.ID] = true
		}

		body := `{"field_values":{},"event_trace":[{"kind":"submit","at":"2026-07-12T00:00:00Z"}],"anchors":` + string(anchorsJSON) + `}`
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+pid+"/cards/"+cid.String()+"/submit", strings.NewReader(body)), cookie))
		if rr.Code != 200 || !strings.Contains(rr.Body.String(), "event: done") {
			t.Fatalf("submit: %d — %s", rr.Code, rr.Body.String())
		}

		got, err := q.GetCardInstance(context.Background(), cid)
		if err != nil {
			t.Fatalf("GetCardInstance (after): %v", err)
		}
		if got.Status != "completed" {
			t.Fatalf("status = %q, want completed — a span-less anchor must never block completion (spec §5)", got.Status)
		}

		after, err := q.ListGraphNodesByProject(context.Background(), projectID)
		if err != nil {
			t.Fatalf("ListGraphNodesByProject (after): %v", err)
		}
		newEvidenceCount := 0
		for _, n := range after {
			if beforeIDs[n.ID] {
				continue
			}
			if n.Type == "evidence" {
				newEvidenceCount++
			}
		}
		if newEvidenceCount != 1 {
			t.Fatalf("expected exactly one new evidence node minted, got %d", newEvidenceCount)
		}
	}

	// Round 1: 0 prior completed uses -> L1. Complete it for real, over the
	// real submit endpoint, so the student now has 1 real completed use.
	pid1, cid1, l1 := surfaceCraapRound()
	for _, a := range l1 {
		if a.Author != "ai" || a.Question == "" || a.BlockID == "" || a.End <= a.Start {
			t.Fatalf("round 1 (0 completions): anchor %+v is not L1-shaped", a)
		}
	}
	completeCraapRound(pid1, cid1, l1)

	// Round 2: the NEXT surface, after ONE real completion, must be
	// L2-shaped (author student, question present, span blank) — proving the
	// fade's counter moved because of the student's own submit, not a
	// hand-seeded row.
	pid2, cid2, l2 := surfaceCraapRound()
	for _, a := range l2 {
		if a.Author != "student" {
			t.Fatalf("round 2 (1 real completion): anchor %+v author = %q, want \"student\" (L2)", a, a.Author)
		}
		if a.Question == "" {
			t.Fatalf("round 2 (1 real completion): anchor %+v has blank question, want L2's still-AI-elicited question", a)
		}
		if !(a.BlockID == "" && a.End == a.Start) {
			t.Fatalf("round 2 (1 real completion): anchor %+v span is filled, want L2's blank span", a)
		}
	}
	// The load-bearing regression fence (spec §5 / task brief): an L2
	// submit, with the span left blank exactly as it would be if the
	// student never located the sentence, must still complete/mint.
	// Locating is never a completion requirement.
	completeCraapRound(pid2, cid2, l2)

	// Round 3: the NEXT surface, after a SECOND real completion, must be
	// L3-shaped (author student, question ALSO blank — she elicits it too).
	_, _, l3 := surfaceCraapRound()
	for _, a := range l3 {
		if a.Author != "student" {
			t.Fatalf("round 3 (2 real completions): anchor %+v author = %q, want \"student\" (L3)", a, a.Author)
		}
		if a.Question != "" {
			t.Fatalf("round 3 (2 real completions): anchor %+v has a question %q, want L3's blank question", a, a.Question)
		}
		if !(a.BlockID == "" && a.End == a.Start) {
			t.Fatalf("round 3 (2 real completions): anchor %+v span is filled, want L3's blank span", a)
		}
	}
}

// ingestMaterialForTest pastes text into projectID under title, returning the
// new material's id (the /materials response DTO carries it) — unlike
// ingestCraapMaterial, which discards it, this is for tests that need to
// distinguish two materials in the same project.
func ingestMaterialForTest(t *testing.T, h http.Handler, cookie *http.Cookie, projectID, title, text string) string {
	t.Helper()
	rec := httptest.NewRecorder()
	req := withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+projectID+"/materials",
		strings.NewReader(`{"title":"`+title+`","text":"`+text+`","takeaway":"t","tier":"二手"}`)), cookie)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("ingestMaterialForTest(%q): %d — %s", title, rec.Code, rec.Body.String())
	}
	var out struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal material response: %v — %s", err, rec.Body.String())
	}
	return out.ID
}

// craapAnchorsForTwoMaterials is Finding 1's regression fixture: a fresh
// project with an OLDER material already marked "evaluated" (via a hand-
// planted card_instance -> material "evaluates" edge, status 'skipped' so it
// neither blocks SurfaceCardCandidates' project-wide proposed/active silence
// guard nor inflates the guidance-fade's CountCompletedCardUsesByUser), and a
// NEWER, un-evaluated material — reproducing the exact shape the bug needed
// to stay hidden: project-materials[0] (oldest, by created_at) is NOT the
// card's own (checked) material. The classifier skips the evaluated older
// material and proposes craap on the newer one, so this is the material the
// surfaced card actually evaluates. Returns the persisted anchors plus the
// newer material's id.
func craapAnchorsForTwoMaterials(t *testing.T, h http.Handler, pool *pgxpool.Pool, cookie *http.Cookie) ([]agent.Anchor, string) {
	t.Helper()
	q := sqlc.New(pool)
	pid := createProjectForTest(t, h, cookie)
	projectID := uuid.MustParse(pid)

	oldMatID := ingestMaterialForTest(t, h, cookie, pid, "旧材料", craapMaterialText)
	dummyCI, err := q.CreateProjectCardInstance(context.Background(), sqlc.CreateProjectCardInstanceParams{
		ProjectID: pgUUID(projectID), CardID: "craap", Status: "skipped",
	})
	if err != nil {
		t.Fatalf("seed evaluated-old-material card_instance: %v", err)
	}
	if _, err := q.InsertGraphEdge(context.Background(), sqlc.InsertGraphEdgeParams{
		ProjectID: projectID, Type: "evaluates",
		FromKind: "card_instance", FromID: dummyCI.ID,
		ToKind: "material", ToID: uuid.MustParse(oldMatID),
	}); err != nil {
		t.Fatalf("seed evaluates edge on old material: %v", err)
	}

	newMatID := ingestMaterialForTest(t, h, cookie, pid, "新材料", craapMaterialText)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/projects/"+pid+"/turn", strings.NewReader(`{"user_input":"这条来源可信吗"}`))
	h.ServeHTTP(rr, withCookie(req, cookie))
	body := rr.Body.String()
	if rr.Code != 200 || !strings.Contains(body, "event: card") || !strings.Contains(body, `"card_id":"craap"`) {
		t.Fatalf("expected a craap card event: %d — %s", rr.Code, body)
	}
	if !strings.Contains(body, `"material_id":"`+newMatID+`"`) {
		t.Fatalf("card frame material_id must be the newer, un-evaluated material %s (not the older, already-evaluated one %s):\n%s", newMatID, oldMatID, body)
	}

	cis, err := q.ListCardInstancesByProject(context.Background(), pgUUID(projectID))
	if err != nil {
		t.Fatalf("ListCardInstancesByProject: %v", err)
	}
	var cid uuid.UUID
	found := false
	for _, ci := range cis {
		if ci.CardID == "craap" && ci.ID != dummyCI.ID {
			cid = ci.ID
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("no NEW craap card_instance persisted (beyond the seeded dummy)")
	}
	row, err := q.GetCardInstance(context.Background(), cid)
	if err != nil {
		t.Fatalf("GetCardInstance: %v", err)
	}
	var anchors []agent.Anchor
	if err := json.Unmarshal(row.Anchors, &anchors); err != nil {
		t.Fatalf("unmarshal persisted anchors: %v — raw: %s", err, row.Anchors)
	}
	return anchors, newMatID
}

// TestSurfaceAnchors_L2L3PinToCardsOwnMaterial_NotOldest is Finding 1's
// regression test. TestSurfaceAnchors_FadesWithCompletedUses above could not
// catch this bug: every one of its rounds used a fresh project with exactly
// ONE material, so project-materials[0] always coincidentally WAS the
// checked material. Here each round's project carries TWO materials, with
// the card's own (checked) material deliberately the SECOND (newer) one —
// proving surfaceAnchors narrows `materials` to the checked material at
// L2/L3 (agent/anchors.go's matID = materials[0].ID lines then read the
// RIGHT material) rather than silently pinning every non-L1 anchor to
// whichever article the student pasted first.
func TestSurfaceAnchors_L2L3PinToCardsOwnMaterial_NotOldest(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: craapAnchorGenStubProvider(), ChatResolver: fakeResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)

	// 1 prior completed craap use for this user -> L2.
	seedCompletedCraapUse(t, pool, uuid.MustParse(createProjectForTest(t, h, cookie)))
	l2, newMatID2 := craapAnchorsForTwoMaterials(t, h, pool, cookie)
	if len(l2) == 0 {
		t.Fatalf("L2: expected generated anchors, got none")
	}
	for _, a := range l2 {
		if a.MaterialID != newMatID2 {
			t.Errorf("L2 anchor %+v material_id = %q, want the card's own (newer) material %q — not the project's oldest material", a, a.MaterialID, newMatID2)
		}
	}

	// A 2nd prior completed craap use -> L3.
	seedCompletedCraapUse(t, pool, uuid.MustParse(createProjectForTest(t, h, cookie)))
	l3, newMatID3 := craapAnchorsForTwoMaterials(t, h, pool, cookie)
	if len(l3) == 0 {
		t.Fatalf("L3: expected generated anchors, got none")
	}
	for _, a := range l3 {
		if a.MaterialID != newMatID3 {
			t.Errorf("L3 anchor %+v material_id = %q, want the card's own (newer) material %q — not the project's oldest material", a, a.MaterialID, newMatID3)
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

// TestSurfaceAnchorsMetersEmptyResult is the N6 review finding's regression
// pin: Task 5 (commit b498727) moved surfaceAnchors' llm_call metering
// ABOVE its empty-result early return (studioturn.go), so a real (paid) call
// that yields ZERO usable anchors must still be metered. No card in the real
// embedded registry can reach that shape through the public HTTP turn
// endpoint — craap/sift are the only two annotate/compare-primitive cards
// and both have a non-empty completion-tag vocabulary, so their
// fallbackAnchors path (agent/anchors.go) always yields at least one
// tag-anchor even when the model's reply fails to parse. This test instead
// drives surfaceAnchors directly (via the SurfaceAnchorsForTest seam in
// export_test.go) with a synthetic cards.Spec that has NEITHER params.tags
// NOR steps — the one shape for which fallbackAnchors legitimately returns a
// zero-length slice — paired with the same fakeProvider/fakeResolver this
// file's other metering tests use (a real, non-empty Resolved). It must FAIL
// (no purpose=anchors row) if the record call were moved back below the
// len(Anchors)==0 bail.
func TestSurfaceAnchorsMetersEmptyResult(t *testing.T) {
	pool := newAPITestPool(t)
	a := New(Deps{Queries: sqlc.New(pool), Pool: pool, Provider: fakeProvider(), ChatResolver: fakeResolver(), SpecByID: cards.ByID})
	h := a.Handler()
	cookie := signInSeed(t, pool)
	projectID := uuid.MustParse(createProjectForTest(t, h, cookie))

	spec := cards.Spec{ID: "test-empty-annotate", Name: "空白测试卡", Primitive: "annotate"}
	store := agent.NewSqlcAgentStore(sqlc.New(pool), pool)
	cardInstanceID := uuid.New().String() // never persisted to — the empty bail fires before SetCardInstanceAnchors

	raw, ok := a.SurfaceAnchorsForTest(context.Background(), store, projectID, spec, cardInstanceID, "")
	if ok {
		t.Fatalf("expected surfaceAnchors to bail (ok=false) on a zero-anchor generation, got raw=%s", raw)
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
		t.Fatalf("no purpose=anchors llm_call row persisted for a real call that yielded zero anchors — the metering-before-empty-bail fix regressed; got %d calls: %+v", len(calls), calls)
	}
	if anchorsCall.Provider != "deepseek" || anchorsCall.Model != "deepseek-chat" || anchorsCall.Tier != "chaperone" {
		t.Fatalf("routing not carried through from gateway.Resolved: %+v", anchorsCall)
	}
	if anchorsCall.PromptTokens == 0 && anchorsCall.CompletionTokens == 0 {
		t.Fatalf("expected non-zero token counts, got %+v", anchorsCall)
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
