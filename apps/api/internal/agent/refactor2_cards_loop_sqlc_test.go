package agent_test

// refactor2_cards_loop_sqlc_test.go — Task 5's real-DB pass: RunAgentStep's
// surface_card dispatch, CompleteCard's graph_effects/framework write, and
// RecordDisposition, all over the real sqlcAgentStore adapter
// (agentstore.go) and a testcontainers Postgres. Reuses newTurnTestPool /
// seededStudentID from turn_test.go (same agent_test package).

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/skills"
	"mindimprint/api/internal/store/sqlc"
	"mindimprint/api/internal/studio"
)

func TestRefactor2CardsLoop_UnevaluatedSourceSurfacesCraap(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newTurnTestPool(t)
	q := sqlc.New(pool)
	store := agent.NewSqlcAgentStore(q, pool)

	task, err := q.CreateTask(ctx, sqlc.CreateTaskParams{UserID: seededStudentID, Title: "refactor2-cards-loop"})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	project, err := q.CreateProject(ctx, sqlc.CreateProjectParams{
		UserID: seededStudentID, Qualification: "EE", Title: "中国是否让地球变得更可持续？", BoardCfgVer: 1,
	})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	material, err := q.CreateProjectMaterial(ctx, sqlc.CreateProjectMaterialParams{
		TaskID:    pgtype.UUID{Bytes: task.ID, Valid: true},
		ProjectID: pgtype.UUID{Bytes: project.ID, Valid: true},
		Kind:      "article",
		Source:    "fetched",
		Title:     "NASA: China's renewable build-out",
		Blocks:    []byte(`[]`),
	})
	if err != nil {
		t.Fatalf("CreateProjectMaterial: %v", err)
	}

	prov := gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: "should never be called"},
		{Kind: gateway.EventDone, StopReason: gateway.StopStop},
	})
	deps := agent.AgentDeps{Store: store, Provider: prov, Resolved: gateway.Resolved{Provider: "deepseek", Model: "deepseek-chat", Tier: "coach"}}

	action, err := agent.RunAgentStep(ctx, deps, project.ID, agent.Trigger{Kind: "T-B"})
	if err != nil {
		t.Fatalf("RunAgentStep: %v", err)
	}
	if action == nil || action.Kind != "surface_card" {
		t.Fatalf("want a surface_card action, got %+v", action)
	}
	cardInstanceID, err := uuid.Parse(action.CardInstanceID)
	if err != nil {
		t.Fatalf("parse CardInstanceID: %v", err)
	}

	list, err := q.ListCardInstancesByProject(ctx, pgtype.UUID{Bytes: project.ID, Valid: true})
	if err != nil {
		t.Fatalf("ListCardInstancesByProject: %v", err)
	}
	if len(list) != 1 || list[0].ID != cardInstanceID {
		t.Fatalf("ListCardInstancesByProject = %+v, want 1 row matching %s", list, cardInstanceID)
	}
	if list[0].CardID != "craap" || list[0].Status != "proposed" {
		t.Fatalf("unexpected card_instance: card_id=%q status=%q", list[0].CardID, list[0].Status)
	}
	if !list[0].TaskID.Valid || list[0].TaskID.Bytes != task.ID {
		t.Fatalf("TaskID = %+v, want the material's own task %s", list[0].TaskID, task.ID)
	}

	edges, err := q.ListGraphEdgesByProject(ctx, project.ID)
	if err != nil {
		t.Fatalf("ListGraphEdgesByProject: %v", err)
	}
	if len(edges) != 1 {
		t.Fatalf("want 1 graph_edge (card_instance->material), got %d", len(edges))
	}
	if edges[0].FromKind != "card_instance" || edges[0].FromID != cardInstanceID ||
		edges[0].ToKind != "material" || edges[0].ToID != material.ID {
		t.Fatalf("unexpected edge: %+v", edges[0])
	}

	// A second RunAgentStep pass must not re-propose the same material —
	// it now has an evaluation card_instance.
	action2, err := agent.RunAgentStep(ctx, deps, project.ID, agent.Trigger{Kind: "T-B"})
	if err != nil {
		t.Fatalf("RunAgentStep (2nd pass): %v", err)
	}
	if action2 != nil {
		t.Fatalf("want silence on the 2nd pass, got %+v", action2)
	}
}

func TestRefactor2CardsLoop_CompleteCardMintsEvidenceAndFramework(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newTurnTestPool(t)
	q := sqlc.New(pool)
	store := agent.NewSqlcAgentStore(q, pool)

	task, err := q.CreateTask(ctx, sqlc.CreateTaskParams{UserID: seededStudentID, Title: "refactor2-cards-complete"})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	project, err := q.CreateProject(ctx, sqlc.CreateProjectParams{
		UserID: seededStudentID, Qualification: "EE", Title: "中国是否让地球变得更可持续？", BoardCfgVer: 1,
	})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	material, err := q.CreateProjectMaterial(ctx, sqlc.CreateProjectMaterialParams{
		TaskID:    pgtype.UUID{Bytes: task.ID, Valid: true},
		ProjectID: pgtype.UUID{Bytes: project.ID, Valid: true},
		Kind:      "article",
		Source:    "fetched",
		Title:     "NASA: China's renewable build-out",
		Blocks:    []byte(`[]`),
	})
	if err != nil {
		t.Fatalf("CreateProjectMaterial: %v", err)
	}

	spec, ok := cards.ByID("craap")
	if !ok {
		t.Fatal("cards.ByID(craap) not found — Task 1's registry config is missing")
	}

	deps := agent.AgentDeps{Store: store}
	surfaced, err := agent.SurfaceCard(ctx, deps, project.ID, spec, material.ID)
	if err != nil {
		t.Fatalf("SurfaceCard: %v", err)
	}
	cardInstanceID, err := uuid.Parse(surfaced.CardInstanceID)
	if err != nil {
		t.Fatalf("parse CardInstanceID: %v", err)
	}

	anchors := []agent.Anchor{
		{ID: "a0", MaterialID: material.ID.String(), Dimension: "currency", Author: "ai", Answer: "2024年发布，数据较新"},
		{ID: "a1", MaterialID: material.ID.String(), Dimension: "relevance", Author: "ai", Answer: "直接支持中国可持续论点"},
		{ID: "a2", MaterialID: material.ID.String(), Dimension: "authority", Author: "ai", Answer: "NASA地球观测团队发布，具备权威性"},
		{ID: "a3", MaterialID: material.ID.String(), Dimension: "accuracy", Author: "ai", Answer: "数据可在Nature Sustainability交叉核对"},
		{ID: "a4", MaterialID: material.ID.String(), Dimension: "purpose", Author: "ai", Answer: "科普告知性质，非商业推广"},
		{ID: "a5", MaterialID: material.ID.String(), Dimension: "risk_note", Author: "student", Answer: "仍需留意样本口径是否一致"},
	}
	anchorsJSON, err := json.Marshal(anchors)
	if err != nil {
		t.Fatalf("marshal anchors: %v", err)
	}
	if _, err := q.SetCardInstanceAnchors(ctx, sqlc.SetCardInstanceAnchorsParams{
		ID:        cardInstanceID,
		ProjectID: pgtype.UUID{Bytes: project.ID, Valid: true},
		Anchors:   anchorsJSON,
	}); err != nil {
		t.Fatalf("SetCardInstanceAnchors: %v", err)
	}

	complete, err := agent.CompleteCard(ctx, deps, spec, cardInstanceID)
	if err != nil {
		t.Fatalf("CompleteCard: %v", err)
	}
	if !complete {
		t.Fatal("want complete = true")
	}

	nodes, err := q.ListGraphNodesByProject(ctx, project.ID)
	if err != nil {
		t.Fatalf("ListGraphNodesByProject: %v", err)
	}
	if len(nodes) != 1 || nodes[0].Type != "evidence" || nodes[0].Author != "student" {
		t.Fatalf("unexpected minted nodes: %+v", nodes)
	}
	var body map[string]any
	if err := json.Unmarshal(nodes[0].Body, &body); err != nil {
		t.Fatalf("unmarshal node body: %v", err)
	}
	if _, ok := body["source_quality"]; !ok {
		t.Fatalf("evidence node body missing source_quality: %v", body)
	}

	edges, err := q.ListGraphEdgesByProject(ctx, project.ID)
	if err != nil {
		t.Fatalf("ListGraphEdgesByProject: %v", err)
	}
	var evaluatedAs int
	for _, e := range edges {
		if e.Type == "evaluated-as" && e.FromKind == "material" && e.FromID == material.ID && e.ToID == nodes[0].ID {
			evaluatedAs++
		}
	}
	if evaluatedAs != 1 {
		t.Fatalf("want 1 evaluated-as edge material->evidence, got %d among %+v", evaluatedAs, edges)
	}

	got, err := q.GetCardInstance(ctx, cardInstanceID)
	if err != nil {
		t.Fatalf("GetCardInstance: %v", err)
	}
	if got.Status != "proposed" {
		t.Fatalf("Status = %q, want unchanged (proposed) — CompleteCard must never set solid/completed", got.Status)
	}
	var framework map[string]any
	if err := json.Unmarshal(got.FrameworkFill, &framework); err != nil {
		t.Fatalf("unmarshal framework_fill: %v", err)
	}
	if framework["strategy"] != "reveal_framework_after_completion" {
		t.Fatalf("framework_fill = %v, missing strategy", framework)
	}
}

func TestRefactor2CardsLoop_RecordDispositionRoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newTurnTestPool(t)
	q := sqlc.New(pool)
	store := agent.NewSqlcAgentStore(q, pool)

	project, err := q.CreateProject(ctx, sqlc.CreateProjectParams{
		UserID: seededStudentID, Qualification: "EE", Title: "中国是否让地球变得更可持续？", BoardCfgVer: 1,
	})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	criterion := "D5"
	ivn, err := q.InsertIntervention(ctx, sqlc.InsertInterventionParams{
		ProjectID: project.ID, Type: "question",
		Anchor:    []byte(`{"kind":"graph_node","id":"` + project.ID.String() + `"}`),
		Criterion: &criterion, Body: "这条主张现在还没有素材支撑——它的证据是什么？",
	})
	if err != nil {
		t.Fatalf("InsertIntervention: %v", err)
	}

	deps := agent.AgentDeps{Store: store}
	if err := agent.RecordDisposition(ctx, deps, ivn.ID, "reject", "太短"); err == nil {
		t.Fatal("want an error for a <15-char reason")
	}

	reason := "这条追问和我原本的方向不一致，我想先按自己的思路推进"
	if err := agent.RecordDisposition(ctx, deps, ivn.ID, "reject", reason); err != nil {
		t.Fatalf("RecordDisposition: %v", err)
	}
}

// TestRefactor2CardsLoop_CreateCardInstanceOnTasklessMaterial — the
// regression this migration exists for. Slice 6b's project-scoped source-log
// ingestion creates materials with NO task_id (the task surface was deleted
// in 5d); when the coach later surfaces a card on that material,
// sqlcAgentStore.CreateCardInstance resolves task_id from the material row
// and must be able to pass a NULL through rather than narrowing it to the
// zero UUID (which does not exist in `tasks` and trips the FK). Before the
// fix (card_instances.task_id NOT NULL + zero-UUID narrowing), this failed
// with a foreign-key violation.
func TestRefactor2CardsLoop_CreateCardInstanceOnTasklessMaterial(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newTurnTestPool(t)
	q := sqlc.New(pool)
	store := agent.NewSqlcAgentStore(q, pool)

	project, err := q.CreateProject(ctx, sqlc.CreateProjectParams{
		UserID: seededStudentID, Qualification: "EE", Title: "中国是否让地球变得更可持续？", BoardCfgVer: 1,
	})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	// No TaskID set — pgtype.UUID{} zero value is Valid:false, i.e. NULL.
	material, err := q.CreateProjectMaterial(ctx, sqlc.CreateProjectMaterialParams{
		ProjectID: pgtype.UUID{Bytes: project.ID, Valid: true},
		Kind:      "article",
		Source:    "pasted",
		Title:     "student-pasted source, no task anywhere in sight",
		Blocks:    []byte(`[]`),
	})
	if err != nil {
		t.Fatalf("CreateProjectMaterial: %v", err)
	}
	if material.TaskID.Valid {
		t.Fatalf("test setup: material.TaskID = %+v, want NULL", material.TaskID)
	}

	row, err := store.CreateCardInstance(ctx, project.ID, material.ID, "craap", "evaluate_sources")
	if err != nil {
		t.Fatalf("CreateCardInstance on a taskless material: %v", err)
	}

	got, err := q.GetCardInstance(ctx, row.ID)
	if err != nil {
		t.Fatalf("GetCardInstance: %v", err)
	}
	if got.TaskID.Valid {
		t.Fatalf("card_instances.task_id = %+v, want NULL (inherited from the taskless material)", got.TaskID)
	}
}

// TestRefactor2CardsLoop_ToulminBuildsArgument is Slice 7's load-bearing
// end-to-end pass: it crosses the exact summon→submit→mint→gate seam the
// previous slice shipped broken. A project seeded at S4 (a source evaluated,
// no argument started) must surface the PROJECT-scoped toulmin card
// (materialID == "" — never the zero uuid, never an error), accept a complete
// argument-graph submit, mint the five Toulmin nodes plus a supports and a
// cites edge, and drive the build_argument gate's machine tier to clear.
//
// The seed reproduces the REAL post-CRAAP/SIFT graph: a real CRAAP `promote`
// mint leaves an ORPHAN evidence node (the evaluated source, not yet wired into
// any argument) plus an `evaluated-as` edge to it, and SIFT leaves a
// `cross-checked-by` edge. So the seed writes an actual orphan `evidence`
// graph_node + the two edges the classifier reads to conclude "this source is
// spoken for by both CRAAP and SIFT, and no argument exists yet". This is the
// point of the strengthened test: build_argument's machine tier must clear
// DESPITE the orphan CRAAP evidence — which it does because no_orphan_evidence
// was dropped from S4 (it is unsatisfiable in the real flow). The
// cross-checked-by target is dangling (graph_edge has no FK to graph_node); it
// only needs to exist as a fact so SIFT does not preempt toulmin as cands[0].
func TestRefactor2CardsLoop_ToulminBuildsArgument(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newTurnTestPool(t)
	q := sqlc.New(pool)
	store := agent.NewSqlcAgentStore(q, pool)

	task, err := q.CreateTask(ctx, sqlc.CreateTaskParams{UserID: seededStudentID, Title: "refactor2-toulmin"})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	project, err := q.CreateProject(ctx, sqlc.CreateProjectParams{
		UserID: seededStudentID, Qualification: "EE", Title: "中国是否让地球变得更可持续？", BoardCfgVer: 1,
	})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	material, err := q.CreateProjectMaterial(ctx, sqlc.CreateProjectMaterialParams{
		TaskID:    pgtype.UUID{Bytes: task.ID, Valid: true},
		ProjectID: pgtype.UUID{Bytes: project.ID, Valid: true},
		Kind:      "article",
		Source:    "fetched",
		Title:     "NASA: China's renewable build-out",
		Blocks:    []byte(`[]`),
	})
	if err != nil {
		t.Fatalf("CreateProjectMaterial: %v", err)
	}

	// Seed S4 precondition, production-faithful: the source is evaluated (CRAAP
	// done) AND already cross-checked (SIFT done), so neither re-surfaces and
	// toulmin is the sole candidate. CRAAP's `promote` leaves a REAL orphan
	// evidence node (no outgoing supports edge) + an evaluated-as edge to it.
	craapEvidenceID, err := store.InsertGraphNode(ctx, project.ID, agent.MintNode{
		Type:   "evidence",
		Author: "student",
		Body:   map[string]any{"source_quality": map[string]any{"authority": "NASA地球观测团队发布，具备权威性"}},
	})
	if err != nil {
		t.Fatalf("seed CRAAP evidence node: %v", err)
	}
	if err := store.InsertGraphEdge(ctx, project.ID, agent.MintEdge{
		Type: "evaluated-as", FromKind: "material", FromID: material.ID.String(),
		ToKind: "graph_node", ToID: craapEvidenceID.String(),
	}); err != nil {
		t.Fatalf("seed evaluated-as edge: %v", err)
	}
	if err := store.InsertGraphEdge(ctx, project.ID, agent.MintEdge{
		Type: "cross-checked-by", FromKind: "material", FromID: material.ID.String(),
		ToKind: "graph_node", ToID: uuid.NewString(),
	}); err != nil {
		t.Fatalf("seed cross-checked-by edge: %v", err)
	}
	// N3a: the project-scoped perspective-matrix surface sits AHEAD of toulmin
	// (classifier.go) — the skill's station chain puts evaluate_perspectives
	// strictly upstream of the argument work. So the S4 precondition now also
	// includes evaluate_perspectives being satisfied: two student-authored
	// `perspective` nodes, exactly as perspective-matrix's `perspectives`
	// graph_effect mints them. Without this the perspective card, not toulmin,
	// would be cands[0] — which is the correct production behavior, and this
	// test is about the toulmin hop.
	for _, who := range []string{"地方政府", "受影响居民"} {
		if _, err := store.InsertGraphNode(ctx, project.ID, agent.MintNode{
			Type:   "perspective",
			Author: "student",
			Body:   map[string]any{"text": who},
		}); err != nil {
			t.Fatalf("seed perspective node %s: %v", who, err)
		}
	}

	// Sanity: build_argument's machine tier is NOT yet clear (no concession node).
	sk, ok := skills.ByID("writing-project")
	if !ok {
		t.Fatal("skills.ByID(writing-project) not found")
	}
	if before := agent.CheckGate(sk, "build_argument", loadGraphView(ctx, t, q, project.ID), agent.RecordedGate{}); before.Status == "machine_clear" {
		t.Fatalf("build_argument should not be machine_clear before the mint, got %q", before.Status)
	}

	// Act 1 — Summon: RunAgentStep must surface toulmin, project-scoped, with
	// an EMPTY MaterialID (the whole point: not the zero uuid, not an error).
	prov := gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: "should never be called"},
		{Kind: gateway.EventDone, StopReason: gateway.StopStop},
	})
	deps := agent.AgentDeps{Store: store, Provider: prov, Resolved: gateway.Resolved{Provider: "deepseek", Model: "deepseek-chat", Tier: "coach"}}

	action, err := agent.RunAgentStep(ctx, deps, project.ID, agent.Trigger{Kind: "T-B"})
	if err != nil {
		t.Fatalf("RunAgentStep: %v", err)
	}
	if action == nil || action.Kind != "surface_card" {
		t.Fatalf("want a surface_card action, got %+v", action)
	}
	if action.CardID != "toulmin" {
		t.Fatalf("surfaced card = %q, want toulmin (CRAAP/SIFT suppressed, toulmin is cands[0])", action.CardID)
	}
	if action.MaterialID != "" {
		t.Fatalf("toulmin MaterialID = %q, want empty (project-scoped)", action.MaterialID)
	}
	cardInstanceID, err := uuid.Parse(action.CardInstanceID)
	if err != nil {
		t.Fatalf("parse CardInstanceID: %v", err)
	}

	// Surfacing a project-scoped card mints NO evaluates edge — only the two
	// seed edges exist.
	edges, err := q.ListGraphEdgesByProject(ctx, project.ID)
	if err != nil {
		t.Fatalf("ListGraphEdgesByProject: %v", err)
	}
	for _, e := range edges {
		if e.Type == "evaluates" {
			t.Fatalf("project-scoped toulmin must not mint an evaluates edge, got %+v", e)
		}
	}

	// Act 2 — Submit: a COMPLETE Toulmin graph. Each slot gets a text anchor
	// (≥12 runes); the three needSrc slots (warrant/evidence/concession) also
	// get a source anchor citing the real seeded material.
	matID := material.ID.String()
	anchors := []agent.Anchor{
		{ID: "s-claim", Dimension: "claim", Author: "student", Answer: "中国的可再生能源建设正在让地球变得更可持续"},
		{ID: "s-warrant-t", Dimension: "warrant", Author: "student", Answer: "大规模光伏与风电的装机数据能直接支撑这一主张"},
		{ID: "s-warrant-src", Dimension: "warrant", Author: "student", MaterialID: matID},
		{ID: "s-evidence-t", Dimension: "evidence", Author: "student", Answer: "NASA地球观测数据显示中国绿化与减排的成效显著"},
		{ID: "s-evidence-src", Dimension: "evidence", Author: "student", MaterialID: matID},
		{ID: "s-counter", Dimension: "counter", Author: "student", Answer: "反方指出中国仍是全球最大的碳排放国这一硬事实"},
		{ID: "s-concession-t", Dimension: "concession", Author: "student", Answer: "承认碳排放总量第一但人均与增速指标正在快速改善"},
		{ID: "s-concession-src", Dimension: "concession", Author: "student", MaterialID: matID},
	}
	anchorsJSON, err := json.Marshal(anchors)
	if err != nil {
		t.Fatalf("marshal anchors: %v", err)
	}
	if _, err := q.SetCardInstanceAnchors(ctx, sqlc.SetCardInstanceAnchorsParams{
		ID:        cardInstanceID,
		ProjectID: pgtype.UUID{Bytes: project.ID, Valid: true},
		Anchors:   anchorsJSON,
	}); err != nil {
		t.Fatalf("SetCardInstanceAnchors: %v", err)
	}

	spec, ok := cards.ByID("toulmin")
	if !ok {
		t.Fatal("cards.ByID(toulmin) not found — Task 1's registry config is missing")
	}
	complete, err := agent.CompleteCard(ctx, deps, spec, cardInstanceID)
	if err != nil {
		t.Fatalf("CompleteCard: %v", err)
	}
	if !complete {
		t.Fatal("want complete = true for a full Toulmin graph")
	}

	// Assert the mint. The toulmin-only slot types (claim/warrant/counter/
	// concession) get exactly one student-authored node each. `evidence` is the
	// exception: the toulmin mint adds ONE, and the seeded orphan CRAAP evidence
	// is still present, so there are TWO — proving the mint/gate hold with the
	// orphan present. We identify the toulmin evidence node specifically as the
	// one carrying the supports->claim edge (below), not by a raw type count.
	nodes, err := q.ListGraphNodesByProject(ctx, project.ID)
	if err != nil {
		t.Fatalf("ListGraphNodesByProject: %v", err)
	}
	byType := map[string][]sqlc.GraphNode{}
	for _, n := range nodes {
		byType[n.Type] = append(byType[n.Type], n)
	}
	for _, typ := range []string{"claim", "warrant", "counter", "concession"} {
		if len(byType[typ]) != 1 {
			t.Fatalf("node %s count = %d, want 1", typ, len(byType[typ]))
		}
		if byType[typ][0].Author != "student" {
			t.Fatalf("node %s author = %q, want student", typ, byType[typ][0].Author)
		}
	}
	if len(byType["evidence"]) != 2 {
		t.Fatalf("evidence node count = %d, want 2 (seeded orphan CRAAP evidence + toulmin's)", len(byType["evidence"]))
	}
	evidenceIDs := map[uuid.UUID]bool{}
	for _, n := range byType["evidence"] {
		evidenceIDs[n.ID] = true
	}

	// supports: exactly one evidence -> claim edge (the toulmin evidence; the
	// orphan CRAAP evidence has no supports edge, by construction).
	claimID := byType["claim"][0].ID
	edges, err = q.ListGraphEdgesByProject(ctx, project.ID)
	if err != nil {
		t.Fatalf("ListGraphEdgesByProject: %v", err)
	}
	supports, cites := 0, 0
	var toulminEvidenceID uuid.UUID
	for _, e := range edges {
		if e.Type == "supports" && e.FromKind == "graph_node" && evidenceIDs[e.FromID] && e.ToKind == "graph_node" && e.ToID == claimID {
			supports++
			toulminEvidenceID = e.FromID
		}
		if e.Type == "cites" && e.FromKind == "graph_node" && e.ToKind == "material" && e.ToID == material.ID {
			cites++
		}
	}
	if supports != 1 {
		t.Fatalf("want 1 supports edge evidence->claim, got %d among %+v", supports, edges)
	}
	if cites == 0 {
		t.Fatalf("want ≥1 cites edge to the seeded material, got 0 among %+v", edges)
	}
	// The toulmin evidence node (the one wired to the claim) is distinct from the
	// seeded orphan CRAAP evidence, which keeps NO supports edge.
	if toulminEvidenceID == craapEvidenceID {
		t.Fatalf("supports edge came from the orphan CRAAP evidence %s, not the toulmin evidence", craapEvidenceID)
	}

	// Assert the gate: build_argument's machine tier clears (claim supported,
	// concession present) DESPITE the orphan CRAAP evidence node — proving the
	// gate works in the real flow now that no_orphan_evidence was dropped from
	// S4. DEC-3 caps the machine Status at "machine_clear"; a "solid"/"done"
	// station additionally needs an external confirmation (Advance + recorded
	// student_written items) this mint does not — and must not — write.
	after := agent.CheckGate(sk, "build_argument", loadGraphView(ctx, t, q, project.ID), agent.RecordedGate{})
	if after.Status != "machine_clear" {
		t.Fatalf("build_argument gate = %q after the mint, want machine_clear (missing: %v)", after.Status, after.Missing)
	}

	// Slice 7b: the same minted rows must project into five done structure
	// cards — the completed argument shown back in the 结构 pane. Proves the
	// projection over a genuine mint, not hand-built nodes.
	pd, err := studio.Load(ctx, q, project.ID)
	if err != nil {
		t.Fatalf("studio.Load: %v", err)
	}
	proj, err := studio.Project(sk, cards.ByID, pd)
	if err != nil {
		t.Fatalf("studio.Project: %v", err)
	}
	if len(proj.Structure) != 5 {
		t.Fatalf("projected structure = %d cards, want 5", len(proj.Structure))
	}
	for _, c := range proj.Structure {
		if c.Status != "done" {
			t.Fatalf("structure card %s = %q, want done (argument fully minted)", c.ID, c.Status)
		}
		if c.Preview == "" {
			t.Fatalf("structure card %s has empty preview", c.ID)
		}
	}
}

// loadGraphView reconstructs the project's GraphView from its persisted rows
// for a gate reconciliation — the same nodes/edges/cards the studio projection
// reads. Materials are omitted: build_argument's machine predicates read only
// nodes and edges.
func loadGraphView(ctx context.Context, t *testing.T, q *sqlc.Queries, projectID uuid.UUID) agent.GraphView {
	t.Helper()
	nodes, err := q.ListGraphNodesByProject(ctx, projectID)
	if err != nil {
		t.Fatalf("ListGraphNodesByProject: %v", err)
	}
	edges, err := q.ListGraphEdgesByProject(ctx, projectID)
	if err != nil {
		t.Fatalf("ListGraphEdgesByProject: %v", err)
	}
	return agent.GraphViewFromRows(nodes, edges, nil, nil)
}

// TestRefactor2CardsLoop_SemanticMomentToRefeed is Task 8's whole-chain
// proof: it walks N3b's two seams — the semantic classifier (Seam A) and the
// completed-card refeed (Seam B) — back to back, over the real testcontainers
// stack, through the exact store calls the HTTP handlers use
// (projectcards.go's card-submit path: SetCardInstanceAnchors ->
// SubmitProjectCardInstance -> CompleteCard -> SetCardInstanceStatus ->
// RunAgentStep(card_refeed)). Every assertion below reads back real database
// rows, never a fixture.
//
// This file's package is `agent_test` (black-box), not `agent`: the
// project's own countingProvider/capturingProvider test doubles
// (internal/agent/loop_test.go) live in the internal `agent` test package and
// are unexported, so they are not visible here. gateway.StubProvider already
// exports the equivalent capability (LastRequest, the ChatRequest the engine
// actually built), so this test uses that directly rather than redeclaring a
// capturing provider.
func TestRefactor2CardsLoop_SemanticMomentToRefeed(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newTurnTestPool(t)
	q := sqlc.New(pool)
	store := agent.NewSqlcAgentStore(q, pool)
	resolved := gateway.Resolved{Provider: "deepseek", Model: "deepseek-chat", Tier: "coach"}

	// Step 1: a project with NO material at all — the per-material and
	// project-scoped structural surface_card branches (classifier.go) all
	// require a material or an "evaluated-as" edge to fire, so this project
	// yields no structural surface_card candidate, ever. The semantic
	// classifier (Seam A) is therefore the ONLY thing that can produce a
	// surface_card candidate here.
	project, err := q.CreateProject(ctx, sqlc.CreateProjectParams{
		UserID: seededStudentID, Qualification: "EE", Title: "核电是否应该取代化石能源？", BoardCfgVer: 1,
	})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	// A real one-sided paragraph: argues from one side only, dismisses the
	// opposing case outright — exactly momentCard[MomentOneSided]'s Desc.
	studentText := "核电就是绝对清洁又绝对安全的能源，我们现在就应该把所有化石能源都换成核电。" +
		"反对核电的人根本没有认真研究过数据，他们的担心毫无根据，完全不值得认真对待。"

	// Step 2: RunAgentStep with Trigger{Kind:"student_turn"} and a stubbed
	// classifier reply of "one_sided".
	classifyProv := gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: "one_sided"},
		{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 20, OutputTokens: 2}},
		{Kind: gateway.EventDone, StopReason: gateway.StopStop},
	})
	deps1 := agent.AgentDeps{Store: store, Provider: classifyProv, Resolved: resolved}

	action1, err := agent.RunAgentStep(ctx, deps1, project.ID, agent.Trigger{Kind: "student_turn", StudentText: studentText})
	if err != nil {
		t.Fatalf("RunAgentStep (student_turn): %v", err)
	}
	if classifyProv.LastRequest.Messages == nil {
		t.Fatal("want the classifier to have actually been called (LastRequest unset)")
	}
	if action1 == nil || action1.Kind != "surface_card" {
		t.Fatalf("want a surface_card action from the named moment, got %+v", action1)
	}
	if action1.CardID != "concession" {
		t.Fatalf("surfaced card = %q, want concession (the one_sided moment's card)", action1.CardID)
	}
	if action1.MaterialID != "" {
		t.Fatalf("concession MaterialID = %q, want empty (project-scoped, per momentCard)", action1.MaterialID)
	}
	cardInstanceID, err := uuid.Parse(action1.CardInstanceID)
	if err != nil {
		t.Fatalf("parse CardInstanceID: %v", err)
	}

	// Step 3: assert a REAL card_instance row for "concession" exists, proposed.
	list, err := q.ListCardInstancesByProject(ctx, pgtype.UUID{Bytes: project.ID, Valid: true})
	if err != nil {
		t.Fatalf("ListCardInstancesByProject: %v", err)
	}
	if len(list) != 1 || list[0].ID != cardInstanceID {
		t.Fatalf("ListCardInstancesByProject = %+v, want 1 row matching %s", list, cardInstanceID)
	}
	if list[0].CardID != "concession" || list[0].Status != "proposed" {
		t.Fatalf("unexpected card_instance: card_id=%q status=%q", list[0].CardID, list[0].Status)
	}

	// Step 4: submit it completed with real field_values, through the exact
	// store-call sequence projectcards.go's handler runs: SetCardInstanceAnchors
	// -> SubmitProjectCardInstance -> GetCardInstance -> CompleteCard ->
	// SetCardInstanceStatus("completed"). concession is a field-only card (no
	// "completion" predicates, no "graph_effects" in its spec JSON), so its
	// anchors are empty and its real content lives entirely in field_values,
	// keyed step-key -> field-key, exactly as SerializeCardForRefeed
	// (refeed.go) reads it back.
	thesis := "大规模推广核电是让能源结构变得可持续的关键一步"
	counterStrongest := "反方最强点是：福岛与切尔诺贝利证明重大核事故一旦发生，后果不可逆、代价极其巨大，选址与监管一旦失误便无法挽回。"
	concede := "我承认严重核事故一旦发生，后果确实可能不可逆、代价巨大，这一点不能回避。"
	rebuttal := "但只要采用第三代反应堆设计并强化独立监管，事故概率可以降到极低水平，而气候变化的长期代价更加确定且同样不可逆。"
	fieldValues := map[string]map[string]any{
		"concession": {"thesis": thesis, "counter": counterStrongest, "concede": concede, "rebut": rebuttal},
	}
	fieldValuesJSON, err := json.Marshal(fieldValues)
	if err != nil {
		t.Fatalf("marshal field values: %v", err)
	}

	if err := store.SetCardInstanceAnchors(ctx, project.ID, cardInstanceID, []byte(`[]`)); err != nil {
		t.Fatalf("SetCardInstanceAnchors: %v", err)
	}
	if err := store.SubmitProjectCardInstance(ctx, project.ID, cardInstanceID, fieldValuesJSON, []byte(`[]`)); err != nil {
		t.Fatalf("SubmitProjectCardInstance: %v", err)
	}

	spec, ok := cards.ByID("concession")
	if !ok {
		t.Fatal("cards.ByID(concession) not found")
	}
	deps2 := agent.AgentDeps{Store: store}
	complete, err := agent.CompleteCard(ctx, deps2, spec, cardInstanceID)
	if err != nil {
		t.Fatalf("CompleteCard: %v", err)
	}
	if !complete {
		t.Fatal("want complete = true for a fully filled concession card")
	}
	if err := store.SetCardInstanceStatus(ctx, project.ID, cardInstanceID, "completed"); err != nil {
		t.Fatalf("SetCardInstanceStatus: %v", err)
	}

	// Real-row confirmation that the submit actually landed: status flipped to
	// completed, and field_values round-trips the exact text the student wrote.
	completedRow, err := q.GetCardInstance(ctx, cardInstanceID)
	if err != nil {
		t.Fatalf("GetCardInstance: %v", err)
	}
	if completedRow.Status != "completed" {
		t.Fatalf("Status = %q, want completed", completedRow.Status)
	}
	var storedFields map[string]map[string]any
	if err := json.Unmarshal(completedRow.FieldValues, &storedFields); err != nil {
		t.Fatalf("unmarshal stored field_values: %v", err)
	}
	if storedFields["concession"]["concede"] != concede {
		t.Fatalf("stored field_values did not round-trip the concession text: %+v", storedFields["concession"])
	}

	// Step 5+6: RunAgentStep with Trigger{Kind:"card_refeed", CardInstanceID}.
	// Assert the intervention row persists, anchored to that card_instance, AND
	// that the prompt the provider received actually carried the submitted
	// field values (proving the serializer really reached the model over the
	// real DB round-trip, not just in the pure-function tests).
	coachBody := "你的让步段有没有正面回应「福岛事故后果不可逆」这一点，而不是绕开它去谈别的？"
	refeedProv := gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: coachBody},
		{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 30, OutputTokens: 20}},
		{Kind: gateway.EventDone, StopReason: gateway.StopStop},
	})
	deps3 := agent.AgentDeps{Store: store, Provider: refeedProv, Resolved: resolved}

	action2, err := agent.RunAgentStep(ctx, deps3, project.ID, agent.Trigger{Kind: "card_refeed", CardInstanceID: cardInstanceID.String()})
	if err != nil {
		t.Fatalf("RunAgentStep (card_refeed): %v", err)
	}
	if action2 == nil || action2.Kind != "intervention" {
		t.Fatalf("want an intervention action from the refeed, got %+v", action2)
	}
	if action2.Output.Anchor.Kind != "card_instance" || action2.Output.Anchor.ID != cardInstanceID.String() {
		t.Fatalf("want the intervention anchored to the card_instance, got %+v", action2.Output.Anchor)
	}

	var refeedPrompt string
	for _, m := range refeedProv.LastRequest.Messages {
		if m.Role == gateway.RoleUser {
			refeedPrompt = m.Content
		}
	}
	if refeedPrompt == "" {
		t.Fatal("want a non-empty user prompt sent to the coach")
	}
	if !strings.Contains(refeedPrompt, counterStrongest) {
		t.Fatalf("refeed prompt did not carry the submitted counter_strongest text:\n%s", refeedPrompt)
	}
	if !strings.Contains(refeedPrompt, concede) {
		t.Fatalf("refeed prompt did not carry the submitted concede text:\n%s", refeedPrompt)
	}

	// The intervention row itself, read back from the real table: anchored to
	// the card_instance via the anchor jsonb (InsertIntervention never sets
	// the separate card_instance_id FK column — see loop.go's InterventionRow
	// literal — the anchor blob is the real anchoring mechanism here).
	ivns, err := q.ListInterventionsByProject(ctx, project.ID)
	if err != nil {
		t.Fatalf("ListInterventionsByProject: %v", err)
	}
	if len(ivns) != 1 {
		t.Fatalf("want exactly 1 intervention row, got %d", len(ivns))
	}
	var anchor struct {
		Kind string `json:"kind"`
		ID   string `json:"id"`
	}
	if err := json.Unmarshal(ivns[0].Anchor, &anchor); err != nil {
		t.Fatalf("unmarshal intervention anchor: %v", err)
	}
	if anchor.Kind != "card_instance" || anchor.ID != cardInstanceID.String() {
		t.Fatalf("intervention anchor = %+v, want {card_instance %s}", anchor, cardInstanceID)
	}
	if ivns[0].Body != coachBody {
		t.Fatalf("intervention body = %q, want %q", ivns[0].Body, coachBody)
	}

	// Step 7: suppression. A second student_turn with the SAME text must NOT
	// mint a second concession instance. The completed concession card_instance
	// already retires the one_sided moment (EligibleMoments suppresses on ANY
	// status, including "completed"), but "fact_opinion" is still
	// eligible, so the classifier DOES run again — its answer just must not
	// win. Stub it to (wrongly) say "one_sided" again: EligibleMoments must
	// exclude it from the candidate set it hands ClassifyMoment, so even a
	// model that ignored its own instructions collapses to none. This is the
	// real suppression path (classifier.go/moment.go), not a mocked shortcut.
	secondProv := gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: "one_sided"},
		{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 20, OutputTokens: 2}},
		{Kind: gateway.EventDone, StopReason: gateway.StopStop},
	})
	deps4 := agent.AgentDeps{Store: store, Provider: secondProv, Resolved: resolved}

	action3, err := agent.RunAgentStep(ctx, deps4, project.ID, agent.Trigger{Kind: "student_turn", StudentText: studentText})
	if err != nil {
		t.Fatalf("RunAgentStep (2nd student_turn): %v", err)
	}
	if secondProv.LastRequest.Messages == nil {
		t.Fatal("want the classifier to have been called again (fact_opinion is still eligible)")
	}
	if action3 != nil {
		t.Fatalf("want silence — concession is suppressed in ANY status — got %+v", action3)
	}

	list2, err := q.ListCardInstancesByProject(ctx, pgtype.UUID{Bytes: project.ID, Valid: true})
	if err != nil {
		t.Fatalf("ListCardInstancesByProject (2nd pass): %v", err)
	}
	concessionCount := 0
	for _, row := range list2 {
		if row.CardID == "concession" {
			concessionCount++
		}
	}
	if len(list2) != 1 || concessionCount != 1 {
		t.Fatalf("want exactly 1 concession card_instance total after the 2nd student_turn, got %+v", list2)
	}
}
