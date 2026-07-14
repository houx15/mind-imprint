package api_test

// projectcards_test.go — Task 4: POST /api/v1/projects/{id}/cards/{cid}/activate
// and .../skip, the project-scoped card lifecycle endpoints that sit beside
// postProjectTurn's card surfacing (Task 5c-2). Uses the real testcontainers
// Postgres + the seeded demo project (00000000-0000-0000-0000-000000000101,
// owned by Phoebe) so loadOwnedProjectCard's membership scan exercises real
// rows, not a fixture.

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

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/store/sqlc"
)

// createProjectCardForTest inserts a card_instance scoped to the seeded demo
// project (…0101) / task (…0100) via CreateProjectCardInstance, and returns
// its id as a string for building request paths. Uses the "craap" spec
// (not "sift_craap") — Task 5's submit test needs a card whose spec
// actually declares completion predicates + graph_effects so a fully
// answered submission can mint an evidence node; "sift_craap" declares
// neither.
func createProjectCardForTest(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	q := sqlc.New(pool)
	ci, err := q.CreateProjectCardInstance(t.Context(), sqlc.CreateProjectCardInstanceParams{
		TaskID:    pgtype.UUID{Bytes: mustUUID("00000000-0000-0000-0000-000000000100"), Valid: true},
		ProjectID: pgtype.UUID{Bytes: mustUUID("00000000-0000-0000-0000-000000000101"), Valid: true},
		CardID:    "craap",
		Status:    "active",
	})
	if err != nil {
		t.Fatalf("createProjectCardForTest: %v", err)
	}
	return ci.ID.String()
}

// craapCompleteAnchors builds a fully-satisfying CRAAP []Anchor JSON: the
// five CRAAP tags (currency/relevance/authority/accuracy/purpose) each with
// a non-empty AI-authored Answer, plus a student-authored risk_note — the
// exact shape craap.json's completion predicates
// (every_tag_present + field_written_by risk_note/student) require. Every
// anchor is keyed to the seeded material 00000000-0000-0000-0000-000000000110
// (migration 0018, project …0101) so CompleteCard's checkedMaterialID +
// GraphEffects have a material to promote into an evidence node.
func craapCompleteAnchors(t *testing.T, pool *pgxpool.Pool, cid string) string {
	t.Helper()
	_ = pool // seeded material id is a stable fixture (migration 0018); no live lookup needed
	_ = cid
	materialID := "00000000-0000-0000-0000-000000000110"
	anchors := []agent.Anchor{
		{ID: "a0", MaterialID: materialID, Dimension: "currency", Author: "ai", Answer: "2024年发布，数据较新"},
		{ID: "a1", MaterialID: materialID, Dimension: "relevance", Author: "ai", Answer: "直接支持中国可持续论点"},
		{ID: "a2", MaterialID: materialID, Dimension: "authority", Author: "ai", Answer: "NASA地球观测团队发布，具备权威性"},
		{ID: "a3", MaterialID: materialID, Dimension: "accuracy", Author: "ai", Answer: "数据可在Nature Sustainability交叉核对"},
		{ID: "a4", MaterialID: materialID, Dimension: "purpose", Author: "ai", Answer: "科普告知性质，非商业推广"},
		{ID: "a5", MaterialID: materialID, Dimension: "risk_note", Author: "student", Answer: "仍需留意样本口径是否一致"},
	}
	b, err := json.Marshal(anchors)
	if err != nil {
		t.Fatalf("marshal craap anchors: %v", err)
	}
	return string(b)
}

func TestProjectCardActivateSkip(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool}).Handler()
	cookie := signInSeed(t, pool)
	cid := createProjectCardForTest(t, pool)
	base := "/api/v1/projects/00000000-0000-0000-0000-000000000101/cards/" + cid

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", base+"/activate", nil), cookie))
	if rr.Code != 204 {
		t.Fatalf("activate: %d", rr.Code)
	}

	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", base+"/skip", strings.NewReader(`{"event_trace":[{"kind":"skip","at":"2026-07-12T00:00:00Z"}]}`)), cookie))
	if rr.Code != 204 {
		t.Fatalf("skip: %d — %s", rr.Code, rr.Body.String())
	}

	// 404 for a random (non-project) card id.
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", "/api/v1/projects/00000000-0000-0000-0000-000000000101/cards/"+uuid.NewString()+"/activate", nil), cookie))
	if rr.Code != 404 {
		t.Fatalf("foreign card: %d", rr.Code)
	}
}

// TestProjectCardSubmit — Task 5: POST .../cards/{cid}/submit persists the
// filled envelope, runs CompleteCard (which mints an evidence node once the
// CRAAP completion predicates hold), and refeeds one RunAgentStep over SSE.
func TestProjectCardSubmit(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, Provider: fakeProvider(), ChatResolver: fakeResolver(), SpecByID: cardsByID()}).Handler()
	cookie := signInSeed(t, pool)
	projectID := uuid.MustParse("00000000-0000-0000-0000-000000000101")
	cid := createProjectCardForTest(t, pool) // craap on a seeded material, status active
	base := "/api/v1/projects/00000000-0000-0000-0000-000000000101/cards/" + cid

	q := sqlc.New(pool)

	// Snapshot graph_nodes BEFORE the submit — the seed migration (0018)
	// already plants a pre-existing type=evidence node in this project (the
	// 5b orphan-evidence fixture, id …0144, body {"text":...}). Asserting
	// "any evidence node exists" would pass even if CompleteCard mints
	// nothing, so we must isolate the nodes that are NEW as of this submit.
	before, err := q.ListGraphNodesByProject(context.Background(), projectID)
	if err != nil {
		t.Fatalf("ListGraphNodesByProject (before): %v", err)
	}
	beforeIDs := make(map[uuid.UUID]bool, len(before))
	for _, n := range before {
		beforeIDs[n.ID] = true
	}

	// Fully-satisfying CRAAP anchors: all 5 tags answered + a student risk_note.
	anchors := craapCompleteAnchors(t, pool, cid) // helper builds anchors keyed to the card's material
	body := `{"field_values":{"final_verdict":"存疑"},"event_trace":[{"kind":"submit","at":"2026-07-12T00:00:00Z"}],"anchors":` + anchors + `}`
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", base+"/submit", strings.NewReader(body)), cookie))
	if rr.Code != 200 || !strings.Contains(rr.Body.String(), "event: done") {
		t.Fatalf("submit: %d — %s", rr.Code, rr.Body.String())
	}
	// FIX 1: a satisfying submit's "done" frame must report card_status
	// "completed" — the client (conversation.ts submitCard) only retires its
	// local card state on this explicit signal, never on a bare "done".
	if !strings.Contains(rr.Body.String(), `"card_status":"completed"`) {
		t.Fatalf("done frame missing card_status=completed: %s", rr.Body.String())
	}
	// status completed
	got, _ := q.GetCardInstance(context.Background(), uuid.MustParse(cid))
	if got.Status != "completed" {
		t.Fatalf("status = %q", got.Status)
	}

	// A NEW evidence node was minted by this submit — not merely the
	// pre-seeded one. Delta by identity (id not present before the submit),
	// then belt-and-suspenders on body shape: GraphEffects mints
	// Body: {"source_quality": {...}} (card_effects.go), which the seed
	// node's {"text": "..."} body does not have.
	after, err := q.ListGraphNodesByProject(context.Background(), projectID)
	if err != nil {
		t.Fatalf("ListGraphNodesByProject (after): %v", err)
	}
	var newEvidence *sqlc.GraphNode
	for i, n := range after {
		if beforeIDs[n.ID] {
			continue // pre-existing node (e.g. the seed's orphan evidence fixture)
		}
		if n.Type == "evidence" {
			newEvidence = &after[i]
			break
		}
	}
	if newEvidence == nil {
		t.Fatalf("no NEW evidence node minted by submit (found %d nodes before, %d after)", len(before), len(after))
	}
	var decodedBody map[string]json.RawMessage
	if err := json.Unmarshal(newEvidence.Body, &decodedBody); err != nil {
		t.Fatalf("unmarshal minted evidence node body: %v — %s", err, newEvidence.Body)
	}
	if _, ok := decodedBody["source_quality"]; !ok {
		t.Fatalf("minted evidence node body missing source_quality (CRAAP GraphEffects shape): %s", newEvidence.Body)
	}
}

// TestProjectCardSubmit_FormPathDoesNotComplete — the re-scope gate (whole-
// branch-review, Slice 5c-2): a submit via the form path — anchors: [] as
// envelopeReducer actually produces today, since the schema field renderer
// only ever fills field_values, never anchors (that bridge is deferred to
// Slice 6's material-annotation surface) — must NOT flip the card to
// "completed" and must NOT mint a new evidence node. Before this fix,
// submitProjectCard set status="completed" unconditionally regardless of
// whether CompleteCard's predicates held, permanently marking every
// form-only CRAAP submit "done" with no evidence and no resubmit path.
func TestProjectCardSubmit_FormPathDoesNotComplete(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, Provider: fakeProvider(), ChatResolver: fakeResolver(), SpecByID: cardsByID()}).Handler()
	cookie := signInSeed(t, pool)
	projectID := uuid.MustParse("00000000-0000-0000-0000-000000000101")
	cid := createProjectCardForTest(t, pool) // craap on a seeded material, status active
	base := "/api/v1/projects/00000000-0000-0000-0000-000000000101/cards/" + cid

	q := sqlc.New(pool)

	before, err := q.ListGraphNodesByProject(context.Background(), projectID)
	if err != nil {
		t.Fatalf("ListGraphNodesByProject (before): %v", err)
	}
	beforeIDs := make(map[uuid.UUID]bool, len(before))
	for _, n := range before {
		beforeIDs[n.ID] = true
	}

	// Mirrors what envelopeReducer actually produces for a form-only CRAAP
	// fill today: no anchors at all.
	body := `{"field_values":{"final_verdict":"存疑"},"event_trace":[{"kind":"submit","at":"2026-07-12T00:00:00Z"}],"anchors":[]}`
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", base+"/submit", strings.NewReader(body)), cookie))
	if rr.Code != 200 || !strings.Contains(rr.Body.String(), "event: done") {
		t.Fatalf("submit: %d — %s", rr.Code, rr.Body.String())
	}
	// FIX 1: a non-satisfying submit's "done" frame must report card_status
	// "active" (never "completed") — this is exactly the signal
	// conversation.ts's submitCard needs to keep the card mounted instead of
	// discarding a student's in-progress answers.
	if !strings.Contains(rr.Body.String(), `"card_status":"active"`) {
		t.Fatalf("done frame missing card_status=active: %s", rr.Body.String())
	}

	// status stays active — NOT completed — so the student can resubmit.
	got, _ := q.GetCardInstance(context.Background(), uuid.MustParse(cid))
	if got.Status != "active" {
		t.Fatalf("status = %q, want active (non-satisfying submit must not complete)", got.Status)
	}

	// No NEW evidence node minted.
	after, err := q.ListGraphNodesByProject(context.Background(), projectID)
	if err != nil {
		t.Fatalf("ListGraphNodesByProject (after): %v", err)
	}
	for _, n := range after {
		if beforeIDs[n.ID] {
			continue
		}
		if n.Type == "evidence" {
			t.Fatalf("unexpected NEW evidence node minted by a non-satisfying (anchors:[]) submit: %+v", n)
		}
	}
}

// TestProjectCardSubmit_TouchesLastActiveAt — Slice 5d Task 9 (part 2b):
// submitProjectCard also drives RunAgentStep — filling a tool card IS
// student activity, same as postProjectTurn — so it must advance
// project.last_active_at too (the roster's 最近活跃 column depends on it).
// Modeled on studioturn_test.go's TestProjectTurn_TouchesLastActiveAt.
func TestProjectCardSubmit_TouchesLastActiveAt(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, Provider: fakeProvider(), ChatResolver: fakeResolver(), SpecByID: cardsByID()}).Handler()
	cookie := signInSeed(t, pool)
	q := sqlc.New(pool)
	projectID := uuid.MustParse("00000000-0000-0000-0000-000000000101")
	cid := createProjectCardForTest(t, pool) // craap on a seeded material, status active
	base := "/api/v1/projects/00000000-0000-0000-0000-000000000101/cards/" + cid

	before, err := q.GetProject(context.Background(), projectID)
	if err != nil {
		t.Fatalf("GetProject before: %v", err)
	}

	// Form-path submit (mirrors TestProjectCardSubmit_FormPathDoesNotComplete)
	// — the touch must fire regardless of whether the submit completes the
	// card.
	body := `{"field_values":{"final_verdict":"存疑"},"event_trace":[{"kind":"submit","at":"2026-07-12T00:00:00Z"}],"anchors":[]}`
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", base+"/submit", strings.NewReader(body)), cookie))
	if rr.Code != 200 || !strings.Contains(rr.Body.String(), "event: done") {
		t.Fatalf("submit: %d — %s", rr.Code, rr.Body.String())
	}

	after, err := q.GetProject(context.Background(), projectID)
	if err != nil {
		t.Fatalf("GetProject after: %v", err)
	}
	if !after.LastActiveAt.After(before.LastActiveAt) {
		t.Fatalf("last_active_at did not advance: before=%v after=%v", before.LastActiveAt, after.LastActiveAt)
	}
}

// TestProjectCardSubmit_RejectsForeignProjectMaterialAnchor is the
// whole-branch-review IMPORTANT 3 fix: validateAnchors (cards.go) only
// checks each anchor's shape, never whether its material_id actually
// belongs to the project the card is being submitted in. Without a
// server-side check, a client could cite ANOTHER project's material as its
// SIFT lateral source and CompleteCard would happily mint a "cites" edge to
// it — satisfying lateral_source_present (and S3's cross_check gate) with a
// source that isn't hers, the exact hinge ("an INDEPENDENT source that is
// HERS") the slice depends on. This builds a second project + material the
// signed-in student does not own in THIS project's scope, submits an anchor
// naming that foreign material, and asserts the request is rejected (404 —
// existence hidden, the same convention loadOwnedProject/logSourceOpen use)
// BEFORE any mutation: the card_instance must be left untouched.
func TestProjectCardSubmit_RejectsForeignProjectMaterialAnchor(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, Provider: fakeProvider(), ChatResolver: fakeResolver(), SpecByID: cardsByID()}).Handler()
	cookie := signInSeed(t, pool)
	q := sqlc.New(pool)

	// A second project + material that does NOT belong to the seeded demo
	// project (…0101) the card under test lives in.
	otherProject, err := q.CreateProject(context.Background(), sqlc.CreateProjectParams{
		UserID:        SeedUserID,
		Qualification: "EE",
		Title:         "a wholly different project",
		BoardCfgVer:   1,
	})
	if err != nil {
		t.Fatalf("CreateProject (other): %v", err)
	}
	foreignMaterial, err := q.CreateProjectMaterial(context.Background(), sqlc.CreateProjectMaterialParams{
		ProjectID: pgtype.UUID{Bytes: otherProject.ID, Valid: true},
		Kind:      "article",
		Source:    "pasted",
		Title:     "another project's own source",
		Blocks:    []byte(`[]`),
	})
	if err != nil {
		t.Fatalf("CreateProjectMaterial (foreign): %v", err)
	}

	cid := createProjectCardForTest(t, pool) // craap on the seeded demo project (…0101), status active
	base := "/api/v1/projects/00000000-0000-0000-0000-000000000101/cards/" + cid

	anchors := []agent.Anchor{
		{
			ID: "a0", MaterialID: foreignMaterial.ID.String(), BlockID: "b0",
			Dimension: "find", Author: "student",
			Question: "找一个独立来源", Answer: "另一个项目的来源——不该被允许当作横向证据",
		},
	}
	anchorsJSON, err := json.Marshal(anchors)
	if err != nil {
		t.Fatalf("marshal anchors: %v", err)
	}
	body := `{"field_values":{},"event_trace":[{"kind":"submit","at":"2026-07-12T00:00:00Z"}],"anchors":` + string(anchorsJSON) + `}`

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", base+"/submit", strings.NewReader(body)), cookie))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("want 404 (cross-project material hidden as not-found), got %d — %s", rr.Code, rr.Body.String())
	}

	// A rejected submit must not have mutated the card at all.
	got, err := q.GetCardInstance(context.Background(), uuid.MustParse(cid))
	if err != nil {
		t.Fatalf("GetCardInstance: %v", err)
	}
	if got.Status != "active" {
		t.Fatalf("status = %q, want unchanged active — a rejected submit must not mutate the card", got.Status)
	}
}

// TestProjectCardSubmit_CompleteCardErrorSurfacesAsSSEError is FIX 5 (fix-D
// review follow-up): before this fix, an error returned by agent.CompleteCard
// was only slog.Error'd and the handler fell straight through to the refeed
// as if nothing had gone wrong — the SSE stream carried no "event: error" at
// all, so the client had no way to distinguish "your submit failed" from
// "your submit is still active because it wasn't complete". This drives
// CompleteCard's own genuine error path (checkedMaterialID returns "" when
// every submitted anchor has an empty material_id, even though
// every_tag_present + field_written_by are otherwise satisfied) through the
// real HTTP submit endpoint and asserts the response actually contains an
// error event — not a silent, error-free "done".
func TestProjectCardSubmit_CompleteCardErrorSurfacesAsSSEError(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, Provider: fakeProvider(), ChatResolver: fakeResolver(), SpecByID: cardsByID()}).Handler()
	cookie := signInSeed(t, pool)
	q := sqlc.New(pool)
	cid := createProjectCardForTest(t, pool) // craap, status active
	base := "/api/v1/projects/00000000-0000-0000-0000-000000000101/cards/" + cid

	// Fully satisfies every_tag_present + field_written_by(risk_note,
	// student) — EvaluateCompletion reports complete == true — but every
	// anchor's material_id is empty, so checkedMaterialID (card_lifecycle.go)
	// can't resolve which material to promote and CompleteCard returns a
	// genuine error instead of minting.
	anchors := []agent.Anchor{
		{ID: "a0", Dimension: "currency", Author: "ai", Answer: "无归属来源"},
		{ID: "a1", Dimension: "relevance", Author: "ai", Answer: "无归属来源"},
		{ID: "a2", Dimension: "authority", Author: "ai", Answer: "无归属来源"},
		{ID: "a3", Dimension: "accuracy", Author: "ai", Answer: "无归属来源"},
		{ID: "a4", Dimension: "purpose", Author: "ai", Answer: "无归属来源"},
		{ID: "a5", Dimension: "risk_note", Author: "student", Answer: "无归属来源"},
	}
	anchorsJSON, err := json.Marshal(anchors)
	if err != nil {
		t.Fatalf("marshal anchors: %v", err)
	}
	body := `{"field_values":{},"event_trace":[{"kind":"submit","at":"2026-07-12T00:00:00Z"}],"anchors":` + string(anchorsJSON) + `}`

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", base+"/submit", strings.NewReader(body)), cookie))
	if rr.Code != 200 {
		t.Fatalf("submit: %d — %s", rr.Code, rr.Body.String())
	}
	// The load-bearing assertion: an error event must be in the stream. Before
	// FIX 5 this would fail — the handler swallowed CompleteCard's error and
	// streamed only intervention/gate/done frames, exactly as if the submit
	// had cleanly succeeded.
	if !strings.Contains(rr.Body.String(), "event: error") {
		t.Fatalf("expected an SSE error event for CompleteCard's swallowed error, got: %s", rr.Body.String())
	}
	// A card whose CompleteCard call errored must not be left completed.
	got, err := q.GetCardInstance(context.Background(), uuid.MustParse(cid))
	if err != nil {
		t.Fatalf("GetCardInstance: %v", err)
	}
	if got.Status != "active" {
		t.Fatalf("status = %q, want unchanged active", got.Status)
	}
}

// surfaceCraapAndReadAnchors drives the real HTTP turn endpoint (same trigger
// as TestProjectTurn_SurfacesCraapCard_GeneratesAnchors) so the Studio surface
// seam (Task 2) generates + persists AI anchors on a freshly-proposed craap
// card_instance, then reads those persisted anchors back. Returns the card
// instance id and its generated anchors — the ONLY legitimate source of
// anchors for the mint test below; hand-building anchors here would
// reintroduce the vacuous 5c-2 mint-test bug this task exists to regression-test.
func surfaceCraapAndReadAnchors(t *testing.T, h http.Handler, pool *pgxpool.Pool, cookie *http.Cookie, projectID uuid.UUID) (string, []agent.Anchor) {
	t.Helper()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/projects/"+projectID.String()+"/turn", strings.NewReader(`{"user_input":"这条来源可信吗"}`))
	h.ServeHTTP(rr, withCookie(req, cookie))
	body := rr.Body.String()
	if !strings.Contains(body, "event: card") || !strings.Contains(body, `"card_id":"craap"`) {
		t.Fatalf("expected a craap card event:\n%s", body)
	}

	q := sqlc.New(pool)
	cis, err := q.ListCardInstancesByProject(context.Background(), pgtype.UUID{Bytes: projectID, Valid: true})
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
		t.Fatalf("expected generated anchors on the surfaced craap card, got none")
	}
	return cid.String(), anchors
}

// TestProjectCardSubmit_SurfaceFillMintE2E is the non-vacuous keystone
// regression the 5c-2 whole-branch review demanded: 5c-2's original mint
// test hand-built a satisfying []Anchor literal, which passed regardless of
// whether the real surface path ever generated anything — it never proved
// the surface->fill->submit->mint path actually connects. This test instead
// (1) surfaces the craap card over the real turn endpoint so Task 2's
// surface seam generates + persists the 5 tag-keyed anchors (Task 1's
// generator), (2) fills every generated anchor's answer + appends a student
// risk_note anchor (the only editing this test does), (3) submits that
// envelope over the real submit endpoint, and (4) asserts a NEW evidence
// node is minted and the card completes — proving the whole chain, not a
// fixture standing in for it.
func TestProjectCardSubmit_SurfaceFillMintE2E(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, Provider: fakeProvider(), ChatResolver: fakeResolver(), SpecByID: cardsByID()}).Handler()
	cookie := signInSeed(t, pool)
	projectID := uuid.MustParse("00000000-0000-0000-0000-000000000101")
	q := sqlc.New(pool)

	cid, anchors := surfaceCraapAndReadAnchors(t, h, pool, cookie, projectID)
	base := "/api/v1/projects/" + projectID.String() + "/cards/" + cid

	// Fill every GENERATED anchor's answer, then append the student risk_note
	// anchor — the exact shape craap.json's completion predicates
	// (every_tag_present + field_written_by risk_note/student) require.
	for i := range anchors {
		anchors[i].Answer = "学生的判断与理由，足够长以通过校验"
	}
	anchors = append(anchors, agent.Anchor{
		ID: "risk_note", MaterialID: anchors[0].MaterialID, BlockID: "",
		Dimension: "risk_note", Author: "student",
		Question: "这条来源在你的论证里起什么作用？有什么风险？",
		Answer:   "它支撑我的核心数据，但只有单一来源，需交叉验证。",
	})
	anchorsJSON, err := json.Marshal(anchors)
	if err != nil {
		t.Fatalf("marshal filled anchors: %v", err)
	}

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
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", base+"/submit", strings.NewReader(body)), cookie))
	if rr.Code != 200 || !strings.Contains(rr.Body.String(), "event: done") {
		t.Fatalf("submit: %d — %s", rr.Code, rr.Body.String())
	}

	got, err := q.GetCardInstance(context.Background(), uuid.MustParse(cid))
	if err != nil {
		t.Fatalf("GetCardInstance (after): %v", err)
	}
	if got.Status != "completed" {
		t.Fatalf("status = %q, want completed", got.Status)
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
		t.Fatalf("expected exactly one new evidence node minted, before=%d after=%d new_evidence=%d", len(before), len(after), newEvidenceCount)
	}
}

// TestProjectCardSubmit_SurfaceFillMintE2E_IncompleteAnswerDoesNotMint is the
// negative twin: leaving one GENERATED dimension's answer empty (the
// risk_note is present and every other tag is filled) must NOT satisfy
// craap.json's every_tag_present predicate, so the card must stay "active"
// and no evidence node may be minted. This is also the load-bearing proof —
// it only passes because submitProjectCard actually gates completion on the
// predicates; a submit path that always completes (the pre-fix 5c-2 bug)
// would fail this assertion.
func TestProjectCardSubmit_SurfaceFillMintE2E_IncompleteAnswerDoesNotMint(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, Provider: fakeProvider(), ChatResolver: fakeResolver(), SpecByID: cardsByID()}).Handler()
	cookie := signInSeed(t, pool)
	projectID := uuid.MustParse("00000000-0000-0000-0000-000000000101")
	q := sqlc.New(pool)

	cid, anchors := surfaceCraapAndReadAnchors(t, h, pool, cookie, projectID)
	base := "/api/v1/projects/" + projectID.String() + "/cards/" + cid

	preSubmit, err := q.GetCardInstance(context.Background(), uuid.MustParse(cid))
	if err != nil {
		t.Fatalf("GetCardInstance (before): %v", err)
	}
	if preSubmit.Status == "completed" {
		t.Fatalf("card_instance already completed before submit — test setup invalid")
	}

	// Fill every generated anchor EXCEPT the first — leave it empty so
	// every_tag_present cannot hold, even though we still append a
	// well-formed student risk_note.
	for i := range anchors {
		if i == 0 {
			continue
		}
		anchors[i].Answer = "学生的判断与理由，足够长以通过校验"
	}
	anchors = append(anchors, agent.Anchor{
		ID: "risk_note", MaterialID: anchors[0].MaterialID, BlockID: "",
		Dimension: "risk_note", Author: "student",
		Question: "这条来源在你的论证里起什么作用？有什么风险？",
		Answer:   "它支撑我的核心数据，但只有单一来源，需交叉验证。",
	})
	anchorsJSON, err := json.Marshal(anchors)
	if err != nil {
		t.Fatalf("marshal filled anchors: %v", err)
	}

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
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", base+"/submit", strings.NewReader(body)), cookie))
	if rr.Code != 200 || !strings.Contains(rr.Body.String(), "event: done") {
		t.Fatalf("submit: %d — %s", rr.Code, rr.Body.String())
	}
	// FIX 1: still active, and the "done" frame's missing list must name the
	// dimension that was left empty — EvaluateCompletion's own report,
	// surfaced instead of silently dropped.
	if !strings.Contains(rr.Body.String(), `"card_status":"active"`) {
		t.Fatalf("done frame missing card_status=active: %s", rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), `"missing":[]`) {
		t.Fatalf("done frame's missing list is empty despite an unanswered dimension: %s", rr.Body.String())
	}

	got, err := q.GetCardInstance(context.Background(), uuid.MustParse(cid))
	if err != nil {
		t.Fatalf("GetCardInstance (after): %v", err)
	}
	if got.Status == "completed" || got.Status != preSubmit.Status {
		t.Fatalf("status = %q (was %q pre-submit), want unchanged and not completed (incomplete submit must not complete)", got.Status, preSubmit.Status)
	}

	after, err := q.ListGraphNodesByProject(context.Background(), projectID)
	if err != nil {
		t.Fatalf("ListGraphNodesByProject (after): %v", err)
	}
	for _, n := range after {
		if beforeIDs[n.ID] {
			continue
		}
		if n.Type == "evidence" {
			t.Fatalf("unexpected NEW evidence node minted by an incomplete submit: %+v", n)
		}
	}
}
