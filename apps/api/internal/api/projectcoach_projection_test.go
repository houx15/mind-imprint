package api_test

// projectcoach_projection_test.go — Task 7 (S2): the always-on spine
// projection's 文献库 block must carry the reading sub-agent's state, not
// just the pre-S2 `title｜decision` line — 已归纳 (finalized takeaway) shows
// the durable 印记 (proposal_impact), 在读 (material set, not yet finalized)
// shows a gentle in-progress finding count, and 未读 (no material) is
// unchanged. buildSpineProjection takes no *http.Request, so it's reached
// via the BuildSpineProjectionForTest seam (export_test.go) rather than
// through an HTTP round-trip.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/store/sqlc"
)

// projectionTestHandler is planTestHandler's twin, but also keeps the *API
// pointer alive — buildSpineProjection is reached directly (no *http.Request
// to drive it through, unlike the request-scoped endpoints other test files
// exercise purely through the returned http.Handler).
func projectionTestHandler(t *testing.T) (api *API, h http.Handler, cookie *http.Cookie, q *sqlc.Queries) {
	t.Helper()
	pool := newAPITestPool(t)
	q = sqlc.New(pool)
	api = New(Deps{
		Queries: q, Pool: pool,
		Provider: fakeProvider(), ChatResolver: fakeResolver(), SpecByID: cards.ByID,
	})
	cookie = signInSeed(t, pool)
	return api, api.Handler(), cookie, q
}

// seedFinalizedReference seeds a reference with a linked material, a phase
// tag, and a FINALIZED takeaway carrying proposalImpact — the 已归纳 state.
func seedFinalizedReference(t *testing.T, h http.Handler, cookie *http.Cookie, q *sqlc.Queries, title, phaseTag, proposalImpact string) {
	t.Helper()
	ctx := context.Background()
	projectID := mustUUID(seedProjectID)
	matID := ingestMaterialForTest(t, h, cookie, seedProjectID, title, craapMaterialText)
	ref, err := q.CreateReference(ctx, sqlc.CreateReferenceParams{
		ProjectID: projectID, Title: title, Tags: []byte("[]"), SearchHints: []byte("[]"),
	})
	if err != nil {
		t.Fatalf("CreateReference: %v", err)
	}
	if _, err := q.SetReferenceMaterial(ctx, sqlc.SetReferenceMaterialParams{
		ID: ref.ID, ProjectID: projectID,
		MaterialID: pgtype.UUID{Bytes: uuid.MustParse(matID), Valid: true},
	}); err != nil {
		t.Fatalf("link reference to material: %v", err)
	}
	phase := phaseTag
	if _, err := q.UpdateReadingBrief(ctx, sqlc.UpdateReadingBriefParams{
		ID: ref.ID, ProjectID: projectID, PhaseTag: &phase,
	}); err != nil {
		t.Fatalf("set phase tag: %v", err)
	}
	// EA: a real finalized takeaway carries findings + a credibility verdict too,
	// not just proposalImpact — the projection must surface them.
	raw, merr := json.Marshal(map[string]any{
		"proposalImpact": proposalImpact,
		"findings":       []string{"中国贡献了全球净叶面积增加的相当一部分"},
		"credibility":    map[string]any{"verdict": "strong", "why": "同行评议"},
	})
	if merr != nil {
		t.Fatalf("marshal takeaway: %v", merr)
	}
	if _, err := q.FinalizeReadingTakeaway(ctx, sqlc.FinalizeReadingTakeawayParams{
		ID: ref.ID, ProjectID: projectID, Takeaway: raw,
	}); err != nil {
		t.Fatalf("finalize takeaway: %v", err)
	}
}

// seedInProgressReference seeds a reference with a linked material and ONE
// CONFIRMED (status=completed) reading card attributed to it via anchors —
// so readingOutcomesByMaterialCtx's Findings count is >= 1 — but no
// finalized takeaway. The 在读 state.
func seedInProgressReference(t *testing.T, h http.Handler, cookie *http.Cookie, q *sqlc.Queries, title string) {
	t.Helper()
	ctx := context.Background()
	projectID := mustUUID(seedProjectID)
	matID := ingestMaterialForTest(t, h, cookie, seedProjectID, title, craapMaterialText)
	ref, err := q.CreateReference(ctx, sqlc.CreateReferenceParams{
		ProjectID: projectID, Title: title, Tags: []byte("[]"), SearchHints: []byte("[]"),
	})
	if err != nil {
		t.Fatalf("CreateReference: %v", err)
	}
	if _, err := q.SetReferenceMaterial(ctx, sqlc.SetReferenceMaterialParams{
		ID: ref.ID, ProjectID: projectID,
		MaterialID: pgtype.UUID{Bytes: uuid.MustParse(matID), Valid: true},
	}); err != nil {
		t.Fatalf("link reference to material: %v", err)
	}
	ci, err := q.CreateProjectCardInstance(ctx, sqlc.CreateProjectCardInstanceParams{
		ProjectID: pgtype.UUID{Bytes: projectID, Valid: true}, CardID: "toulmin", Status: "active",
	})
	if err != nil {
		t.Fatalf("create card instance: %v", err)
	}
	anchors := fmt.Sprintf(`[{"id":"a0","material_id":%q,"quote":"支持论点的一句话","author":"student"}]`, matID)
	if _, err := q.SetCardInstanceAnchors(ctx, sqlc.SetCardInstanceAnchorsParams{
		ID: ci.ID, ProjectID: pgtype.UUID{Bytes: projectID, Valid: true}, Anchors: []byte(anchors),
	}); err != nil {
		t.Fatalf("set anchors: %v", err)
	}
	framework, merr := json.Marshal(map[string]string{"finding": "支持论点的一句话", "judgment": "支撑论点"})
	if merr != nil {
		t.Fatalf("marshal framework: %v", merr)
	}
	if _, err := q.SetCardInstanceFramework(ctx, sqlc.SetCardInstanceFrameworkParams{
		ID: ci.ID, ProjectID: pgtype.UUID{Bytes: projectID, Valid: true}, FrameworkFill: framework,
	}); err != nil {
		t.Fatalf("set framework: %v", err)
	}
	if _, err := q.SetCardInstanceStatus(ctx, sqlc.SetCardInstanceStatusParams{
		ID: ci.ID, ProjectID: pgtype.UUID{Bytes: projectID, Valid: true}, Status: "completed",
	}); err != nil {
		t.Fatalf("complete card: %v", err)
	}
}

// TestProjection_ReadingStates — Task 7: the spine projection's 文献库 block
// must carry the reading sub-agent's state: 已归纳 shows the durable 印记
// (proposal_impact), 在读 shows a gentle in-progress finding count.
func TestProjection_ReadingStates(t *testing.T) {
	api, h, cookie, q := projectionTestHandler(t)
	_ = cookie

	seedFinalizedReference(t, h, cookie, q, "已归纳源", "反例检验", "作为让步段证据")
	seedInProgressReference(t, h, cookie, q, "在读源")

	proj, err := api.BuildSpineProjectionForTest(context.Background(), mustUUID(seedProjectID))
	if err != nil {
		t.Fatalf("projection: %v", err)
	}
	if !strings.Contains(proj, "印记：作为让步段证据") {
		t.Fatalf("已归纳 source must carry proposal_impact:\n%s", proj)
	}
	// EA: findings + credibility now reach the main coach too (not a dead-end).
	if !strings.Contains(proj, "发现：中国贡献了全球净叶面积增加的相当一部分") {
		t.Fatalf("已归纳 source must surface the top finding:\n%s", proj)
	}
	if !strings.Contains(proj, "可信度：strong") {
		t.Fatalf("已归纳 source must surface the credibility verdict:\n%s", proj)
	}
	// Retarget on the distinguishing suffix, not bare "在读": the seeded
	// in-progress title is itself "在读源", which contains "在读" and is
	// rendered in EVERY branch — so a bare Contains("在读") passes even if
	// the 在读 branch were deleted. The finding-count suffix only appears
	// from the 在读 branch, so this fails if that branch breaks.
	if !strings.Contains(proj, "在读·已确认 1 条发现") {
		t.Fatalf("在读 source must show exactly 1 confirmed finding:\n%s", proj)
	}
}

// seedUnreadReference seeds a reference with NO linked material — the 未读
// state — but with Credibility and Decision set through the real PATCH
// /references/{rid} endpoint, exactly as the pre-reading 信源体检 flow does
// (both fields are independently settable before a student ever opens the
// material). Regression guard for Finding 1: the 未读 branch must still
// render BOTH ｜<decision> and ｜可信度 <credibility>, exactly like the
// pre-S2 line.
func seedUnreadReference(t *testing.T, h http.Handler, cookie *http.Cookie, title, credibility, decision string) {
	t.Helper()
	base := "/api/v1/projects/" + seedProjectID
	rec := doJSON(t, h, cookie, "POST", base+"/references", fmt.Sprintf(`{"title":%q}`, title))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create reference: %d %s", rec.Code, rec.Body)
	}
	var wrap struct {
		Reference struct {
			ID string `json:"id"`
		} `json:"reference"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &wrap); err != nil {
		t.Fatalf("decode reference: %v — %s", err, rec.Body)
	}
	patch := fmt.Sprintf(`{"credibility":%q,"decision":%q}`, credibility, decision)
	rec = doJSON(t, h, cookie, "PATCH", base+"/references/"+wrap.Reference.ID, patch)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch reference credibility/decision: %d %s", rec.Code, rec.Body)
	}
}

// TestProjection_ExplorationLine — Task 7: the spine projection's 文献库
// block must surface the exploration graph's branch state (open leads +
// dangling sources) so the continuous coach stays aware of it, and must omit
// the line entirely when there's nothing to flag.
func TestProjection_ExplorationLine(t *testing.T) {
	api, h, cookie, q := projectionTestHandler(t)
	base := "/api/v1/projects/" + seedProjectID

	// 2 open leads.
	for _, text := range []string{"中国的碳排放总量会不会推翻论点？", "另一条待追的线索"} {
		rec := doJSON(t, h, cookie, "POST", base+"/exploration/leads", fmt.Sprintf(`{"text":%q}`, text))
		if rec.Code != http.StatusCreated {
			t.Fatalf("create lead: %d %s", rec.Code, rec.Body)
		}
	}

	// 1 dangling source: a reference with an engaged material, no decision,
	// and not connected by any lead.
	ctx := context.Background()
	projectID := mustUUID(seedProjectID)
	matID := ingestMaterialForTest(t, h, cookie, seedProjectID, "悬空源", craapMaterialText)
	ref, err := q.CreateReference(ctx, sqlc.CreateReferenceParams{
		ProjectID: projectID, Title: "悬空源", Tags: []byte("[]"), SearchHints: []byte("[]"),
	})
	if err != nil {
		t.Fatalf("CreateReference: %v", err)
	}
	if _, err := q.SetReferenceMaterial(ctx, sqlc.SetReferenceMaterialParams{
		ID: ref.ID, ProjectID: projectID,
		MaterialID: pgtype.UUID{Bytes: uuid.MustParse(matID), Valid: true},
	}); err != nil {
		t.Fatalf("link reference to material: %v", err)
	}

	proj, err := api.BuildSpineProjectionForTest(ctx, projectID)
	if err != nil {
		t.Fatalf("projection: %v", err)
	}
	if !strings.Contains(proj, "探索：待追 2 条线索 · 1 个悬空来源") {
		t.Fatalf("projection must surface open leads + dangling sources:\n%s", proj)
	}
}

// TestProjection_ExplorationLineOmittedWhenClear — the 探索 line must be
// entirely absent when there are zero open leads and zero dangling sources
// (a project that hasn't touched exploration at all, e.g. the projection
// tests' seed project before this test runs).
func TestProjection_ExplorationLineOmittedWhenClear(t *testing.T) {
	api, _, _, _ := projectionTestHandler(t)

	proj, err := api.BuildSpineProjectionForTest(context.Background(), mustUUID(seedProjectID))
	if err != nil {
		t.Fatalf("projection: %v", err)
	}
	if strings.Contains(proj, "探索：") {
		t.Fatalf("projection must NOT contain a 探索 line when clear:\n%s", proj)
	}
}

// TestProjection_UnreadCarriesCredibility — Finding 1 regression guard: a
// 未读 reference (no material) that has Credibility set via the
// pre-reading 信源体检 flow must still surface it in the projection line,
// alongside Decision, exactly like the pre-S2 line. Must FAIL if the
// Credibility append is dropped from the 未读 (default) branch.
func TestProjection_UnreadCarriesCredibility(t *testing.T) {
	api, h, cookie, _ := projectionTestHandler(t)

	seedUnreadReference(t, h, cookie, "未读源", "strong", "use")

	proj, err := api.BuildSpineProjectionForTest(context.Background(), mustUUID(seedProjectID))
	if err != nil {
		t.Fatalf("projection: %v", err)
	}
	if !strings.Contains(proj, "｜use") {
		t.Fatalf("未读 source must carry decision:\n%s", proj)
	}
	if !strings.Contains(proj, "｜可信度 strong") {
		t.Fatalf("未读 source must carry credibility:\n%s", proj)
	}
}
