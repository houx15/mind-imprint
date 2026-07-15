package studio

import (
	"encoding/json"
	"sort"
	"testing"
)

// The wire key set must match packages/contracts/src/studioState.ts (StudioProjection).
func TestStudioProjectionJSONKeys(t *testing.T) {
	p := StudioProjection{
		Project:       ProjectHeader{Title: "t", QualLabel: "0457 个人报告"},
		Stations:      []StationDTO{{Code: "S4", Name: "论证构建", View: "结构", State: "current", Gate: &GateDTO{Total: 7, Passed: 2}}},
		ActiveStation: "S4",
		Coach: CoachDTO{
			Anchor:    "论证图 · 治理决心主张",
			Messages:  []CoachMessageDTO{{Kind: "ai", Body: "b", Tag: "D5", Anchor: "论证图 · 治理决心主张"}},
			Equipment: []EquipCardDTO{{ID: "e1", Name: "钢人卡", Spont: "提示后", Meth: "concession", MaterialID: "m1"}},
		},
		Onboarding: OnboardingDTO{RestatePrompt: "r", RubricRows: []RubricRowDTO{{Official: "o", Plain: "p", Weak: true}}, PlanSteps: []string{"立题"}},
		Materials: []MaterialDTO{{
			ID: "m1", Title: "t", SourceURL: "https://x", Kind: "article", Origin: "fetched",
			Blocks: []MaterialBlockDTO{{ID: "b1", Text: "x"}}, Locked: true, Role: "r", Tier: "ti", Takeaway: "tk",
			Anchors: []json.RawMessage{json.RawMessage(`{"id":"a1"}`)}, TimeSpentS: 240, LateralRead: true, IsLateralInstrument: false,
			LateralRelation: "印证", LateralJudgment: "从二手转述降级为需要追源的说法",
		}},
		ActiveCard: &ActiveCardDTO{
			CardInstanceID: "ci1", CardID: "sift", Status: "active",
			Anchors: []json.RawMessage{json.RawMessage(`{"id":"a1"}`)}, MaterialID: "m1",
		},
		Structure: []StructureCardDTO{{ID: "claim", Role: "核心主张", Status: "done", Preview: "主张句"}},
		Writing: WritingDTO{
			Buffer: "b", CitationsMatched: true,
			LatestSnapshot: &WritingSnapshotDTO{ID: "s1", Seq: 1, CommittedAt: "2026-07-13T00:00:00Z", WordCount: 1800, InBand: true},
			WordBudget:     WordBudgetDTO{Min: 1500, Max: 2000},
			Review: WritingReviewDTO{Ordered: true, Items: []WritingReviewItemDTO{{
				InterventionID: "iv1", Criterion: "D 结构", Band: "达标", Evidence: "e", Missing: "m", Fix: "f",
				Disposition: &DispositionDTO{Action: "accept", Reason: "r"},
			}}},
		},
	}
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	top := marshalKeys(t, raw)
	want := []string{"activeCard", "activeStation", "coach", "materials", "onboarding", "project", "stations", "structure", "writing"}
	if !equalStrs(top, want) {
		t.Fatalf("top-level keys = %v, want %v", top, want)
	}

	var mTop map[string]json.RawMessage
	if err := json.Unmarshal(raw, &mTop); err != nil {
		t.Fatal(err)
	}
	var materials []json.RawMessage
	if err := json.Unmarshal(mTop["materials"], &materials); err != nil {
		t.Fatal(err)
	}
	if len(materials) != 1 {
		t.Fatalf("materials len = %d, want 1", len(materials))
	}
	// material: {id,title,sourceUrl,kind,origin,blocks,locked,role,tier,takeaway,anchors,timeSpentS,lateralRead,isLateralInstrument,siftSkipped,lateralRelation,lateralJudgment}
	// — must match packages/contracts/src/studioState.ts MaterialSource exactly.
	assertKeys(t, materials[0], []string{"anchors", "blocks", "id", "isLateralInstrument", "kind", "lateralJudgment", "lateralRead", "lateralRelation", "locked", "origin", "role", "siftSkipped", "sourceUrl", "takeaway", "tier", "timeSpentS", "title"})

	var structure []json.RawMessage
	if err := json.Unmarshal(mTop["structure"], &structure); err != nil {
		t.Fatal(err)
	}
	if len(structure) != 1 {
		t.Fatalf("structure len = %d, want 1", len(structure))
	}
	// structure card: {id,role,status,preview} — must match
	// packages/contracts/src/studioState.ts StructureCard exactly.
	assertKeys(t, structure[0], []string{"id", "preview", "role", "status"})

	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}

	// coach: {anchor, messages, equipment}
	assertKeys(t, m["coach"], []string{"anchor", "equipment", "messages"})

	var coach map[string]json.RawMessage
	if err := json.Unmarshal(m["coach"], &coach); err != nil {
		t.Fatal(err)
	}

	// coach.messages[0] is an "ai" message with tag+anchor: {kind,body,tag,anchor}
	var messages []json.RawMessage
	if err := json.Unmarshal(coach["messages"], &messages); err != nil {
		t.Fatal(err)
	}
	if len(messages) != 1 {
		t.Fatalf("coach.messages len = %d, want 1", len(messages))
	}
	assertKeys(t, messages[0], []string{"anchor", "body", "kind", "tag"})

	// coach.equipment[0]: {id,name,spont,meth,materialId}
	var equipment []json.RawMessage
	if err := json.Unmarshal(coach["equipment"], &equipment); err != nil {
		t.Fatal(err)
	}
	if len(equipment) != 1 {
		t.Fatalf("coach.equipment len = %d, want 1", len(equipment))
	}
	assertKeys(t, equipment[0], []string{"id", "materialId", "meth", "name", "spont"})

	// onboarding: {restatePrompt, rubricRows, planSteps}
	assertKeys(t, m["onboarding"], []string{"planSteps", "restatePrompt", "rubricRows"})

	var onboarding map[string]json.RawMessage
	if err := json.Unmarshal(m["onboarding"], &onboarding); err != nil {
		t.Fatal(err)
	}
	var rubricRows []json.RawMessage
	if err := json.Unmarshal(onboarding["rubricRows"], &rubricRows); err != nil {
		t.Fatal(err)
	}
	if len(rubricRows) != 1 {
		t.Fatalf("onboarding.rubricRows len = %d, want 1", len(rubricRows))
	}
	// rubric row: {official,plain,weak}
	assertKeys(t, rubricRows[0], []string{"official", "plain", "weak"})

	// station: {code,name,view,state,gate} (backflow omitted when false, gate present)
	var stations []json.RawMessage
	if err := json.Unmarshal(m["stations"], &stations); err != nil {
		t.Fatal(err)
	}
	if len(stations) != 1 {
		t.Fatalf("stations len = %d, want 1", len(stations))
	}
	assertKeys(t, stations[0], []string{"code", "gate", "name", "state", "view"})

	// activeCard: {cardInstanceId,cardId,status,anchors,materialId} — must
	// match packages/contracts/src/studioState.ts ActiveCard exactly.
	assertKeys(t, m["activeCard"], []string{"anchors", "cardId", "cardInstanceId", "materialId", "status"})

	// writing: {buffer,citationsMatched,latestSnapshot,review,wordBudget} —
	// must match packages/contracts/src/studioState.ts WritingProjection.
	assertKeys(t, m["writing"], []string{"buffer", "citationsMatched", "latestSnapshot", "review", "wordBudget"})

	var writing map[string]json.RawMessage
	if err := json.Unmarshal(m["writing"], &writing); err != nil {
		t.Fatal(err)
	}

	// writing.latestSnapshot: {id,seq,committedAt,wordCount,inBand}
	assertKeys(t, writing["latestSnapshot"], []string{"committedAt", "id", "inBand", "seq", "wordCount"})

	// writing.wordBudget: {min,max}
	assertKeys(t, writing["wordBudget"], []string{"max", "min"})

	var review map[string]json.RawMessage
	if err := json.Unmarshal(writing["review"], &review); err != nil {
		t.Fatal(err)
	}
	var reviewItems []json.RawMessage
	if err := json.Unmarshal(review["items"], &reviewItems); err != nil {
		t.Fatal(err)
	}
	if len(reviewItems) != 1 {
		t.Fatalf("writing.review.items len = %d, want 1", len(reviewItems))
	}
	// writing.review.items[0]: {interventionId,criterion,band,evidence,missing,fix,disposition}
	assertKeys(t, reviewItems[0], []string{"band", "criterion", "disposition", "evidence", "fix", "interventionId", "missing"})

	var reviewItem map[string]json.RawMessage
	if err := json.Unmarshal(reviewItems[0], &reviewItem); err != nil {
		t.Fatal(err)
	}
	// disposition: {action,reason}
	assertKeys(t, reviewItem["disposition"], []string{"action", "reason"})
}

// TestStudioProjectionActiveCardNil asserts the "no open card" case marshals
// the key as a literal JSON null, not an omitted key — the client's zod
// schema (`activeCard: ActiveCard.nullable()`) requires the key present.
func TestStudioProjectionActiveCardNil(t *testing.T) {
	p := StudioProjection{Materials: []MaterialDTO{}}
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	got, ok := m["activeCard"]
	if !ok {
		t.Fatal("activeCard key missing — want it present (as null) even when no card is open")
	}
	if string(got) != "null" {
		t.Fatalf("activeCard = %s, want the literal null", got)
	}
}

// TestMaterialDTOLateralNoteAlwaysPresent asserts lateralRelation/
// lateralJudgment are present (as "") even with no cross_check yet — this
// DTO's convention (matching role/tier/takeaway): a field with no producer
// yet is present-and-empty, never hidden behind an omitted/optional key.
func TestMaterialDTOLateralNoteAlwaysPresent(t *testing.T) {
	m := MaterialDTO{ID: "m1", Blocks: []MaterialBlockDTO{}, Anchors: []json.RawMessage{}}
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	var probe struct {
		LateralRelation *string `json:"lateralRelation"`
		LateralJudgment *string `json:"lateralJudgment"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		t.Fatal(err)
	}
	if probe.LateralRelation == nil || *probe.LateralRelation != "" {
		t.Fatalf("lateralRelation = %v, want present and \"\"", probe.LateralRelation)
	}
	if probe.LateralJudgment == nil || *probe.LateralJudgment != "" {
		t.Fatalf("lateralJudgment = %v, want present and \"\"", probe.LateralJudgment)
	}
}

// TestCoachMessageDTOKinds asserts each CoachMessageDTO union variant marshals
// to exactly its expected key set per packages/contracts/src/studioState.ts
// CoachMessage: student -> {kind,body}; ai -> {kind,body,tag?,anchor?};
// flag -> {kind,label,body}. Body is REQUIRED on every variant (never omitted,
// even when empty) — this is the regression this test guards against.
func TestCoachMessageDTOKinds(t *testing.T) {
	cases := []struct {
		name string
		msg  CoachMessageDTO
		want []string
	}{
		{
			name: "ai with tag and anchor",
			msg:  CoachMessageDTO{Kind: "ai", Body: "b", Tag: "D5", Anchor: "论证图 · 治理决心主张"},
			want: []string{"anchor", "body", "kind", "tag"},
		},
		{
			name: "ai with neither tag nor anchor",
			msg:  CoachMessageDTO{Kind: "ai", Body: "b"},
			want: []string{"body", "kind"},
		},
		{
			name: "student",
			msg:  CoachMessageDTO{Kind: "student", Body: "s"},
			want: []string{"body", "kind"},
		},
		{
			name: "flag",
			msg:  CoachMessageDTO{Kind: "flag", Label: "跳过", Body: "f"},
			want: []string{"body", "kind", "label"},
		},
		{
			name: "ai with empty body must still include body key",
			msg:  CoachMessageDTO{Kind: "ai", Body: ""},
			want: []string{"body", "kind"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			raw, err := json.Marshal(c.msg)
			if err != nil {
				t.Fatal(err)
			}
			got := marshalKeys(t, raw)
			if !equalStrs(got, c.want) {
				t.Fatalf("keys = %v, want %v (raw=%s)", got, c.want, raw)
			}
		})
	}
}

// TestStationDTOGateBackflowOmission asserts the omitempty negative case for
// gate/backflow: absent when nil/false, present when set.
func TestStationDTOGateBackflowOmission(t *testing.T) {
	t.Run("gate nil, backflow false: both keys omitted", func(t *testing.T) {
		s := StationDTO{Code: "S1", Name: "起点", View: "结构", State: "locked"}
		raw, err := json.Marshal(s)
		if err != nil {
			t.Fatal(err)
		}
		got := marshalKeys(t, raw)
		want := []string{"code", "name", "state", "view"}
		if !equalStrs(got, want) {
			t.Fatalf("keys = %v, want %v (raw=%s)", got, want, raw)
		}
	})

	t.Run("gate set, backflow true: both keys present", func(t *testing.T) {
		s := StationDTO{Code: "S4", Name: "论证构建", View: "结构", State: "current", Gate: &GateDTO{Total: 7, Passed: 2}, Backflow: true}
		raw, err := json.Marshal(s)
		if err != nil {
			t.Fatal(err)
		}
		got := marshalKeys(t, raw)
		want := []string{"backflow", "code", "gate", "name", "state", "view"}
		if !equalStrs(got, want) {
			t.Fatalf("keys = %v, want %v (raw=%s)", got, want, raw)
		}
	})
}

// marshalKeys unmarshals raw JSON into a map and returns its sorted key set.
func marshalKeys(t *testing.T, raw json.RawMessage) []string {
	t.Helper()
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("unmarshal into map failed: %v (raw=%s)", err, raw)
	}
	return keys(m)
}

// assertKeys unmarshals raw into a map and asserts its sorted key set equals want.
func assertKeys(t *testing.T, raw json.RawMessage, want []string) {
	t.Helper()
	got := marshalKeys(t, raw)
	if !equalStrs(got, want) {
		t.Fatalf("keys = %v, want %v (raw=%s)", got, want, raw)
	}
}

func keys(m map[string]json.RawMessage) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func equalStrs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
