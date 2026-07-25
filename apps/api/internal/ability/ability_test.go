package ability

import (
	"testing"
	"time"

	"mindimprint/api/internal/agent"
)

// depthReport builds a Report carrying only the given depth-dim levels (by
// code) — the ability aggregator reads only Code+Level off DepthAxis.
func depthReport(levels map[string]string) agent.Report {
	dims := make([]agent.DepthDim, 0, len(levels))
	for code, level := range levels {
		dims = append(dims, agent.DepthDim{Code: code, Level: level})
	}
	return agent.Report{DepthAxis: dims}
}

// sig builds an AutonomySignal with just the fields the aggregator reads.
func sig(code string, level int, opportunity string) agent.AutonomySignal {
	return agent.AutonomySignal{Code: code, Level: level, Opportunity: opportunity}
}

func at(day int) time.Time { return time.Date(2026, 7, day, 0, 0, 0, 0, time.UTC) }

func TestAggregateDepthRecencyWeightedAndLowNGuard(t *testing.T) {
	// D1 contributes [L1=1 (older), L3=3 (newer)] → weighted (0.6*1 + 1*3)/1.6 = 2.25 → level 2, evidence 2.
	// D3 contributes only [L2=2] once → below the 2-session guard → level -1.
	// D4 is "NA" both times → NA carries no evidence (never a low score) → level -1, evidence 0.
	// D5/D6 never appear → level -1, evidence 0.
	samples := []Sample{
		{Report: depthReport(map[string]string{"D1": "L1", "D3": "L2", "D4": "NA"}), CreatedAt: at(1)},
		{Report: depthReport(map[string]string{"D1": "L3", "D4": "NA"}), CreatedAt: at(2)},
	}
	m := Aggregate(samples)
	if m.TotalSessions != 2 {
		t.Fatalf("totalSessions = %d, want 2", m.TotalSessions)
	}
	if len(m.Depth) != 6 {
		t.Fatalf("depth dims = %d, want 6 (D1-D6 always)", len(m.Depth))
	}
	byCode := map[string]DepthAbility{}
	for _, d := range m.Depth {
		byCode[d.Code] = d
	}
	if d := byCode["D1"]; d.Level != 2 || d.EvidenceCount != 2 {
		t.Fatalf("D1 = level %d evidence %d, want level 2 evidence 2", d.Level, d.EvidenceCount)
	}
	if byCode["D1"].LevelLabel == "" {
		t.Fatalf("D1 level label empty, want the L2 anchor text")
	}
	if d := byCode["D3"]; d.Level != -1 || d.EvidenceCount != 1 {
		t.Fatalf("D3 = level %d evidence %d, want level -1 evidence 1 (low-N)", d.Level, d.EvidenceCount)
	}
	if d := byCode["D4"]; d.Level != -1 || d.EvidenceCount != 0 {
		t.Fatalf("D4 = level %d evidence %d, want level -1 evidence 0 (NA excluded)", d.Level, d.EvidenceCount)
	}
	if d := byCode["D5"]; d.Level != -1 || d.EvidenceCount != 0 {
		t.Fatalf("D5 = level %d evidence %d, want level -1 evidence 0 (never scored)", d.Level, d.EvidenceCount)
	}
}

func TestAggregateAutonomy(t *testing.T) {
	// A3 level sums into BoundarySettings; A4 into AdversaryInvites.
	// OpportunitiesTaken counts given_taken; OpportunitiesMissed counts given_not_taken.
	// not_supplied counts toward neither. Metacognition/SOLO is gone from the model.
	samples := []Sample{
		{
			Report: agent.Report{
				DepthAxis:    []agent.DepthDim{{Code: "D6", Level: "L3"}},
				AutonomyAxis: []agent.AutonomySignal{sig("A1", 1, "given_taken"), sig("A2", 1, "given_taken"), sig("A3", 2, "given_taken"), sig("A4", 0, "not_supplied")},
			},
			CreatedAt: at(1),
		},
		{
			Report: agent.Report{
				DepthAxis:    []agent.DepthDim{{Code: "D6", Level: "L3"}},
				AutonomyAxis: []agent.AutonomySignal{sig("A3", 1, "given_not_taken"), sig("A4", 0, "not_supplied")},
			},
			CreatedAt: at(2),
		},
		{
			Report: agent.Report{
				DepthAxis: []agent.DepthDim{{Code: "D6", Level: "L4"}},
			},
			CreatedAt: at(3),
		},
	}
	m := Aggregate(samples)
	if m.Autonomy.Sessions != 3 {
		t.Fatalf("autonomy sessions = %d, want 3", m.Autonomy.Sessions)
	}
	if m.Autonomy.BoundarySettings != 3 || m.Autonomy.AdversaryInvites != 0 {
		t.Fatalf("autonomy sums = %+v, want boundary 3 adversary 0", m.Autonomy)
	}
	if m.Autonomy.OpportunitiesTaken != 3 || m.Autonomy.OpportunitiesMissed != 1 {
		t.Fatalf("autonomy opportunities = taken %d missed %d, want 3/1", m.Autonomy.OpportunitiesTaken, m.Autonomy.OpportunitiesMissed)
	}
}

func TestAggregateEmpty(t *testing.T) {
	m := Aggregate(nil)
	if m.TotalSessions != 0 || len(m.Depth) != 6 {
		t.Fatalf("empty model = sessions %d depth %d, want 0 / 6", m.TotalSessions, len(m.Depth))
	}
	for _, d := range m.Depth {
		if d.Level != -1 {
			t.Fatalf("empty depth %s level %d, want -1", d.Code, d.Level)
		}
	}
}
