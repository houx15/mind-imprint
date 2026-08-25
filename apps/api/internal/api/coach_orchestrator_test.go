package api_test

// coach_orchestrator_test.go — Task 5 · POST /coach runs through the orchestrator
// agent loop. One turn whose stubbed model output emits set_status + open_tool +
// propose_note applies the directive (stage/openTool/widthTier), surfaces the note
// proposal, persists studio_state, and returns the OrchestratorReply wire shape.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

// orchestratorStubProvider replays one orchestrator JSON turn (the model's whole
// output) with a non-zero usage so the metering path runs.
func orchestratorStubProvider(jsonOut string) gateway.Provider {
	return gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: jsonOut},
		{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 30, OutputTokens: 12}},
		{Kind: gateway.EventDone, StopReason: gateway.StopStop},
	})
}

func orchestratorHandler(t *testing.T, jsonOut string) (http.Handler, *http.Cookie, *pgxpool.Pool) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: orchestratorStubProvider(jsonOut), ChatResolver: fakeResolver(), SpecByID: cards.ByID,
	}).Handler()
	return h, signInSeed(t, pool), pool
}

// TestPostCoach_ModelFailureSurfacesError — when the model reply is unparseable
// (braced-but-broken JSON, truncation, envelope drift), the coach must return a
// real 502 ai_dialogue_failed, NOT a canned "先自己说说看…" 200 that makes 印记
// look broken/stupid to the student (USER RULE 2026-08-25). The FE catches the
// 502 and shows an honest retry note; the student turn is already persisted so a
// resend self-heals.
func TestPostCoach_ModelFailureSurfacesError(t *testing.T) {
	// braced-but-broken: fails to parse AND (because it contains "{") is not
	// salvaged as prose → ProposeStatusTurn returns its parse error.
	h, cookie, pool := orchestratorHandler(t, `{"narrate": "oops`)
	setStudioStage(t, pool, seedProjectID, agent.StageProposalForming)
	base := "/api/v1/projects/" + seedProjectID

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", base+"/coach",
		strings.NewReader(`{"user_input":"我的研究问题是净影响"}`)), cookie))
	if rr.Code != http.StatusBadGateway {
		t.Fatalf("model failure should surface as 502, got %d — %s", rr.Code, rr.Body)
	}
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v — %s", err, rr.Body)
	}
	if body.Error.Code != "ai_dialogue_failed" {
		t.Fatalf("want ai_dialogue_failed, got %q — %s", body.Error.Code, rr.Body)
	}
	// Must NOT smuggle the canned coach opener into the response.
	if strings.Contains(rr.Body.String(), "先自己说说看") {
		t.Fatalf("must not fabricate a canned reply on model failure: %s", rr.Body)
	}
}

// setStudioStage puts the seed project into a started status so status-scoped
// tool filtering (studioflow) admits the tool a test exercises. Fresh projects
// default to topic_discussion (FlowTopic), which permits only propose_question;
// most tool tests need framework/proposal/essay.
func setStudioStage(t *testing.T, pool *pgxpool.Pool, projectID string, stage agent.StudioStage) {
	t.Helper()
	st := agent.DefaultStudioState()
	st.Started = true
	st.Stage = stage
	st.OpenTool = agent.ToolForming
	b, err := json.Marshal(st)
	if err != nil {
		t.Fatalf("marshal studio_state: %v", err)
	}
	if err := sqlc.New(pool).SetStudioState(context.Background(), sqlc.SetStudioStateParams{ID: mustUUID(projectID), StudioState: b}); err != nil {
		t.Fatalf("SetStudioState: %v", err)
	}
}

// TestPostCoach_FrameworkAppliesNoteDropsForeignTools — status-router contract:
// in framework, propose_note applies; set_status/open_tool are NOT framework
// tools (transitions are deterministic now), so they are filtered out and have
// NO effect — the stage does NOT jump to body_writing.
func TestPostCoach_FrameworkAppliesNoteDropsForeignTools(t *testing.T) {
	out := `{"narrate":"我把这条记进目标了。","tools":[` +
		`{"name":"set_status","args":{"stage":"body_writing"}},` +
		`{"name":"open_tool","args":{"tool":"writing","reason":"该写正文"}},` +
		`{"name":"propose_note","args":{"section":"objective","value":"净影响"}}]}`
	h, cookie, pool := orchestratorHandler(t, out)
	setStudioStage(t, pool, seedProjectID, agent.StageProposalForming) // framework
	base := "/api/v1/projects/" + seedProjectID

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", base+"/coach",
		strings.NewReader(`{"user_input":"我的研究问题是净影响"}`)), cookie))
	if rr.Code != http.StatusOK {
		t.Fatalf("coach = %d — %s", rr.Code, rr.Body)
	}

	var resp struct {
		Narrate   string `json:"narrate"`
		Directive struct {
			Stage string `json:"stage"`
		} `json:"directive"`
		Note *struct {
			Section string `json:"section"`
			Value   string `json:"value"`
		} `json:"note"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v — %s", err, rr.Body)
	}
	if resp.Note == nil || resp.Note.Section != "objective" || resp.Note.Value != "净影响" {
		t.Fatalf("note proposal wrong: %+v — %s", resp.Note, rr.Body)
	}
	// The foreign set_status/open_tool were filtered — no jump to body_writing.
	if resp.Directive.Stage == string(agent.StageBodyWriting) {
		t.Fatalf("foreign set_status must be dropped; stage jumped to body_writing — %s", rr.Body)
	}

	raw, err := sqlc.New(pool).GetStudioState(context.Background(), mustUUID(seedProjectID))
	if err != nil {
		t.Fatalf("GetStudioState: %v", err)
	}
	var st agent.StudioState
	if err := json.Unmarshal(raw, &st); err != nil {
		t.Fatalf("unmarshal studio_state: %v — %s", err, raw)
	}
	if st.Stage == agent.StageBodyWriting {
		t.Fatalf("studio_state stage must NOT have jumped to body_writing: %+v", st)
	}
}

// TestPostCoach_SummonCardOffersCardAndRecordsEvent — the orchestrator's
// summon_card tool surfaces a card chip on the reply and records a coach_proposed
// event (the card-proposal path that replaced the retired classify chip).
// perspective-matrix is proposable and not pre-seeded, so the in-flight guard
// (cardEligibleForSummon) admits it.
func TestPostCoach_SummonCardOffersCardAndRecordsEvent(t *testing.T) {
	out := `{"narrate":"这里适合停一下。","tools":[` +
		`{"name":"summon_card","args":{"card_id":"perspective-matrix","reason":"只从一个视角看这个问题","nudge_text":"要不要用视角矩阵摊开看看？"}}]}`
	h, cookie, pool := orchestratorHandler(t, out)
	setStudioStage(t, pool, seedProjectID, agent.StageProposalForming) // framework allows summon_card
	base := "/api/v1/projects/" + seedProjectID

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", base+"/coach",
		strings.NewReader(`{"user_input":"我觉得中国显然让地球更可持续了，这就是事实"}`)), cookie))
	if rr.Code != http.StatusOK {
		t.Fatalf("coach = %d — %s", rr.Code, rr.Body)
	}
	var resp struct {
		Card *struct {
			CardID    string `json:"cardId"`
			NudgeText string `json:"nudgeText"`
		} `json:"card"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v — %s", err, rr.Body)
	}
	if resp.Card == nil || resp.Card.CardID != "perspective-matrix" {
		t.Fatalf("want perspective-matrix card, got %+v — %s", resp.Card, rr.Body)
	}
	if resp.Card.NudgeText == "" {
		t.Fatalf("card must carry a nudge — %s", rr.Body)
	}
	if got := countCoachProposedEvents(t, pool, seedProjectID); got != 1 {
		t.Fatalf("coach_proposed events = %d, want 1", got)
	}
}

// TestPostCoach_SummonCardNotSummonableDropped — re-catalog: a summon_card for a
// card that isn't in this status's deck ∪ cross-cutting pool (here a
// reading-toolkit card, cda, offered in framework) is DROPPED — no card chip,
// no coach_proposed event.
func TestPostCoach_SummonCardNotSummonableDropped(t *testing.T) {
	out := `{"narrate":"这里适合停一下。","tools":[` +
		`{"name":"summon_card","args":{"card_id":"cda","reason":"话语背后的权力","nudge_text":"要不要拆一拆用词？"}}]}`
	h, cookie, pool := orchestratorHandler(t, out)
	setStudioStage(t, pool, seedProjectID, agent.StageProposalForming) // framework
	base := "/api/v1/projects/" + seedProjectID

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", base+"/coach",
		strings.NewReader(`{"user_input":"我想聊聊这个来源的用词"}`)), cookie))
	if rr.Code != http.StatusOK {
		t.Fatalf("coach = %d — %s", rr.Code, rr.Body)
	}
	var resp struct {
		Card *struct {
			CardID string `json:"cardId"`
		} `json:"card"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v — %s", err, rr.Body)
	}
	if resp.Card != nil {
		t.Fatalf("cda (reading-toolkit) must be dropped in framework, got %+v", resp.Card)
	}
	if got := countCoachProposedEvents(t, pool, seedProjectID); got != 0 {
		t.Fatalf("coach_proposed events = %d, want 0 (dropped)", got)
	}
}

// TestPostCoach_SummonCardSkippedCardNotReoffered — 铁律 2 · 不操纵: once the
// student has dismissed a card (a `skipped` instance exists), the orchestrator's
// summon_card must NOT re-offer it. The card chip is suppressed and no new
// coach_proposed event is recorded. This restores the dismiss-suppression
// coverage the deleted classify test used to provide.
func TestPostCoach_SummonCardSkippedCardNotReoffered(t *testing.T) {
	// concession is BOTH coach-proposable (dismiss endpoint) AND cross-cutting
	// (summonable in framework), so the dismiss→suppress path is exercisable.
	out := `{"narrate":"这里适合停一下。","tools":[` +
		`{"name":"summon_card","args":{"card_id":"concession","reason":"遇到相悖的证据","nudge_text":"要不要用让步段以退为进？"}}]}`
	h, cookie, pool := orchestratorHandler(t, out)
	setStudioStage(t, pool, seedProjectID, agent.StageProposalForming) // framework allows summon_card
	base := "/api/v1/projects/" + seedProjectID

	// The student dismissed this card earlier → a `skipped` instance exists.
	rrD := httptest.NewRecorder()
	h.ServeHTTP(rrD, withCookie(httptest.NewRequest("POST", base+"/cards/dismiss-proposal",
		strings.NewReader(`{"card_id":"concession"}`)), cookie))
	if rrD.Code != http.StatusNoContent {
		t.Fatalf("dismiss = %d, want 204 — %s", rrD.Code, rrD.Body)
	}

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", base+"/coach",
		strings.NewReader(`{"user_input":"我觉得中国显然让地球更可持续了，这就是事实"}`)), cookie))
	if rr.Code != http.StatusOK {
		t.Fatalf("coach = %d — %s", rr.Code, rr.Body)
	}
	var resp struct {
		Card *struct {
			CardID string `json:"cardId"`
		} `json:"card"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v — %s", err, rr.Body)
	}
	if resp.Card != nil {
		t.Fatalf("dismissed card must NOT be re-offered, got %+v — %s", resp.Card, rr.Body)
	}
	// dismiss records a coach_proposal_skipped event, but the suppressed summon
	// must record NO coach_proposed event.
	if got := countCoachProposedEvents(t, pool, seedProjectID); got != 0 {
		t.Fatalf("coach_proposed events = %d, want 0 (suppressed)", got)
	}
}

// plainReplyStubProvider replays a plain conversational reply (NOT orchestrator
// JSON) — what the legacy ProposeProjectCoachReply producer expects, since it
// just takes the model's raw text. Returns the concrete *gateway.StubProvider
// (not wrapped in the gateway.Provider interface) so a test can inspect
// .LastRequest after the call — e.g. to assert the built prompt does NOT
// duplicate the student's current turn (fix round 1).
func plainReplyStubProvider(reply string) *gateway.StubProvider {
	return gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: reply},
		{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 20, OutputTokens: 8}},
		{Kind: gateway.EventDone, StopReason: gateway.StopStop},
	})
}

// TestPostCoach_SubagentScopeUsesLegacyPath — Task 9a: a coach turn tagged with
// a SUB-AGENT scope (find_sources / reflection) must NOT be driven by the
// studio orchestrator. It takes the retained legacy per-surface path instead:
// the stub model returns a PLAIN reply string (not orchestrator tool-call
// JSON) and the response still narrates it correctly, proving the legacy
// producer (not ProposeOrchestratorTurn) ran. The turn is stored under the
// sub-agent's own surface, and studio_state is left untouched.
func TestPostCoach_SubagentScopeUsesLegacyPath(t *testing.T) {
	const reply = "先说说你觉得这个来源可信在哪？"
	const studentText = "这篇文章看起来靠谱吗"
	pool := newAPITestPool(t)
	stub := plainReplyStubProvider(reply)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: stub, ChatResolver: fakeResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)
	base := "/api/v1/projects/" + seedProjectID

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", base+"/coach",
		strings.NewReader(`{"user_input":"`+studentText+`","scope":"find_sources"}`)), cookie))
	if rr.Code != http.StatusOK {
		t.Fatalf("coach = %d — %s", rr.Code, rr.Body)
	}

	// (fix round 1) the built prompt must carry the student's current turn
	// EXACTLY ONCE. The bug this regressed: persisting the student turn to
	// Postgres BEFORE loading history meant LoadActiveCoachHistory read the
	// just-committed row back, and the handler then appended the SAME turn a
	// second time in memory — so the model saw it twice back-to-back. Inspect
	// the captured provider request (ProposeProjectCoachReply's user message
	// embeds the whole history as "- 学生：<text>" lines via
	// BuildProjectCoachContext) rather than the reply text, since the stub's
	// reply is fixed regardless of prompt content.
	if got := len(stub.LastRequest.Messages); got != 2 {
		t.Fatalf("provider request messages = %d, want 2 (system+user) — %+v", got, stub.LastRequest.Messages)
	}
	userMsg := stub.LastRequest.Messages[1]
	if userMsg.Role != gateway.RoleUser {
		t.Fatalf("messages[1].role = %q, want %q — %+v", userMsg.Role, gateway.RoleUser, stub.LastRequest.Messages)
	}
	if n := strings.Count(userMsg.Content, studentText); n != 1 {
		t.Fatalf("student turn %q appears %d times in the built prompt, want exactly 1 — prompt:\n%s", studentText, n, userMsg.Content)
	}

	var resp struct {
		Narrate   string `json:"narrate"`
		Directive struct {
			Stage string `json:"stage"`
		} `json:"directive"`
		Note            *json.RawMessage `json:"note"`
		Card            *json.RawMessage `json:"card"`
		ReviewRequested bool             `json:"reviewRequested"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v — %s", err, rr.Body)
	}

	// (a) narrate is the stub's plain reply — proves the LEGACY producer ran
	// (an orchestrator JSON blob would never come back verbatim as narrate).
	if resp.Narrate != reply {
		t.Fatalf("narrate = %q, want %q — %s", resp.Narrate, reply, rr.Body)
	}
	// (b) directive is the unchanged default studio_state.
	if resp.Directive.Stage != string(agent.StageTopicDiscussion) {
		t.Fatalf("directive.stage = %q, want default %q — %s", resp.Directive.Stage, agent.StageTopicDiscussion, rr.Body)
	}
	if resp.Note != nil || resp.Card != nil || resp.ReviewRequested {
		t.Fatalf("sub-agent reply must never carry note/card/reviewRequested — %s", rr.Body)
	}

	// (c) the turn was stored under surface "find_sources", not "studio".
	surface := "find_sources"
	rows, err := sqlc.New(pool).ListChatMessagesByProjectSurface(context.Background(), sqlc.ListChatMessagesByProjectSurfaceParams{
		SeededProjectID: pgtype.UUID{Bytes: mustUUID(seedProjectID), Valid: true},
		Surface:         &surface,
	})
	if err != nil {
		t.Fatalf("ListChatMessagesByProjectSurface: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("find_sources surface turns = %d, want 2 (student+assistant) — %+v", len(rows), rows)
	}
	var sawUser, sawAssistant bool
	for _, m := range rows {
		if m.Role == "user" && m.Content == "这篇文章看起来靠谱吗" {
			sawUser = true
		}
		if m.Role == "assistant" && m.Content == reply {
			sawAssistant = true
		}
	}
	if !sawUser || !sawAssistant {
		t.Fatalf("find_sources surface missing expected turns: %+v", rows)
	}

	// studio surface must stay untouched by this sub-agent turn.
	studioSurface := "studio"
	studioRows, err := sqlc.New(pool).ListChatMessagesByProjectSurface(context.Background(), sqlc.ListChatMessagesByProjectSurfaceParams{
		SeededProjectID: pgtype.UUID{Bytes: mustUUID(seedProjectID), Valid: true},
		Surface:         &studioSurface,
	})
	if err != nil {
		t.Fatalf("ListChatMessagesByProjectSurface(studio): %v", err)
	}
	if len(studioRows) != 0 {
		t.Fatalf("studio surface must stay empty, got %d rows — %+v", len(studioRows), studioRows)
	}

	// (d) studio_state was NOT changed: the `project` table's studio_state column
	// carries a DB-level default (migration 0057), so it's always readable — a
	// sub-agent turn must never call SetStudioState, so it stays exactly that
	// untouched default (stage topic_discussion, updatedAtTurn 0).
	raw, gerr := sqlc.New(pool).GetStudioState(context.Background(), mustUUID(seedProjectID))
	if gerr != nil {
		t.Fatalf("GetStudioState: %v", gerr)
	}
	var st agent.StudioState
	if err := json.Unmarshal(raw, &st); err != nil {
		t.Fatalf("unmarshal studio_state: %v — %s", err, raw)
	}
	if def := agent.DefaultStudioState(); st.Stage != def.Stage || st.UpdatedAtTurn != def.UpdatedAtTurn {
		t.Fatalf("studio_state was mutated by a sub-agent turn: %+v (default %+v)", st, def)
	}
}

// TestPostCoach_CurateReferenceDropsUnknownIDKeepsRealID — P3 Task 1: the
// orchestrator's filterCurateReferenceCall only validates `kind`; the model
// can still hallucinate an `id` that was never in the projection. coach.go's
// curate_reference apply case (filterKnownReferences) must query the
// project's real reference ids and drop any curated item whose id isn't one
// of them — a hallucinated id must never reach studio_state.reference, which
// a later task's frontend resolves into rich content.
func TestPostCoach_CurateReferenceDropsUnknownIDKeepsRealID(t *testing.T) {
	pool := newAPITestPool(t)
	q := sqlc.New(pool)
	projectID := mustUUID(seedProjectID)

	ref, err := q.CreateReference(context.Background(), sqlc.CreateReferenceParams{
		ProjectID: projectID, Title: "NASA 气候数据", Tags: []byte("[]"), SearchHints: []byte("[]"),
	})
	if err != nil {
		t.Fatalf("CreateReference: %v", err)
	}
	bogusID := uuid.New().String()

	out := `{"narrate":"把这条来源摆到侧栏了。","tools":[` +
		`{"name":"curate_reference","args":{"items":[` +
		`{"kind":"material","id":"` + ref.ID.String() + `","label":"NASA 气候数据"},` +
		`{"kind":"material","id":"` + bogusID + `","label":"编造的来源"}` +
		`]}}]}`
	h := New(Deps{
		Queries: q, Pool: pool,
		Provider: orchestratorStubProvider(out), ChatResolver: fakeResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)
	setStudioStage(t, pool, seedProjectID, agent.StageProposalWriting) // proposal allows curate_reference
	base := "/api/v1/projects/" + seedProjectID

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", base+"/coach",
		strings.NewReader(`{"user_input":"这条来源很关键"}`)), cookie))
	if rr.Code != http.StatusOK {
		t.Fatalf("coach = %d — %s", rr.Code, rr.Body)
	}

	raw, gerr := q.GetStudioState(context.Background(), projectID)
	if gerr != nil {
		t.Fatalf("GetStudioState: %v", gerr)
	}
	var st agent.StudioState
	if err := json.Unmarshal(raw, &st); err != nil {
		t.Fatalf("unmarshal studio_state: %v — %s", err, raw)
	}
	if len(st.Reference) != 1 {
		t.Fatalf("studio_state.reference = %+v, want exactly 1 (the real id)", st.Reference)
	}
	if st.Reference[0].ID != ref.ID.String() {
		t.Fatalf("studio_state.reference[0].ID = %q, want %q", st.Reference[0].ID, ref.ID.String())
	}
	for _, item := range st.Reference {
		if item.ID == bogusID {
			t.Fatalf("hallucinated id %q leaked into studio_state.reference: %+v", bogusID, st.Reference)
		}
	}
}

// TestPostCoach_CurateReferenceAnnotationKeepsRealReviewItemID — Task 2
// (annotation entity): filterKnownReferences must also accept the third
// curate_reference kind, "annotation" — its allowed-set now includes real
// review_item intervention ids (整稿体检 results), so a curated annotation id
// that matches one survives while a hallucinated one is still dropped, same
// as TestPostCoach_CurateReferenceDropsUnknownIDKeepsRealID above for
// kind="material".
func TestPostCoach_CurateReferenceAnnotationKeepsRealReviewItemID(t *testing.T) {
	pool := newAPITestPool(t)
	q := sqlc.New(pool)
	projectID := mustUUID(seedProjectID)
	cookie := signInSeed(t, pool)
	base := "/api/v1/projects/" + seedProjectID

	// Seed a real review_item intervention — same commit-snapshot +
	// order-review flow as annotations_test.go's
	// TestAnnotationsEndpoint_ListsPersistedReviewItems.
	reviewReply := `[{"criterion_code":"表E","band":"5–6 段","evidence":"第2段接住反方","missing":"跳步没补","fix":"补上定义"}]`
	reviewH := New(Deps{
		Queries:      q,
		Pool:         pool,
		Provider:     reviewStubProvider(reviewReply),
		ChatResolver: fakeResolver(),
	}).Handler()

	content := strings.Repeat("字", 1600)
	recSnap := httptest.NewRecorder()
	reviewH.ServeHTTP(recSnap, withCookie(httptest.NewRequest("POST", base+"/snapshots",
		strings.NewReader(`{"content":"`+content+`"}`)), cookie))
	if recSnap.Code != http.StatusCreated {
		t.Fatalf("commit snapshot = %d, want 201; body=%s", recSnap.Code, recSnap.Body)
	}
	var snap struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(recSnap.Body.Bytes(), &snap); err != nil {
		t.Fatalf("decode snapshot: %v", err)
	}

	recReview := httptest.NewRecorder()
	reviewH.ServeHTTP(recReview, withCookie(httptest.NewRequest("POST", base+"/snapshots/"+snap.ID+"/review",
		strings.NewReader("")), cookie))
	if recReview.Code != http.StatusOK {
		t.Fatalf("order review = %d, want 200; body=%s", recReview.Code, recReview.Body)
	}

	items, err := q.ListReviewItemsByProject(context.Background(), projectID)
	if err != nil {
		t.Fatalf("ListReviewItemsByProject: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("ListReviewItemsByProject returned 0 rows after ordering a review")
	}
	realID := items[0].ID.String()
	bogusID := uuid.New().String()

	out := `{"narrate":"把这条批注记下了。","tools":[` +
		`{"name":"curate_reference","args":{"items":[` +
		`{"kind":"annotation","id":"` + realID + `","label":"表E 反方跳步"},` +
		`{"kind":"annotation","id":"` + bogusID + `","label":"编造的批注"}` +
		`]}}]}`
	coachH := New(Deps{
		Queries: q, Pool: pool,
		Provider: orchestratorStubProvider(out), ChatResolver: fakeResolver(), SpecByID: cards.ByID,
	}).Handler()
	setStudioStage(t, pool, seedProjectID, agent.StageProposalWriting) // proposal allows curate_reference

	rr := httptest.NewRecorder()
	coachH.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", base+"/coach",
		strings.NewReader(`{"user_input":"这条批注要记下来"}`)), cookie))
	if rr.Code != http.StatusOK {
		t.Fatalf("coach = %d — %s", rr.Code, rr.Body)
	}

	raw, gerr := q.GetStudioState(context.Background(), projectID)
	if gerr != nil {
		t.Fatalf("GetStudioState: %v", gerr)
	}
	var st agent.StudioState
	if err := json.Unmarshal(raw, &st); err != nil {
		t.Fatalf("unmarshal studio_state: %v — %s", err, raw)
	}
	if len(st.Reference) != 1 {
		t.Fatalf("studio_state.reference = %+v, want exactly 1 (the real annotation id)", st.Reference)
	}
	if st.Reference[0].ID != realID {
		t.Fatalf("studio_state.reference[0].ID = %q, want %q", st.Reference[0].ID, realID)
	}
	if st.Reference[0].Kind != "annotation" {
		t.Fatalf("studio_state.reference[0].Kind = %q, want %q", st.Reference[0].Kind, "annotation")
	}
	for _, item := range st.Reference {
		if item.ID == bogusID {
			t.Fatalf("hallucinated annotation id %q leaked into studio_state.reference: %+v", bogusID, st.Reference)
		}
	}
}
