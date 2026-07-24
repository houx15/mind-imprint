// Package teacher holds pure teacher-facing projections of the canonical
// assessment object (agent.Report). Badges are display summaries only — never
// a composite score (RL-5). Recomputed on every read; never persisted.
package teacher

import (
	"fmt"

	"mindimprint/api/internal/agent"
)

var depthRank = map[string]int{"L1": 1, "L2": 2, "L3": 3, "L4": 4}

// DBadge summarises the six depth levels as a min–max range (en-dash), a single
// level when they coincide, or "—" when no dimension carries a real level.
func DBadge(r agent.Report) string {
	min, max := 0, 0
	for _, d := range r.DepthAxis {
		rank, ok := depthRank[d.Level]
		if !ok {
			continue // NA / unknown → no evidence, skip
		}
		if min == 0 || rank < min {
			min = rank
		}
		if rank > max {
			max = rank
		}
	}
	if min == 0 {
		return "—"
	}
	if min == max {
		return fmt.Sprintf("L%d", min)
	}
	return fmt.Sprintf("L%d–L%d", min, max) // U+2013 en-dash
}

// ABadge is the mean of the autonomy levels for signals whose opportunity was
// actually supplied (机会供给先于判定: not_supplied is platform debt, excluded),
// to one decimal, or "—" when no signal was supplied.
func ABadge(r agent.Report) string {
	sum, n := 0, 0
	for _, a := range r.AutonomyAxis {
		if a.Opportunity == "not_supplied" {
			continue
		}
		sum += a.Level
		n++
	}
	if n == 0 {
		return "—"
	}
	return fmt.Sprintf("%.1f", float64(sum)/float64(n))
}
