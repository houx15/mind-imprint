package studio

import (
	"testing"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/store/sqlc"
)

func TestProjectFraming_ReadsAllFourNodeTypes(t *testing.T) {
	d := ProjectData{Nodes: []sqlc.GraphNode{
		{Type: "research_question", Body: []byte(`{"text":"中国是否让地球变得更可持续？"}`)},
		{Type: "term_definition", Body: []byte(`{"term":"可持续性","definition":"长期维持而不耗尽资源的能力","origin":"station_view"}`)},
		{Type: "provisional_answer", Body: []byte(`{"text":"是，因为可再生能源投资全球第一","origin":"station_view"}`)},
		{Type: "preregistration", Body: []byte(`{"directions":["查 IEA 年度报告","查 Nature Sustainability"],"origin":"station_view"}`)},
	}}
	fr := projectFraming(d)
	if fr.ResearchQuestion != "中国是否让地球变得更可持续？" {
		t.Errorf("ResearchQuestion = %q, want the research_question node's text", fr.ResearchQuestion)
	}
	if len(fr.Terms) != 1 || fr.Terms[0].Term != "可持续性" || fr.Terms[0].Definition != "长期维持而不耗尽资源的能力" {
		t.Errorf("Terms = %+v, want one row 可持续性/长期维持...", fr.Terms)
	}
	if len(fr.Answers) != 1 || fr.Answers[0] != "是，因为可再生能源投资全球第一" {
		t.Errorf("Answers = %v, want one row", fr.Answers)
	}
	if len(fr.SearchPlan) != 2 || fr.SearchPlan[0] != "查 IEA 年度报告" {
		t.Errorf("SearchPlan = %v, want the two directions in order", fr.SearchPlan)
	}
}

// TestProjectFraming_SkipsUnparseableBodies asserts a node whose body fails to
// unmarshal is skipped, never fabricated into a blank row — projectOnboarding's
// own defensive rule (projection.go).
func TestProjectFraming_SkipsUnparseableBodies(t *testing.T) {
	d := ProjectData{Nodes: []sqlc.GraphNode{
		{Type: "research_question", Body: []byte(`not json`)},
		{Type: "term_definition", Body: []byte(`{`)},
		{Type: "provisional_answer", Body: []byte(`null and garbage`)},
		{Type: "preregistration", Body: []byte(`[]`)}, // valid JSON, wrong shape: Directions stays nil
	}}
	fr := projectFraming(d)
	if fr.ResearchQuestion != "" {
		t.Errorf("ResearchQuestion = %q, want empty (unparseable body skipped)", fr.ResearchQuestion)
	}
	if len(fr.Terms) != 0 {
		t.Errorf("Terms = %+v, want empty (unparseable body skipped)", fr.Terms)
	}
	if len(fr.Answers) != 0 {
		t.Errorf("Answers = %v, want empty (unparseable body skipped)", fr.Answers)
	}
	if fr.Terms == nil {
		t.Error("Terms must be initialised non-nil so the JSON is [] not null")
	}
	if fr.Answers == nil {
		t.Error("Answers must be initialised non-nil so the JSON is [] not null")
	}
	if fr.SearchPlan == nil {
		t.Error("SearchPlan must be initialised non-nil so the JSON is [] not null")
	}
}

// TestProjectPerspectives_MarksCardMintedRowsReadOnly asserts a perspective
// node minted by the perspective-matrix tool card (agent/card_effects.go),
// body {"text","cells"} with NO origin key and NO level key, projects as a
// read-only row (level "", editable false) and still appears in the list —
// the graph does not care which surface asserted it.
func TestProjectPerspectives_MarksCardMintedRowsReadOnly(t *testing.T) {
	d := ProjectData{Nodes: []sqlc.GraphNode{
		// Station-view-written row: has origin + level, editable.
		{Type: "perspective", Body: []byte(`{"text":"国家视角：能源转型","level":"national","origin":"station_view"}`)},
		// Card-minted row: exactly card_effects.go's shape — no origin, no level.
		{Type: "perspective", Body: []byte(`{"text":"支持方：碳中和承诺","cells":{"claim":"支持","evidence":"NDC 承诺"}}`)},
	}}
	pv := projectPerspectives(d, map[string]agent.RecordedGate{})
	if len(pv.Rows) != 2 {
		t.Fatalf("Rows = %+v, want 2 (card-minted row must still appear)", pv.Rows)
	}
	station, card := pv.Rows[0], pv.Rows[1]
	if station.Level != "national" || !station.Editable {
		t.Errorf("station-view row = %+v, want level=national editable=true", station)
	}
	if card.Text != "支持方：碳中和承诺" {
		t.Errorf("card-minted row text = %q, want 支持方：碳中和承诺", card.Text)
	}
	if card.Level != "" {
		t.Errorf("card-minted row Level = %q, want \"\" (no level key in the mint)", card.Level)
	}
	if card.Editable {
		t.Error("card-minted row Editable = true, want false (no origin key in the mint)")
	}
}

// TestProjectPerspectives_ReadsRecordedAttestation asserts
// SourcesPerPerspective mirrors the RECORDED evaluate_perspectives gate item,
// the same way canFinish mirrors whole_draft_review (Project(), projection.go).
func TestProjectPerspectives_ReadsRecordedAttestation(t *testing.T) {
	solid := map[string]agent.RecordedGate{
		"evaluate_perspectives": {Items: map[string]string{"sources_per_perspective": "solid"}},
	}
	pv := projectPerspectives(ProjectData{}, solid)
	if !pv.SourcesPerPerspective {
		t.Error("SourcesPerPerspective = false, want true when the recorded gate item is solid")
	}

	notSolid := map[string]agent.RecordedGate{
		"evaluate_perspectives": {Items: map[string]string{"sources_per_perspective": "pending"}},
	}
	pv = projectPerspectives(ProjectData{}, notSolid)
	if pv.SourcesPerPerspective {
		t.Error("SourcesPerPerspective = true, want false when the recorded gate item is not solid")
	}

	pv = projectPerspectives(ProjectData{}, map[string]agent.RecordedGate{})
	if pv.SourcesPerPerspective {
		t.Error("SourcesPerPerspective = true, want false when the gate was never recorded")
	}
	if pv.Rows == nil {
		t.Error("Rows must be initialised non-nil so the JSON is [] not null")
	}
}
