package ability

import (
	"testing"
	"time"

	"mindimprint/api/internal/agent"
)

func rep(depth map[string]int, boundary, adv int, anchored, prompted []string, solo []agent.SoloRow) agent.Report {
	dims := make([]agent.DepthDimScore, 0, len(depth))
	for code, sc := range depth {
		dims = append(dims, agent.DepthDimScore{Code: code, Score: sc})
	}
	return agent.Report{
		DepthAxis:    agent.DepthAxis{Dims: dims},
		AutonomyAxis: agent.AutonomyAxis{AdversaryInvites: adv, AnchoredSignals: anchored, PromptedSignals: prompted},
		PromptLens:   agent.PromptLens{BoundarySettings: boundary},
		Solo:         solo,
	}
}

func at(day int) time.Time { return time.Date(2026, 7, day, 0, 0, 0, 0, time.UTC) }

func TestAggregateDepthRecencyWeightedAndLowNGuard(t *testing.T) {
	// D1 contributes [1 (older), 3 (newer)] → weighted (0.6*1 + 1*3)/1.6 = 2.25 → level 2, evidence 2.
	// D3 contributes only [2] once → below the 2-session guard → level -1.
	// D4 scores 0 twice → 0 is not evidence → level -1, evidence 0.
	samples := []Sample{
		{Report: rep(map[string]int{"D1": 1, "D3": 2, "D4": 0}, 0, 0, nil, nil, nil), CreatedAt: at(1)},
		{Report: rep(map[string]int{"D1": 3, "D4": 0}, 0, 0, nil, nil, nil), CreatedAt: at(2)},
	}
	m := Aggregate(samples)
	if m.TotalSessions != 2 {
		t.Fatalf("totalSessions = %d, want 2", m.TotalSessions)
	}
	if len(m.Depth) != 4 {
		t.Fatalf("depth dims = %d, want 4 (D1/D3/D4/D5 always)", len(m.Depth))
	}
	byCode := map[string]DepthAbility{}
	for _, d := range m.Depth {
		byCode[d.Code] = d
	}
	if d := byCode["D1"]; d.Level != 2 || d.EvidenceCount != 2 {
		t.Fatalf("D1 = level %d evidence %d, want level 2 evidence 2", d.Level, d.EvidenceCount)
	}
	if byCode["D1"].LevelLabel == "" {
		t.Fatalf("D1 level label empty, want the score-2 anchor text")
	}
	if d := byCode["D3"]; d.Level != -1 || d.EvidenceCount != 1 {
		t.Fatalf("D3 = level %d evidence %d, want level -1 evidence 1 (low-N)", d.Level, d.EvidenceCount)
	}
	if d := byCode["D4"]; d.Level != -1 || d.EvidenceCount != 0 {
		t.Fatalf("D4 = level %d evidence %d, want level -1 evidence 0 (score 0 excluded)", d.Level, d.EvidenceCount)
	}
	if d := byCode["D5"]; d.Level != -1 || d.EvidenceCount != 0 {
		t.Fatalf("D5 = level %d evidence %d, want level -1 evidence 0 (never scored)", d.Level, d.EvidenceCount)
	}
}

func TestAggregateAutonomyAndMetacognition(t *testing.T) {
	samples := []Sample{
		{Report: rep(nil, 2, 0, []string{"R1"}, []string{"R3"},
			[]agent.SoloRow{{Level: "L3", Initiative: "自发"}, {Level: "L4", Initiative: "引导后"}}), CreatedAt: at(1)},
		{Report: rep(nil, 1, 0, []string{"R1", "R8"}, nil,
			[]agent.SoloRow{{Level: "L3", Initiative: "引导后"}}), CreatedAt: at(2)},
	}
	m := Aggregate(samples)
	if m.Autonomy.BoundarySettings != 3 || m.Autonomy.AdversaryInvites != 0 {
		t.Fatalf("autonomy sums = %+v, want boundary 3 adversary 0", m.Autonomy)
	}
	if m.Autonomy.AnchoredSignals != 3 || m.Autonomy.PromptedSignals != 1 {
		t.Fatalf("autonomy signals = anchored %d prompted %d, want 3/1", m.Autonomy.AnchoredSignals, m.Autonomy.PromptedSignals)
	}
	if m.Metacognition.HighestSolo != "L4" {
		t.Fatalf("highestSolo = %q, want L4", m.Metacognition.HighestSolo)
	}
	if m.Metacognition.Distribution["L3"] != 2 || m.Metacognition.Distribution["L4"] != 1 {
		t.Fatalf("distribution = %v, want L3:2 L4:1", m.Metacognition.Distribution)
	}
	if m.Metacognition.Spontaneous != 1 || m.Metacognition.Prompted != 2 {
		t.Fatalf("solo initiative = spont %d prompted %d, want 1/2", m.Metacognition.Spontaneous, m.Metacognition.Prompted)
	}
}

func TestAggregateEmpty(t *testing.T) {
	m := Aggregate(nil)
	if m.TotalSessions != 0 || len(m.Depth) != 4 {
		t.Fatalf("empty model = sessions %d depth %d, want 0 / 4", m.TotalSessions, len(m.Depth))
	}
	for _, d := range m.Depth {
		if d.Level != -1 {
			t.Fatalf("empty depth %s level %d, want -1", d.Code, d.Level)
		}
	}
	if m.Metacognition.Distribution == nil {
		t.Fatalf("distribution nil, want initialized empty map")
	}
}
