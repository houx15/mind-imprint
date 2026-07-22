package studio

// spotcheck_test.go — N3f Task 4 fix wave, finding M4. SpotCheckTargets is
// the shared anti-drift seam between the ordering handler and Task 5's
// `orderable` projection (see spotcheck.go's header comment): a table test
// here over a synthetic ProjectData is far cheaper than the testcontainers
// path and pins the one thing that must never drift — what counts as a
// target, and in what order.

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/store/sqlc"
)

// toulminSpecByID is a fixture specByID that returns the real toulmin slot
// order/roles without depending on the embedded card catalog — so this test
// stays a pure table test over synthetic data, per the brief.
func toulminSpecByID(id string) (cards.Spec, bool) {
	if id != "toulmin" {
		return cards.Spec{}, false
	}
	return cards.Spec{
		ID: "toulmin",
		Params: cards.Params{
			Slots: []cards.Slot{
				{ID: "claim", Role: "核心主张"},
				{ID: "warrant", Role: "理据 · 推理"},
				{ID: "evidence", Role: "支撑证据"},
				{ID: "counter", Role: "反方 · 钢人"},
				{ID: "concession", Role: "让步 · 转折"},
			},
		},
	}, true
}

func nodeBody(text string) []byte {
	return []byte(`{"text":"` + text + `"}`)
}

func TestSpotCheckTargets_UnknownStationIsEmpty(t *testing.T) {
	out := SpotCheckTargets(ProjectData{}, "not_a_real_station", toulminSpecByID)
	if len(out) != 0 {
		t.Fatalf("unknown station = %+v, want empty", out)
	}
}

// TestSpotCheckTargets_SourcesEmptyWithNoArticles: no article materials -> no
// targets, regardless of what else is in the graph.
func TestSpotCheckTargets_SourcesEmptyWithNoArticles(t *testing.T) {
	d := ProjectData{Materials: []sqlc.Material{{ID: uuid.New(), Kind: "note", Title: "笔记"}}}
	out := SpotCheckTargets(d, agent.SpotCheckSources, toulminSpecByID)
	if len(out) != 0 {
		t.Fatalf("sources with no articles = %+v, want empty", out)
	}
}

// TestSpotCheckTargets_SourcesOrderedByCreatedAtThenID covers the ordering
// contract exactly: primarily by created_at, and — the case that genuinely
// matters because seeded articles can share a created_at — tiebreaking by id
// when two materials share the exact same timestamp. Also covers the
// non-article filter and the evaluated/unevaluated Detail derivation in one
// pass.
func TestSpotCheckTargets_SourcesOrderedByCreatedAtThenID(t *testing.T) {
	t0 := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	// matLate sorts after matEarly by created_at alone.
	matEarly := uuid.MustParse("00000000-0000-0000-0000-0000000000e0")
	matLate := uuid.MustParse("00000000-0000-0000-0000-0000000000f0")
	// matTieHi/matTieLo share the EXACT same created_at as each other (and as
	// matLate) — id must break the tie, matTieHi ("...hi") > matTieLo ("...lo")... string compare on hex, so pick ids where the byte order is unambiguous.
	matTieA := uuid.MustParse("00000000-0000-0000-0000-0000000000aa")
	matTieB := uuid.MustParse("00000000-0000-0000-0000-0000000000bb")
	tShared := t0.Add(time.Hour)

	evidence := uuid.New()
	d := ProjectData{
		Materials: []sqlc.Material{
			{ID: matLate, Kind: "article", Title: "晚到的文章", CreatedAt: tShared.Add(time.Minute)},
			{ID: uuid.New(), Kind: "note", Title: "不是文章", CreatedAt: t0}, // filtered out
			{ID: matTieB, Kind: "article", Title: "并列 B", CreatedAt: tShared},
			{ID: matEarly, Kind: "article", Title: "最早的文章", CreatedAt: t0},
			{ID: matTieA, Kind: "article", Title: "并列 A", CreatedAt: tShared},
		},
		SourceLog: []sqlc.SourceLogEntry{
			{MaterialID: pgtype.UUID{Bytes: matEarly, Valid: true}, Takeaway: "讲了转型速度", Tier: strPtr("一手"), LateralRead: true},
		},
		Nodes: []sqlc.GraphNode{
			{ID: evidence, Type: "evidence", Body: []byte(`{"source_quality":{"risk_note":"只反映局部"}}`)},
		},
		Edges: []sqlc.GraphEdge{
			{Type: "evaluated-as", FromKind: "material", FromID: matEarly, ToKind: "node", ToID: evidence},
		},
	}
	out := SpotCheckTargets(d, agent.SpotCheckSources, toulminSpecByID)
	if len(out) != 4 {
		t.Fatalf("want 4 article targets (note filtered out), got %d: %+v", len(out), out)
	}
	gotIDs := make([]string, len(out))
	for i, tgt := range out {
		gotIDs[i] = tgt.ID
	}
	wantOrder := []string{matEarly.String(), matTieA.String(), matTieB.String(), matLate.String()}
	for i, want := range wantOrder {
		if gotIDs[i] != want {
			t.Fatalf("target[%d] = %s, want %s (full order: %v)", i, gotIDs[i], want, gotIDs)
		}
	}
	// The evaluated one carries its risk_note + tier + takeaway + lateral read.
	if out[0].Detail != "档位：一手；一句话收获：讲了转型速度；作用与风险：只反映局部；横向核查过" {
		t.Fatalf("evaluated detail = %q", out[0].Detail)
	}
	// The untouched ones read the explicit not-written placeholder + no lateral read.
	if out[1].Detail != "档位：；一句话收获：；作用与风险：「（未写）」；未横向核查" {
		t.Fatalf("unevaluated detail = %q", out[1].Detail)
	}
}

// TestSpotCheckTargets_ArgumentInSlotOrderSkippingBlankSlots: targets follow
// the card spec's slot order (not node/insertion order), and a slot with no
// node text is skipped entirely rather than emitted blank.
func TestSpotCheckTargets_ArgumentInSlotOrderSkippingBlankSlots(t *testing.T) {
	d := ProjectData{
		Nodes: []sqlc.GraphNode{
			// Inserted out of slot order: counter, then claim, then evidence.
			// warrant/concession are never written and must be skipped.
			{ID: uuid.New(), Type: "counter", Body: nodeBody("对方最硬的一张牌是执法资源有限")},
			{ID: uuid.New(), Type: "claim", Body: nodeBody("中国治理决心真实存在")},
			{ID: uuid.New(), Type: "evidence", Body: nodeBody("可再生能源投资全球第一")},
		},
	}
	out := SpotCheckTargets(d, agent.SpotCheckArgument, toulminSpecByID)
	if len(out) != 3 {
		t.Fatalf("want 3 targets (warrant/concession skipped), got %d: %+v", len(out), out)
	}
	wantIDs := []string{"claim", "evidence", "counter"} // slot order, not insertion order
	for i, want := range wantIDs {
		if out[i].ID != want {
			t.Fatalf("target[%d].ID = %q, want %q (full: %+v)", i, out[i].ID, want, out)
		}
	}
	// Name is the slot's Role label; Detail is the node's own text — distinct
	// strings, so a Name/Detail swap in the implementation would fail this.
	if out[0].Name != "核心主张" || out[0].Detail != "中国治理决心真实存在" {
		t.Fatalf("claim target = %+v", out[0])
	}
}

// TestSpotCheckTargets_ArgumentEmptyWhenNoSlotsWritten: a graph with no
// Toulmin slot text at all -> empty slice (not five blank targets).
func TestSpotCheckTargets_ArgumentEmptyWhenNoSlotsWritten(t *testing.T) {
	d := ProjectData{Nodes: []sqlc.GraphNode{{ID: uuid.New(), Type: "rubric_translation", Body: []byte(`{}`)}}}
	out := SpotCheckTargets(d, agent.SpotCheckArgument, toulminSpecByID)
	if len(out) != 0 {
		t.Fatalf("no slots written = %+v, want empty", out)
	}
}

// TestSpotCheckTargets_ArgumentEmptyWhenSpecMissingOrNoSlots covers the two
// defensive branches: specByID reporting the toulmin spec not found, and
// reporting it found but with zero slots.
func TestSpotCheckTargets_ArgumentEmptyWhenSpecMissingOrNoSlots(t *testing.T) {
	d := ProjectData{Nodes: []sqlc.GraphNode{{ID: uuid.New(), Type: "claim", Body: nodeBody("主张")}}}

	notFound := func(string) (cards.Spec, bool) { return cards.Spec{}, false }
	if out := SpotCheckTargets(d, agent.SpotCheckArgument, notFound); len(out) != 0 {
		t.Fatalf("spec not found = %+v, want empty", out)
	}

	noSlots := func(string) (cards.Spec, bool) { return cards.Spec{ID: "toulmin"}, true }
	if out := SpotCheckTargets(d, agent.SpotCheckArgument, noSlots); len(out) != 0 {
		t.Fatalf("spec with no slots = %+v, want empty", out)
	}
}
