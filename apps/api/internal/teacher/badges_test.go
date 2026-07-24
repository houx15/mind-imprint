package teacher

import (
	"testing"

	"mindimprint/api/internal/agent"
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
			if got := DBadge(agent.Report{DepthAxis: c.in}); got != c.want {
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
			if got := ABadge(agent.Report{AutonomyAxis: c.in}); got != c.want {
				t.Fatalf("ABadge = %q, want %q", got, c.want)
			}
		})
	}
}
