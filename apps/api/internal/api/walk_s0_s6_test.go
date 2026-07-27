package api_test

// walk_s0_s6_test.go — N3f Task 12: the slice's acceptance walk. Extends
// walkable_stations_test.go's N3d proof (S0->S3) all the way to S6, over a
// FRESH project (POST /api/v1/projects — never the seeded demo project,
// whose migration 0018 hand-writes confirmed_solid gate rows and would prove
// nothing about live code paths), driving ONLY endpoints a real screen
// control calls. Every endpoint used here has a confirmed caller in
// apps/web/src (see the comment above each call below) — the discipline the
// brief demands after N3d's acceptance test proved a station walkable by
// POSTing /materials/{mid}/open-shaped endpoints no UI state could produce.
//
// Station-by-station endpoint -> web caller map (see task-12-report.md for
// the full table):
//   S0 decode_task        POST /onboarding          -> api/projects.ts submitOnboarding
//   S1 frame_question     POST /framing             -> api/projects.ts submitFraming
//   S2 evaluate_perspectives
//                         POST /materials           -> api/materials.ts ingestMaterial
//                         POST /materials/{id}/open -> api/materials.ts logSourceOpen
//                         POST /perspectives        -> api/projects.ts submitPerspectives
//                         POST /gate/.../attest     -> api/writing.ts attestGate
//   S3 evaluate_sources   POST /turn                -> api/studioTurn.ts studioTurn
//                         POST /cards/{cid}/activate-> api/projectCards.ts activateProjectCard
//                         POST /cards/{cid}/submit  -> api/projectCards.ts submitProjectCard
//                         POST /contracts/.../spot-check
//                                                    -> api/writing.ts orderSpotCheck
//   S4 build_argument     (same turn/activate/submit/spot-check callers, on
//                          the toulmin card and the build_argument station)
//   S5 draft_polish       POST /snapshots           -> api/writing.ts commitSnapshot
//                         POST /gate/.../attest     -> api/writing.ts attestGate
//                         POST /snapshots/{id}/review -> api/writing.ts orderReview
//   S6 reflect_archive    POST /reflection          -> api/projects.ts submitReflection
//                         POST /declaration/sign    -> api/writing.ts signDeclaration
//
// Every station boundary is asserted via GET /api/v1/projects/{id}'s own
// stations[].state (fetchStations/stationState, walkable_stations_test.go) —
// never gate internals — so a regression names the station it broke at.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"mindimprint/api/internal/agent"
	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/store/sqlc"
)

// walkMaterialA/B are the two source texts the walk ingests at S2 — distinct
// content so the reader can tell them apart in failure output; thematically
// consistent with the PRD's Phoebe/"中国是否让地球变得更可持续" scenario.
const (
	walkMaterialTextA = "NASA的卫星数据显示，2000年到2017年间全球叶面积指数上升了约5%，中国和印度贡献了净增量的三分之一。这项研究发表在《自然可持续发展》期刊上。"
	walkMaterialTextB = "一份独立环境智库报告指出，尽管中国在造林和可再生能源方面成绩显著，但其碳排放总量仍居全球第一，绿化增量不能完全抵消排放增长带来的净影响。"
)

// surfaceWalkCard drives the real turn endpoint (api/studioTurn.ts's
// studioTurn, the only client caller of POST /turn) and asserts it surfaced
// wantCardID, then reads the fresh card_instance back (status "proposed" —
// the classifier's in-flight guard means at most one card_instance is
// proposed/active project-wide, so this is unambiguous) plus whichever
// material its own "evaluates" edge names (empty for a project-scoped card
// like toulmin) — the same graph fact studio/projection.go's
// materialByCardInstance reads, not a guess from array position.
func surfaceWalkCard(t *testing.T, h http.Handler, pool *pgxpool.Pool, cookie *http.Cookie, projectID, wantCardID, prompt string) (cid, materialID string, anchors []agent.Anchor) {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/projects/"+projectID+"/turn",
		strings.NewReader(`{"user_input":"`+prompt+`"}`))
	h.ServeHTTP(rec, withCookie(req, cookie))
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, "event: card") || !strings.Contains(body, `"card_id":"`+wantCardID+`"`) {
		t.Fatalf("turn expected to surface %q card: %d — %s", wantCardID, rec.Code, body)
	}

	q := sqlc.New(pool)
	cis, err := q.ListCardInstancesByProject(context.Background(), pgUUID(mustUUID(projectID)))
	if err != nil {
		t.Fatalf("ListCardInstancesByProject: %v", err)
	}
	found := false
	var row sqlc.CardInstance
	for _, ci := range cis {
		if ci.CardID == wantCardID && ci.Status == "proposed" {
			row, found = ci, true
			break
		}
	}
	if !found {
		t.Fatalf("no proposed %q card_instance found after turn", wantCardID)
	}

	edges, err := q.ListGraphEdgesByProject(context.Background(), mustUUID(projectID))
	if err != nil {
		t.Fatalf("ListGraphEdgesByProject: %v", err)
	}
	for _, e := range edges {
		if e.Type == "evaluates" && e.FromKind == "card_instance" && e.FromID == row.ID && e.ToKind == "material" {
			materialID = e.ToID.String()
			break
		}
	}
	if len(row.Anchors) > 0 {
		_ = json.Unmarshal(row.Anchors, &anchors)
	}
	return row.ID.String(), materialID, anchors
}

// surfaceReadingCard drives prepareSourceAnnotation (POST
// .../materials/{mid}/annotate) — the reading room's own summon path
// (materials.go). Task 11 (spec-read-together-redesign) retired CRAAP/SIFT
// from /turn's candidates ("reading room ONLY"), so unlike surfaceWalkCard
// above (which lets the classifier pick a material via /turn), the walk now
// picks the material explicitly — mirroring the real flow: the student opens
// ONE specific source to read it, and craap/sift surfaces on THAT source, not
// wherever the classifier happens to look first.
func surfaceReadingCard(t *testing.T, h http.Handler, pool *pgxpool.Pool, cookie *http.Cookie, projectID, wantCardID, materialID string) (cid string, anchors []agent.Anchor) {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/projects/"+projectID+"/materials/"+materialID+"/annotate", nil)
	h.ServeHTTP(rec, withCookie(req, cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("prepareSourceAnnotation(%s): %d — %s", materialID, rec.Code, rec.Body.String())
	}

	q := sqlc.New(pool)
	cis, err := q.ListCardInstancesByProject(context.Background(), pgUUID(mustUUID(projectID)))
	if err != nil {
		t.Fatalf("ListCardInstancesByProject: %v", err)
	}
	found := false
	var row sqlc.CardInstance
	for _, ci := range cis {
		if ci.CardID == wantCardID && ci.Status == "proposed" {
			row, found = ci, true
			break
		}
	}
	if !found {
		t.Fatalf("no proposed %q card_instance found after opening material %s for annotation", wantCardID, materialID)
	}
	if len(row.Anchors) > 0 {
		_ = json.Unmarshal(row.Anchors, &anchors)
	}
	return row.ID.String(), anchors
}

// activateWalkCard drives POST /cards/{cid}/activate (api/projectCards.ts's
// activateProjectCard) — the real screen control confirming "打开" (AGENTS.md
// rule 2: summoning is automatic, opening is the student's own confirmed
// step) before a card is filled.
func activateWalkCard(t *testing.T, h http.Handler, cookie *http.Cookie, projectID, cid string) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST",
		"/api/v1/projects/"+projectID+"/cards/"+cid+"/activate", nil), cookie))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("activate card %s: %d — %s", cid, rec.Code, rec.Body.String())
	}
}

// submitWalkCard drives POST /cards/{cid}/submit (api/projectCards.ts's
// submitProjectCard) with a filled anchor envelope. The SSE body is only
// checked for the "event: done" frame here — no caller needs the raw body
// back (assertCardCompleted below is the real proof of completion, read
// straight off the card_instance row), so this returns nothing.
func submitWalkCard(t *testing.T, h http.Handler, cookie *http.Cookie, projectID, cid string, anchors []agent.Anchor) {
	t.Helper()
	anchorsJSON, err := json.Marshal(anchors)
	if err != nil {
		t.Fatalf("marshal anchors: %v", err)
	}
	body := `{"field_values":{},"event_trace":[{"kind":"submit","at":"2026-07-22T00:00:00Z"}],"anchors":` + string(anchorsJSON) + `}`
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST",
		"/api/v1/projects/"+projectID+"/cards/"+cid+"/submit", strings.NewReader(body)), cookie))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "event: done") {
		t.Fatalf("submit card %s: %d — %s", cid, rec.Code, rec.Body.String())
	}
}

// assertCardCompleted confirms the card_instance actually completed — the
// real proof its completion predicates were satisfied, not just that submit
// returned 200 (an incomplete submit also returns 200 with cardStatus
// "active", per projectcards.go's own doc comment).
func assertCardCompleted(t *testing.T, pool *pgxpool.Pool, cid string) {
	t.Helper()
	row, err := sqlc.New(pool).GetCardInstance(context.Background(), mustUUID(cid))
	if err != nil {
		t.Fatalf("GetCardInstance(%s): %v", cid, err)
	}
	if row.Status != "completed" {
		t.Fatalf("card %s status = %q, want completed", cid, row.Status)
	}
}

// spotCheckOrderable reads spotChecks.<which>.orderable straight off the
// projection (studio/dto.go's SpotChecksDTO, GET /api/v1/projects/{id}) — the
// exact boolean SpotCheckPanel.tsx's button gates on
// (`disabled={pending || !data.orderable}`). Asserting this before ordering
// proves the 信源体检/论证体检 button itself would have been pressable at
// this point in the walk, not merely that the endpoint accepts the call.
func spotCheckOrderable(t *testing.T, h http.Handler, pid string, cookie *http.Cookie, which string) bool {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/projects/"+pid, nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET project = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var out struct {
		SpotChecks map[string]struct {
			Orderable bool `json:"orderable"`
		} `json:"spotChecks"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal projection: %v; raw=%s", err, rec.Body.Bytes())
	}
	v, ok := out.SpotChecks[which]
	if !ok {
		t.Fatalf("projection spotChecks has no key %q; raw=%s", which, rec.Body.Bytes())
	}
	return v.Orderable
}

// fillCraapAnchors fills every generated CRAAP tag anchor's answer, then
// appends the student-authored risk_note anchor — the exact shape craap.json's
// completion predicates (every_tag_present + field_written_by risk_note/
// student) require (mirrors projectcards_test.go's
// TestProjectCardSubmit_SurfaceFillMintE2E, generalized to any checked
// material).
func fillCraapAnchors(anchors []agent.Anchor, checkedMaterialID string) []agent.Anchor {
	out := make([]agent.Anchor, len(anchors))
	copy(out, anchors)
	for i := range out {
		out[i].Answer = "学生的判断与理由，足够长以通过校验"
	}
	return append(out, agent.Anchor{
		ID: "risk_note", MaterialID: checkedMaterialID, Dimension: "risk_note", Author: "student",
		Answer: "它支撑我的核心数据，但只有单一来源，需交叉验证。",
	})
}

// fillSiftAnchors rebuilds the EXACT envelope StudioCompareCard.buildAnchors
// emits (apps/web/src/studio/StudioCompareCard.tsx) — not the AI-generated
// anchors surfaceAnchors produced (the real client discards those entirely;
// buildAnchors is built from spec.steps' fields only). One anchor per
// answerable field (sift.json's stop/investigate/find/relation/trace_origin/
// tier_after/revised_judgment — the textarea+single_choice fields across all
// four steps), id == field key, author "student" on every one, material_id
// set on EVERY field (never blank): the field whose key equals
// params.lateral_dimension ("find") carries lateralMaterialID, every other
// field carries checkedMaterialID — exactly buildAnchors' isLateral branch.
// This is also what makes agent.checkedMaterialID resolve correctly (it
// picks the first non-"find" anchor with a material_id) and what satisfies
// lateral_source_present (find's material_id != checked) and
// field_written_by(trace_origin, student).
func fillSiftAnchors(checkedMaterialID, lateralMaterialID string) []agent.Anchor {
	return []agent.Anchor{
		{ID: "stop", MaterialID: checkedMaterialID, Dimension: "stop", Author: "student",
			Answer: "我的第一反应是想立刻拿它当论据，但还没核过来源就下判断，先停一下。"},
		{ID: "investigate", MaterialID: checkedMaterialID, Dimension: "investigate", Author: "student",
			Answer: "这是一份公开的研究/报告，署名机构可查，不是匿名营销号。"},
		{ID: "find", MaterialID: lateralMaterialID, Dimension: "find", Author: "student",
			Answer: "独立报告印证了造林规模，但强调总排放量仍是世界第一，属于限定关系。"},
		{ID: "relation", MaterialID: checkedMaterialID, Dimension: "relation", Author: "student",
			Answer: "限定"},
		{ID: "trace_origin", MaterialID: checkedMaterialID, Dimension: "trace_origin", Author: "student",
			Answer: "追到NASA地球观测团队发布的原始数据集，不是二手转述。"},
		{ID: "tier_after", MaterialID: checkedMaterialID, Dimension: "tier_after", Author: "student",
			Answer: "一手报道"},
		{ID: "revised_judgment", MaterialID: checkedMaterialID, Dimension: "revised_judgment", Author: "student",
			Answer: "横向查过之后，我把结论收紧为：绿化在扩大，但不能反驳排放总量仍居第一这一事实。"},
	}
}

// toulminAnchors mirrors graphStateToAnchors (apps/web/src/primitives/graph/
// serialize.ts) — the ONLY function that ever turns a Toulmin GraphState into
// the wire []Anchor StudioToulminCard.handleLock submits. It emits SPLIT
// anchors: one text anchor per node (material_id "", answer = the sentence,
// dimension = the node's type = the slot id), plus one SOURCE anchor per
// `cites` edge (material_id = the cited material, answer "", dimension = the
// citing node's type) — never both material_id and answer on the same
// anchor, the shape the real serializer can never produce. slotComplete
// (card_completion.go) ORs both anchors by matching Dimension, so this
// satisfies it exactly the way graphStateToAnchors' real output would: text
// anchors give the ≥12-rune sentence, source anchors on the needSrc slots
// (warrant/evidence/concession, per toulmin.json) give the material_id.
func toulminAnchors(sourceMaterialID string) []agent.Anchor {
	text := func(id, answer string) agent.Anchor {
		return agent.Anchor{ID: id, MaterialID: "", Dimension: id, Author: "student", Answer: answer}
	}
	src := func(id string) agent.Anchor {
		return agent.Anchor{ID: id + "_src", MaterialID: sourceMaterialID, Dimension: id, Author: "student", Answer: ""}
	}
	return []agent.Anchor{
		text("claim", "中国的发展路径在全球可持续叙事里兼具真实贡献与显著代价"),
		text("warrant", "卫星测得的绿化增量能被视为投入的直接证据，但不能替代对排放总量的核算"),
		src("warrant"),
		text("evidence", "NASA数据显示中国和印度贡献了全球净增绿化量的三分之一"),
		src("evidence"),
		text("counter", "反方最强论点：中国碳排放总量仍是全球第一，绿化增量抵不过排放增速"),
		text("concession", "承认排放总量确实全球第一，但人均排放与减排速度的边际改善同样值得计入判断"),
		src("concession"),
	}
}

// TestWalk_S0ToS6_FreshProject is the slice's acceptance test (task-12-brief.md):
// a project created through the real funnel (never the seeded demo project)
// walks every station S0->S6 using only endpoints a real screen control
// calls, asserted station-by-station off the projection's own rail.
func TestWalk_S0ToS6_FreshProject(t *testing.T) {
	pool := newAPITestPool(t)
	cookie := signInSeed(t, pool)

	// hCore drives every step whose model call (if any) degrades gracefully
	// to a deterministic fallback on a non-JSON reply (anchor generation,
	// coach refeed) — fakeProvider's plain-text script, exactly as
	// projectcards_test.go/studioturn_test.go already rely on. The
	// spot-check/review steps below need a DIFFERENT provider (they require
	// a real JSON reply, no fallback) and get their own handler.
	hCore := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: fakeProvider(), ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(),
		SpecByID: cards.ByID,
	}).Handler()

	// --- create: POST /api/v1/projects -----------------------------------
	pid := createProjectForTest(t, hCore, cookie)

	snap := fetchStations(t, hCore, pid, cookie)
	if got := stationState(t, snap, "S0"); got != "current" {
		t.Fatalf("S0 state after create = %q, want current", got)
	}

	// --- S0 decode_task: POST /onboarding ---------------------------------
	rec := httptest.NewRecorder()
	hCore.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+pid+"/onboarding",
		strings.NewReader(`{"restate":"这道题在问中国是否让地球变得更可持续","weakPicks":[0,2]}`)), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("submit onboarding = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	snap = fetchStations(t, hCore, pid, cookie)
	if got := stationState(t, snap, "S0"); got != "done" {
		t.Fatalf("S0 state after onboarding submit = %q, want done", got)
	}
	if got := stationState(t, snap, "S1"); got != "current" {
		t.Fatalf("S1 state after onboarding submit = %q, want current", got)
	}

	// --- S1 frame_question: POST /framing ---------------------------------
	framingBody := `{
		"terms":[
			{"term":"可持续发展","definition":"资源使用不损害后代人满足自身需求的能力这是环境层面的定义"},
			{"term":"中国的角色","definition":"中国的政策与产出对全球环境指标造成的净影响这是国家层面的定义"},
			{"term":"世界","definition":"全球尺度而非仅中国境内的地理与生态范围这是空间层面的定义"}
		],
		"answers":["中国的可再生能源投入使全球减排速度整体加快"],
		"searchPlan":["官方一手排放与能源数据来源"]
	}`
	rec = httptest.NewRecorder()
	hCore.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+pid+"/framing",
		strings.NewReader(framingBody)), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("submit framing = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	snap = fetchStations(t, hCore, pid, cookie)
	if got := stationState(t, snap, "S1"); got != "done" {
		t.Fatalf("S1 state after framing submit = %q, want done", got)
	}
	if got := stationState(t, snap, "S2"); got != "current" {
		t.Fatalf("S2 state after framing submit = %q, want current", got)
	}

	// --- S2 evaluate_perspectives: POST /materials (x2) + /open + ---------
	//     POST /perspectives + POST /gate/evaluate_perspectives/attest ------
	matA := ingestMaterialForTest(t, hCore, cookie, pid, "NASA 绿化研究", walkMaterialTextA)
	if rec := openMaterial(t, hCore, cookie, pid, matA, `{"time_spent_s":30}`); rec.Code != http.StatusNoContent {
		t.Fatalf("open material A = %d, want 204; body=%s", rec.Code, rec.Body)
	}
	matB := ingestMaterialForTest(t, hCore, cookie, pid, "独立环境智库报告", walkMaterialTextB)
	if rec := openMaterial(t, hCore, cookie, pid, matB, `{"time_spent_s":20}`); rec.Code != http.StatusNoContent {
		t.Fatalf("open material B = %d, want 204; body=%s", rec.Code, rec.Body)
	}

	perspectivesBody := `{"perspectives":[
		{"text":"中国国内视角：可再生能源产业规模全球第一","level":"national"},
		{"text":"全球视角：中国碳排放总量仍是世界第一，抵消了绿化收益","level":"global_against"}
	]}`
	rec = httptest.NewRecorder()
	hCore.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+pid+"/perspectives",
		strings.NewReader(perspectivesBody)), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("submit perspectives = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	// Not yet attested: S2 must still read current, not done.
	snap = fetchStations(t, hCore, pid, cookie)
	if got := stationState(t, snap, "S2"); got != "current" {
		t.Fatalf("S2 state after perspectives submit (before attest) = %q, want current", got)
	}

	rec = httptest.NewRecorder()
	hCore.ServeHTTP(rec, withCookie(httptest.NewRequest("POST",
		"/api/v1/projects/"+pid+"/gate/evaluate_perspectives/attest",
		strings.NewReader(`{"item":"sources_per_perspective","confirmed":true}`)), cookie))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("attest sources_per_perspective = %d, want 204; body=%s", rec.Code, rec.Body)
	}
	snap = fetchStations(t, hCore, pid, cookie)
	if got := stationState(t, snap, "S2"); got != "done" {
		t.Fatalf("S2 state after sources_per_perspective attest = %q, want done", got)
	}
	if got := stationState(t, snap, "S3"); got != "current" {
		t.Fatalf("S3 state after sources_per_perspective attest = %q, want current", got)
	}

	// --- S3 evaluate_sources ------------------------------------------------
	// Round 1: CRAAP on matA — Task 11 (spec-read-together-redesign) moved
	// craap/sift to the reading room's own open-source trigger
	// (prepareSourceAnnotation), so the walk picks which source it opens
	// first explicitly, rather than letting /turn's classifier discover one.
	checked1 := matA
	cidCraap1, anchors1 := surfaceReadingCard(t, hCore, pool, cookie, pid, "craap", checked1)
	activateWalkCard(t, hCore, cookie, pid, cidCraap1)
	submitWalkCard(t, hCore, cookie, pid, cidCraap1, fillCraapAnchors(anchors1, checked1))
	assertCardCompleted(t, pool, cidCraap1)

	// The lateral material for SIFT is whichever of A/B was NOT just checked.
	lateral1 := matB

	// Round 2: SIFT surfaces on the SAME (just-checked) material, reached the
	// same way — opening it again (prepareSourceAnnotation is idempotent
	// server-side, materials.go's own doc comment: a second open finds the
	// card already surfaced and no-ops, or here, surfaces the NEXT due card).
	cidSift, _ := surfaceReadingCard(t, hCore, pool, cookie, pid, "sift", checked1)
	activateWalkCard(t, hCore, cookie, pid, cidSift)
	submitWalkCard(t, hCore, cookie, pid, cidSift, fillSiftAnchors(checked1, lateral1))
	assertCardCompleted(t, pool, cidSift)

	// Round 3: CRAAP on the other (still-unevaluated) material.
	checked2 := lateral1
	cidCraap2, anchors2 := surfaceReadingCard(t, hCore, pool, cookie, pid, "craap", checked2)
	activateWalkCard(t, hCore, cookie, pid, cidCraap2)
	submitWalkCard(t, hCore, cookie, pid, cidCraap2, fillCraapAnchors(anchors2, checked2))
	assertCardCompleted(t, pool, cidCraap2)

	// This is the slice's whole thesis: nothing produced by the work above
	// closes S3 on its own — S3 must still read "current" here, exactly the
	// same pre-assertion S2 makes before its own attest above. If some
	// upstream step ever started satisfying evaluate_sources's machine gate
	// by itself, this would catch it; only the 信源体检 order below may close
	// the station.
	snap = fetchStations(t, hCore, pid, cookie)
	if got := stationState(t, snap, "S3"); got != "current" {
		t.Fatalf("S3 state after all source work but before 信源体检 order = %q, want current (the spot-check order is what must close this station, not the CRAAP/SIFT work alone)", got)
	}
	if !spotCheckOrderable(t, hCore, pid, cookie, "evaluateSources") {
		t.Fatalf("spotChecks.evaluateSources.orderable = false — the 信源体检 button (disabled={pending || !data.orderable}) would not have been pressable here")
	}

	// Order 信源体检: POST /contracts/evaluate_sources/spot-check
	// (api/writing.ts's orderSpotCheck) — needs its own Provider stub, since
	// unlike anchor generation/coach refeed this endpoint requires a REAL
	// parseable JSON reply (no deterministic fallback).
	spotSrcReply := `[` +
		`{"target_id":"` + checked1 + `","evidence":"讲清了这条来源的时效与权威","missing":"没写清楚它的目的倾向","fix":"补充这条来源的写作目的"},` +
		`{"target_id":"` + checked2 + `","evidence":"讲清了独立信源的横向核查过程","missing":"缺少对样本量的说明","fix":"核实原始数据的样本范围"}` +
		`]`
	hSpotSources := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: spotCheckStubProvider(spotSrcReply), ChatResolver: fakeResolver(), SpecByID: cards.ByID,
	}).Handler()
	rec = httptest.NewRecorder()
	hSpotSources.ServeHTTP(rec, withCookie(httptest.NewRequest("POST",
		"/api/v1/projects/"+pid+"/contracts/evaluate_sources/spot-check", strings.NewReader("")), cookie))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "event: review") {
		t.Fatalf("order 信源体检 = %d, want 200 with a review event; body=%s", rec.Code, rec.Body)
	}

	snap = fetchStations(t, hCore, pid, cookie)
	if got := stationState(t, snap, "S3"); got != "done" {
		t.Fatalf("S3 state after 信源体检 order = %q, want done", got)
	}
	if got := stationState(t, snap, "S4"); got != "current" {
		t.Fatalf("S4 state after 信源体检 order = %q, want current", got)
	}

	// --- S4 build_argument ---------------------------------------------------
	cidToulmin, _, _ := surfaceWalkCard(t, hCore, pool, cookie, pid, "toulmin", "该开始搭论证结构了")
	activateWalkCard(t, hCore, cookie, pid, cidToulmin)
	submitWalkCard(t, hCore, cookie, pid, cidToulmin, toulminAnchors(matA))
	assertCardCompleted(t, pool, cidToulmin)

	// Same thesis as S3's pre-assertion above: the Toulmin card completing
	// does not by itself close build_argument — only the 论证体检 order does.
	snap = fetchStations(t, hCore, pid, cookie)
	if got := stationState(t, snap, "S4"); got != "current" {
		t.Fatalf("S4 state after toulmin completion but before 论证体检 order = %q, want current (the spot-check order is what must close this station, not the card completing alone)", got)
	}
	if !spotCheckOrderable(t, hCore, pid, cookie, "buildArgument") {
		t.Fatalf("spotChecks.buildArgument.orderable = false — the 论证体检 button (disabled={pending || !data.orderable}) would not have been pressable here")
	}

	// Order 论证体检: POST /contracts/build_argument/spot-check.
	spotArgReply := `[` +
		`{"target_id":"claim","evidence":"主张写得清楚","missing":"","fix":""},` +
		`{"target_id":"warrant","evidence":"推理链条基本完整","missing":"","fix":""},` +
		`{"target_id":"evidence","evidence":"证据来源明确","missing":"","fix":""},` +
		`{"target_id":"counter","evidence":"反方陈述到位","missing":"","fix":""},` +
		`{"target_id":"concession","evidence":"让步与转折清楚","missing":"","fix":""}` +
		`]`
	hSpotArgument := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: spotCheckStubProvider(spotArgReply), ChatResolver: fakeResolver(), SpecByID: cards.ByID,
	}).Handler()
	rec = httptest.NewRecorder()
	hSpotArgument.ServeHTTP(rec, withCookie(httptest.NewRequest("POST",
		"/api/v1/projects/"+pid+"/contracts/build_argument/spot-check", strings.NewReader("")), cookie))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "event: review") {
		t.Fatalf("order 论证体检 = %d, want 200 with a review event; body=%s", rec.Code, rec.Body)
	}

	snap = fetchStations(t, hCore, pid, cookie)
	if got := stationState(t, snap, "S4"); got != "done" {
		t.Fatalf("S4 state after 论证体检 order = %q, want done", got)
	}
	if got := stationState(t, snap, "S5"); got != "current" {
		t.Fatalf("S5 state after 论证体检 order = %q, want current", got)
	}

	// --- S5 draft_polish -----------------------------------------------------
	// Commit an in-band snapshot: POST /snapshots (api/writing.ts's
	// commitSnapshot). writing-project.json's word_budget is 1500..2000;
	// 1700 CJK characters count as 1700 words (agent.CountWords).
	content := strings.Repeat("字", 1700)
	rec = httptest.NewRecorder()
	hCore.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+pid+"/snapshots",
		strings.NewReader(`{"content":`+strconv.Quote(content)+`}`)), cookie))
	if rec.Code != http.StatusCreated {
		t.Fatalf("commit snapshot = %d, want 201; body=%s", rec.Code, rec.Body)
	}
	var snapResp struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &snapResp); err != nil || snapResp.ID == "" {
		t.Fatalf("decode snapshot response: %v — %s", err, rec.Body.String())
	}

	// Attest citations_matched: POST /gate/draft_polish/attest.
	rec = httptest.NewRecorder()
	hCore.ServeHTTP(rec, withCookie(httptest.NewRequest("POST",
		"/api/v1/projects/"+pid+"/gate/draft_polish/attest",
		strings.NewReader(`{"item":"citations_matched","confirmed":true}`)), cookie))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("attest citations_matched = %d, want 204; body=%s", rec.Code, rec.Body)
	}

	// Same thesis as S2/S3/S4's pre-assertions above: committing an in-band
	// snapshot and attesting citations_matched do not by themselves close
	// draft_polish — only the 整稿体检 order does.
	snap = fetchStations(t, hCore, pid, cookie)
	if got := stationState(t, snap, "S5"); got != "current" {
		t.Fatalf("S5 state after snapshot commit + citations_matched attest but before 整稿体检 order = %q, want current (the review order is what must close this station)", got)
	}

	// Order 整稿体检: POST /snapshots/{id}/review (api/writing.ts's
	// orderReview) — its own Provider stub, same reason as the spot-checks.
	reviewReply := `[{"criterion_code":"表D","band":"5–6 段","evidence":"引用来源清楚","missing":"","fix":""}]`
	hReview := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: reviewStubProvider(reviewReply), ChatResolver: fakeResolver(), SpecByID: cards.ByID,
	}).Handler()
	rec = httptest.NewRecorder()
	hReview.ServeHTTP(rec, withCookie(httptest.NewRequest("POST",
		"/api/v1/projects/"+pid+"/snapshots/"+snapResp.ID+"/review", strings.NewReader("")), cookie))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "event: review") {
		t.Fatalf("order 整稿体检 = %d, want 200 with a review event; body=%s", rec.Code, rec.Body)
	}

	snap = fetchStations(t, hCore, pid, cookie)
	if got := stationState(t, snap, "S5"); got != "done" {
		t.Fatalf("S5 state after 整稿体检 order = %q, want done", got)
	}
	if got := stationState(t, snap, "S6"); got != "current" {
		t.Fatalf("S6 state after 整稿体检 order = %q, want current", got)
	}

	// --- S6 reflect_archive: POST /reflection + POST /declaration/sign ------
	rec = httptest.NewRecorder()
	hCore.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+pid+"/reflection",
		strings.NewReader(`{"text":"我一开始以为证据够了，被追问后才发现来源单一，于是补了独立信源核查。"}`)), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("submit reflection = %d, want 200; body=%s", rec.Code, rec.Body)
	}

	rec = httptest.NewRecorder()
	hCore.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+pid+"/declaration/sign",
		strings.NewReader("")), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("sign declaration = %d, want 200; body=%s", rec.Code, rec.Body)
	}

	snap = fetchStations(t, hCore, pid, cookie)
	if got := stationState(t, snap, "S6"); got != "done" {
		t.Fatalf("S6 state after declaration sign = %q, want done — the full S0->S6 walk did not complete", got)
	}
}
