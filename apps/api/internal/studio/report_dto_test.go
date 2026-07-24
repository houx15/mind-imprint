package studio

import (
	"encoding/json"
	"testing"
	"time"

	"mindimprint/api/internal/agent"
)

func sampleReport() agent.Report {
	return agent.Report{
		DepthAxis: []agent.DepthDim{
			{Code: "D1", Name: "任务理解与问题表述", Level: "L3", Evidence: "x", PromptEvidence: "px"},
		},
		AutonomyAxis: []agent.AutonomySignal{
			{Code: "A1", Name: "主动提出问题", Level: 3, Opportunity: "given_taken", Evidence: "y", PromptEvidence: "py"},
		},
		PromptLens: agent.PromptLens{
			Stats:  []agent.LensStat{{Label: "l", Value: "v"}},
			Lenses: []agent.Lens{{Code: "P1", Name: "n", Level: 2, Evidence: "e"}},
			Note:   "note",
		},
		InteractionEvidence: []agent.InteractionRow{
			{Round: 1, Student: "s", AiSummary: "a", Signal: "sig"},
		},
		Narrative: "n",
		Guidance: agent.Guidance{
			NextSteps: []agent.NextStep{{Title: "t", Task: "task"}},
		},
		Axiom: "两轴永不合成总分；单次会话为事件级证据，不构成人级档位判定",
	}
}

func TestToReportDTOCarriesAllFieldsAndGeneratedAt(t *testing.T) {
	r := sampleReport()
	createdAt := time.Date(2026, 7, 19, 0, 0, 0, 0, time.UTC)
	dto := ToReportDTO(r, createdAt)

	if dto.GeneratedAt == "" {
		t.Fatalf("generatedAt is empty")
	}
	if dto.GeneratedAt != createdAt.Format(time.RFC3339) {
		t.Fatalf("generatedAt = %q, want %q", dto.GeneratedAt, createdAt.Format(time.RFC3339))
	}
	if len(dto.DepthAxis) != 1 || dto.DepthAxis[0].Code != "D1" {
		t.Fatalf("depth axis not carried: %+v", dto.DepthAxis)
	}
	if len(dto.AutonomyAxis) != 1 || dto.AutonomyAxis[0].Opportunity != "given_taken" {
		t.Fatalf("autonomy axis not carried: %+v", dto.AutonomyAxis)
	}
	if len(dto.PromptLens.Lenses) != 1 {
		t.Fatalf("promptLens not carried: %+v", dto.PromptLens)
	}
	if len(dto.InteractionEvidence) != 1 {
		t.Fatalf("interactionEvidence not carried: %+v", dto.InteractionEvidence)
	}
	if dto.Narrative != "n" || dto.Axiom != r.Axiom {
		t.Fatalf("narrative/axiom not carried: %+v", dto)
	}
	if len(dto.Guidance.NextSteps) != 1 {
		t.Fatalf("guidance not carried: %+v", dto.Guidance)
	}
	if dto.OfficialProjection != nil {
		t.Fatalf("officialProjection should be nil when Report has none")
	}

	b, err := json.Marshal(dto)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(b)
	for _, want := range []string{
		`"depthAxis"`, `"autonomyAxis"`, `"promptLens"`, `"interactionEvidence"`,
		`"generatedAt"`, `"axiom"`, `"opportunity":"given_taken"`,
	} {
		if !contains(s, want) {
			t.Fatalf("marshalled DTO missing %q: %s", want, s)
		}
	}
	for _, unwanted := range []string{`"subtotal"`, `"solo"`, `"officialProjection"`} {
		if contains(s, unwanted) {
			t.Fatalf("marshalled DTO must not contain %q: %s", unwanted, s)
		}
	}
}

func TestToReportDTOCarriesOfficialProjectionWhenPresent(t *testing.T) {
	r := sampleReport()
	r.OfficialProjection = &agent.OfficialProjection{
		Standard:  agent.OfficialStandardRef{ID: "ap-research", Name: "AP Research"},
		Readiness: agent.OfficialReadiness{Score: 70, Note: "note"},
	}
	dto := ToReportDTO(r, time.Now())
	if dto.OfficialProjection == nil {
		t.Fatalf("officialProjection should be carried when present")
	}
	b, _ := json.Marshal(dto)
	if !contains(string(b), `"officialProjection"`) {
		t.Fatalf("marshalled DTO missing officialProjection when present: %s", b)
	}
}

func contains(s, sub string) bool { return len(s) >= len(sub) && (indexOf(s, sub) >= 0) }
func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
