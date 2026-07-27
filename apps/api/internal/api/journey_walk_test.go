package api_test

// journey_walk_test.go — N6-E Task 7: the slice's acceptance walk. Proves the
// waived-set journey (compose-at-creation, Task 3) works end to end over
// CLIENT-REACHABLE HTTP endpoints only — the same N3d/N3f discipline
// walk_s0_s6_test.go documents — and that the reopen escape hatch (Task 5)
// actually un-waives a station in the projection a real screen reads.
//
// TestJourneyWalk_WaivedFrontCompletesAndFinishes creates a project through a
// composer stubbed to waive decode_task/frame_question/evaluate_perspectives
// (S0-S2), confirms the projection renders them "waived" and S3 "current",
// then walks S3->S6 to completion reusing walk_s0_s6_test.go's own helpers
// verbatim (surfaceWalkCard/activateWalkCard/submitWalkCard/
// fillCraapAnchors/fillSiftAnchors/toulminAnchors/spotCheckOrderable,
// fetchStations/stationState from walkable_stations_test.go) — no second
// harness invented. It ends by asserting canFinish reads true and POST
// finish succeeds, with S0-S2 never having gone "done" — proving waived
// counts as satisfied for finish.
//
// One real wiring fact surfaces along the way: waiving evaluate_perspectives
// never produces the >=2 perspective nodes perspective-matrix's own N3a
// trigger (`anyEvaluated && perspectiveNodes<2`) reads, so the classifier
// keeps offering that card ahead of Toulmin once any source is evaluated —
// exactly the "a waived upstream station leaves its output un-produced"
// limit the slice's own docs name. The walk below dismisses that one offer
// via POST .../cards/{cid}/skip (skipWalkCard) — the same escape hatch a
// real student has (an offer is never a wall, 铁律 2) — which permanently
// retires it (classifier.go: ANY card_instance status suppresses the
// re-offer), then proceeds to Toulmin exactly as walk_s0_s6_test.go's own S4
// does.
//
// TestJourneyWalk_ReopenReintroducesStation drives the same waived front,
// then POSTs /journey/reopen/S1 and confirms the projection's S1 no longer
// reads "waived" while S0/S2 (never reopened) still do.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/store/sqlc"
)

// waiveS0S1S2ComposeReply keeps decode_task/frame_question/
// evaluate_perspectives (S0-S2) waived and the rest of the template kept —
// the exact decision shape TestCreateProjectComposesJourney
// (project_create_test.go) and TestComposeJourneyWaivesUnkeptContracts
// (journey_test.go) already prove ComposeJourney parses and persists
// correctly. Fed to the package's own composeJourneyStubProvider
// (project_create_test.go).
const waiveS0S1S2ComposeReply = `[` +
	`{"id":"decode_task","keep":false,"reason":"她贴的材料里已经把任务要求讲清楚了"},` +
	`{"id":"frame_question","keep":false,"reason":"她已经带着明确的研究问题进来"},` +
	`{"id":"evaluate_perspectives","keep":false,"reason":"她已经想过正反两方的立场"},` +
	`{"id":"evaluate_sources","keep":true,"reason":""},` +
	`{"id":"build_argument","keep":true,"reason":""},` +
	`{"id":"draft_polish","keep":true,"reason":""},` +
	`{"id":"reflect_archive","keep":true,"reason":""}` +
	`]`

// newWaivedFrontProject creates a project through the real funnel (POST
// /api/v1/projects) via a handler whose Provider is stubbed to waive S0-S2,
// and asserts the projection renders exactly that before returning the
// project id — the shared arrangement step both tests in this file start
// from.
func newWaivedFrontProject(t *testing.T, hCompose http.Handler, cookie *http.Cookie) string {
	t.Helper()
	pid := createProjectForTest(t, hCompose, cookie)
	snap := fetchStations(t, hCompose, pid, cookie)
	for _, code := range []string{"S0", "S1", "S2"} {
		if got := stationState(t, snap, code); got != "waived" {
			t.Fatalf("%s state after compose-at-creation = %q, want waived", code, got)
		}
	}
	if got := stationState(t, snap, "S3"); got != "current" {
		t.Fatalf("S3 state after compose-at-creation = %q, want current (evaluate_perspectives waived counts as satisfied for its own reachability check)", got)
	}
	return pid
}

// skipWalkCard drives POST /cards/{cid}/skip (api/projectCards.ts's
// skipProjectCard) — the real screen control for dismissing a surfaced card
// without opening it. 过程即数据: a skip is still submitted with an event
// trace, not silently discarded.
func skipWalkCard(t *testing.T, h http.Handler, cookie *http.Cookie, projectID, cid string) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST",
		"/api/v1/projects/"+projectID+"/cards/"+cid+"/skip",
		strings.NewReader(`{"event_trace":[{"kind":"skip","at":"2026-07-23T00:00:00Z"}]}`)), cookie))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("skip card %s: %d — %s", cid, rec.Code, rec.Body.String())
	}
}

func TestJourneyWalk_WaivedFrontCompletesAndFinishes(t *testing.T) {
	pool := newAPITestPool(t)
	cookie := signInSeed(t, pool)

	// --- create (compose-at-creation waives S0-S2) --------------------------
	hCompose := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: composeJourneyStubProvider(waiveS0S1S2ComposeReply), ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(),
		SpecByID: cards.ByID,
	}).Handler()
	pid := newWaivedFrontProject(t, hCompose, cookie)

	// --- everything else: hCore, the same plain-text-fallback provider
	// walk_s0_s6_test.go's own S3->S6 portion uses (anchor generation/coach
	// refeed degrade gracefully to a deterministic fallback on its non-JSON
	// reply). Only the create call above used a JSON-returning provider.
	hCore := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: fakeProvider(), ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(),
		SpecByID: cards.ByID,
	}).Handler()

	// --- S3 evaluate_sources: POST /materials (x2) + /open + card round-trips,
	// closed by 信源体检 -------------------------------------------------------
	matA := ingestMaterialForTest(t, hCore, cookie, pid, "NASA 绿化研究", walkMaterialTextA)
	if rec := openMaterial(t, hCore, cookie, pid, matA, `{"time_spent_s":30}`); rec.Code != http.StatusNoContent {
		t.Fatalf("open material A = %d, want 204; body=%s", rec.Code, rec.Body)
	}
	matB := ingestMaterialForTest(t, hCore, cookie, pid, "独立环境智库报告", walkMaterialTextB)
	if rec := openMaterial(t, hCore, cookie, pid, matB, `{"time_spent_s":20}`); rec.Code != http.StatusNoContent {
		t.Fatalf("open material B = %d, want 204; body=%s", rec.Code, rec.Body)
	}

	// Task 11 (spec-read-together-redesign): craap/sift surface from opening
	// a specific source (prepareSourceAnnotation), not /turn — mirrors
	// walk_s0_s6_test.go's own S3 (surfaceReadingCard's doc comment).
	checked1 := matA
	cidCraap1, anchors1 := surfaceReadingCard(t, hCore, pool, cookie, pid, "craap", checked1)
	activateWalkCard(t, hCore, cookie, pid, cidCraap1)
	submitWalkCard(t, hCore, cookie, pid, cidCraap1, fillCraapAnchors(anchors1, checked1))
	assertCardCompleted(t, pool, cidCraap1)

	lateral1 := matB

	cidSift, _ := surfaceReadingCard(t, hCore, pool, cookie, pid, "sift", checked1)
	activateWalkCard(t, hCore, cookie, pid, cidSift)
	submitWalkCard(t, hCore, cookie, pid, cidSift, fillSiftAnchors(checked1, lateral1))
	assertCardCompleted(t, pool, cidSift)

	checked2 := lateral1
	cidCraap2, anchors2 := surfaceReadingCard(t, hCore, pool, cookie, pid, "craap", checked2)
	activateWalkCard(t, hCore, cookie, pid, cidCraap2)
	submitWalkCard(t, hCore, cookie, pid, cidCraap2, fillCraapAnchors(anchors2, checked2))
	assertCardCompleted(t, pool, cidCraap2)

	snap := fetchStations(t, hCore, pid, cookie)
	if got := stationState(t, snap, "S3"); got != "current" {
		t.Fatalf("S3 state after all source work but before 信源体检 order = %q, want current", got)
	}
	if !spotCheckOrderable(t, hCore, pid, cookie, "evaluateSources") {
		t.Fatalf("spotChecks.evaluateSources.orderable = false — the 信源体检 button would not have been pressable here")
	}

	spotSrcReply := `[` +
		`{"target_id":"` + checked1 + `","evidence":"讲清了这条来源的时效与权威","missing":"没写清楚它的目的倾向","fix":"补充这条来源的写作目的"},` +
		`{"target_id":"` + checked2 + `","evidence":"讲清了独立信源的横向核查过程","missing":"缺少对样本量的说明","fix":"核实原始数据的样本范围"}` +
		`]`
	hSpotSources := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: spotCheckStubProvider(spotSrcReply), ChatResolver: fakeResolver(), SpecByID: cards.ByID,
	}).Handler()
	rec := httptest.NewRecorder()
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

	// --- S4 build_argument ----------------------------------------------------
	// The known N6-E limit surfaces here: waiving evaluate_perspectives never
	// wrote the >=2 perspective nodes perspective-matrix's own trigger reads,
	// so the classifier offers it ahead of Toulmin the moment any source is
	// evaluated. Dismiss it once (an offer is never a wall) — this
	// permanently retires the offer — then Toulmin surfaces exactly as
	// walk_s0_s6_test.go's own S4 does.
	cidPM, _, _ := surfaceWalkCard(t, hCore, pool, cookie, pid, "perspective-matrix", "该开始搭论证结构了")
	skipWalkCard(t, hCore, cookie, pid, cidPM)

	cidToulmin, _, _ := surfaceWalkCard(t, hCore, pool, cookie, pid, "toulmin", "视角先放一放，来搭论证结构")
	activateWalkCard(t, hCore, cookie, pid, cidToulmin)
	submitWalkCard(t, hCore, cookie, pid, cidToulmin, toulminAnchors(matA))
	assertCardCompleted(t, pool, cidToulmin)

	snap = fetchStations(t, hCore, pid, cookie)
	if got := stationState(t, snap, "S4"); got != "current" {
		t.Fatalf("S4 state after toulmin completion but before 论证体检 order = %q, want current", got)
	}
	if !spotCheckOrderable(t, hCore, pid, cookie, "buildArgument") {
		t.Fatalf("spotChecks.buildArgument.orderable = false — the 论证体检 button would not have been pressable here")
	}

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

	// --- S5 draft_polish --------------------------------------------------------
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

	rec = httptest.NewRecorder()
	hCore.ServeHTTP(rec, withCookie(httptest.NewRequest("POST",
		"/api/v1/projects/"+pid+"/gate/draft_polish/attest",
		strings.NewReader(`{"item":"citations_matched","confirmed":true}`)), cookie))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("attest citations_matched = %d, want 204; body=%s", rec.Code, rec.Body)
	}

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

	// --- S6 reflect_archive: POST /reflection + POST /declaration/sign --------
	rec = httptest.NewRecorder()
	hCore.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+pid+"/reflection",
		strings.NewReader(`{"text":"我一开始以为跳过三步能省时间，结果论证卡还是主动来问我要视角，我先跳过那个提议，回头再补齐视角对比。"}`)), cookie))
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
		t.Fatalf("S6 state after declaration sign = %q, want done — the full S3->S6 walk did not complete", got)
	}

	// --- the whole thesis: S0/S1/S2 never went "done" — they read "waived"
	// from creation all the way through this walk — yet canFinish reads true
	// and finish succeeds.
	for _, code := range []string{"S0", "S1", "S2"} {
		if got := stationState(t, snap, code); got != "waived" {
			t.Fatalf("%s state at the end of the walk = %q, want still waived (never done)", code, got)
		}
	}

	var proj struct {
		CanFinish bool `json:"canFinish"`
	}
	rec = httptest.NewRecorder()
	hCore.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/projects/"+pid, nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET project (canFinish read) = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &proj); err != nil {
		t.Fatalf("unmarshal projection for canFinish: %v; raw=%s", err, rec.Body)
	}
	if !proj.CanFinish {
		t.Fatalf("canFinish = false after the full S3-S6 walk with S0-S2 waived, want true — three stations never done should still let the project finish")
	}

	hFinish := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: assessStubProvider(dualAxisReply), ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(),
		SpecByID: cards.ByID,
	}).Handler()
	rec = httptest.NewRecorder()
	hFinish.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+pid+"/finish", strings.NewReader("")), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("finish = %d, want 200; body=%s", rec.Code, rec.Body)
	}
}

// TestJourneyWalk_ReopenReintroducesStation — same waived front as the walk
// above; POST /journey/reopen/S1 un-waives frame_question, and the
// projection's S1 must stop reading "waived" while S0/S2 (never reopened)
// still do.
func TestJourneyWalk_ReopenReintroducesStation(t *testing.T) {
	pool := newAPITestPool(t)
	cookie := signInSeed(t, pool)

	hCompose := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: composeJourneyStubProvider(waiveS0S1S2ComposeReply), ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(),
		SpecByID: cards.ByID,
	}).Handler()
	pid := newWaivedFrontProject(t, hCompose, cookie)

	rec := httptest.NewRecorder()
	hCompose.ServeHTTP(rec, withCookie(httptest.NewRequest("POST",
		"/api/v1/projects/"+pid+"/journey/reopen/S1", nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("reopen S1 = %d, want 200; body=%s", rec.Code, rec.Body)
	}

	snap := fetchStations(t, hCompose, pid, cookie)
	if got := stationState(t, snap, "S1"); got == "waived" {
		t.Fatalf("S1 state after reopen = %q, want no longer waived", got)
	}
	if got := stationState(t, snap, "S0"); got != "waived" {
		t.Fatalf("S0 state after reopening ONLY S1 = %q, want still waived", got)
	}
	if got := stationState(t, snap, "S2"); got != "waived" {
		t.Fatalf("S2 state after reopening ONLY S1 = %q, want still waived", got)
	}
}
