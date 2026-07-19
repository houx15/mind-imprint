package studio

import (
	"encoding/json"
	"testing"

	"mindimprint/api/internal/agent"
)

func TestToReportDTOCarriesAxesAndGeneratedAt(t *testing.T) {
	r := agent.Report{
		DepthAxis: agent.DepthAxis{
			Dims:     []agent.DepthDimScore{{Code: "D1", Name: "任务理解与问题表述", Score: 3, Evidence: "x"}},
			Subtotal: 3,
		},
		AutonomyAxis: agent.AutonomyAxis{Code: "D2", Name: "学生主体性 / AI 依赖度", Observation: "o", AnchoredSignals: []string{}, PromptedSignals: []string{}},
		CrossAxis:    agent.CrossAxis{Code: "D6", Name: "元认知与反思", DepthLevel: "L3"},
		Narrative:    "n",
		Axiom:        "两轴永不合成总分；单次会话为事件级证据，不构成人级档位判定",
	}
	dto := ToReportDTO(r, "2026-07-19T00:00:00Z")
	if dto.GeneratedAt != "2026-07-19T00:00:00Z" {
		t.Fatalf("generatedAt = %q", dto.GeneratedAt)
	}
	if dto.DepthAxis.Subtotal != 3 || len(dto.DepthAxis.Dims) != 1 {
		t.Fatalf("depth axis not carried: %+v", dto.DepthAxis)
	}
	b, _ := json.Marshal(dto)
	s := string(b)
	for _, want := range []string{`"depthAxis"`, `"autonomyAxis"`, `"crossAxis"`, `"subtotal":3`, `"generatedAt"`, `"axiom"`} {
		if !contains(s, want) {
			t.Fatalf("marshalled DTO missing %q: %s", want, s)
		}
	}
	// autonomy must NOT carry a score field
	if contains(s, `"autonomyAxis":{"code":"D2","name":"学生主体性 / AI 依赖度","score"`) {
		t.Fatalf("autonomy axis leaked a score field")
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
