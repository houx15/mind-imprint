package teacher_test

import (
	"testing"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/teacher"
)

func d(level string) agent.DepthDim { return agent.DepthDim{Level: level} }
func a(level int, opp string) agent.AutonomySignal { return agent.AutonomySignal{Level: level, Opportunity: opp} }

func TestDBadge(t *testing.T) {
	cases := []struct {
		name string
		in   []agent.DepthDim
		want string
	}{
		{"range", []agent.DepthDim{d("L3"), d("L4"), d("L3"), d("L3"), d("L4"), d("L3")}, "L3–L4"},
		{"single", []agent.DepthDim{d("L2"), d("L2")}, "L2"},
		{"na ignored", []agent.DepthDim{d("NA"), d("L1"), d("NA")}, "L1"},
		{"all na", []agent.DepthDim{d("NA"), d("NA")}, "—"},
		{"empty", nil, "—"},
		{"full spread", []agent.DepthDim{d("L1"), d("L4")}, "L1–L4"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := teacher.DBadge(agent.Report{DepthAxis: c.in}); got != c.want {
				t.Fatalf("DBadge = %q, want %q", got, c.want)
			}
		})
	}
}

func TestABadge(t *testing.T) {
	cases := []struct {
		name string
		in   []agent.AutonomySignal
		want string
	}{
		{"plain mean", []agent.AutonomySignal{a(4, "given_taken"), a(4, "given_taken"), a(4, "given_taken"), a(4, "given_taken"), a(5, "given_taken"), a(4, "given_not_taken")}, "4.2"},
		{"not_supplied excluded", []agent.AutonomySignal{a(4, "given_taken"), a(0, "not_supplied"), a(0, "not_supplied")}, "4.0"},
		{"all not_supplied", []agent.AutonomySignal{a(0, "not_supplied"), a(0, "not_supplied")}, "—"},
		{"empty", nil, "—"},
		{"half band", []agent.AutonomySignal{a(0, "given_not_taken"), a(1, "given_taken")}, "0.5"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := teacher.ABadge(agent.Report{AutonomyAxis: c.in}); got != c.want {
				t.Fatalf("ABadge = %q, want %q", got, c.want)
			}
		})
	}
}

func TestAMeanExcludesNotSupplied(t *testing.T) {
	r := agent.Report{AutonomyAxis: []agent.AutonomySignal{
		{Code: "A1", Level: 5, Opportunity: "given_taken"},
		{Code: "A2", Level: 4, Opportunity: "given_taken"},
		{Code: "A3", Level: 4, Opportunity: "given_not_taken"},
		{Code: "A4", Level: 4, Opportunity: "given_taken"},
		{Code: "A5", Level: 0, Opportunity: "not_supplied"},
		{Code: "A6", Level: 0, Opportunity: "not_supplied"},
	}}
	got, ok := teacher.AMean(r)
	if !ok || got != 4.25 {
		t.Fatalf("AMean = %v, %v; want 4.25, true", got, ok)
	}
}

func TestAMeanNoSuppliedSignal(t *testing.T) {
	r := agent.Report{AutonomyAxis: []agent.AutonomySignal{
		{Code: "A1", Level: 0, Opportunity: "not_supplied"},
	}}
	if _, ok := teacher.AMean(r); ok {
		t.Fatal("AMean ok = true; want false when nothing was supplied")
	}
}

func TestDLevelsMinMax(t *testing.T) {
	r := agent.Report{DepthAxis: []agent.DepthDim{
		{Code: "D1", Level: "L3"}, {Code: "D2", Level: "L4"},
		{Code: "D3", Level: "NA"}, {Code: "D4", Level: "L3"},
	}}
	min, max, ok := teacher.DLevels(r)
	if !ok || min != 3 || max != 4 {
		t.Fatalf("DLevels = %d,%d,%v; want 3,4,true", min, max, ok)
	}
}

func TestDLevelsUnrated(t *testing.T) {
	if _, _, ok := teacher.DLevels(agent.Report{DepthAxis: []agent.DepthDim{{Code: "D1", Level: "NA"}}}); ok {
		t.Fatal("DLevels ok = true; want false when no dim carries a level")
	}
}
