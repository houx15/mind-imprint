// Package teacher holds pure teacher-facing projections of the canonical
// assessment object (agent.Report). Badges are display summaries only — never
// a composite score (RL-5). Recomputed on every read; never persisted.
package teacher

import (
	"fmt"

	"mindimprint/api/internal/agent"
)

var depthRank = map[string]int{"L1": 1, "L2": 2, "L3": 3, "L4": 4}

// DLevels reports the min and max rated depth levels (1..4). ok is false when
// no dimension carries a real level (NA/unknown are skipped — no evidence, no
// reading). The single source for every depth-level derivation: DBadge renders
// it, and D2's class distribution buckets on max.
func DLevels(r agent.Report) (min, max int, ok bool) {
	for _, d := range r.DepthAxis {
		rank, found := depthRank[d.Level]
		if !found {
			continue
		}
		if min == 0 || rank < min {
			min = rank
		}
		if rank > max {
			max = rank
		}
	}
	return min, max, min != 0
}

// AMean is the mean autonomy level over signals whose opportunity was actually
// supplied (机会供给先于判定: not_supplied is platform debt, excluded). ok is
// false when no signal was supplied. The single source for every autonomy mean:
// ABadge formats it, and D2's class mean averages it.
func AMean(r agent.Report) (float64, bool) {
	sum, n := 0, 0
	for _, a := range r.AutonomyAxis {
		if a.Opportunity == "not_supplied" {
			continue
		}
		sum += a.Level
		n++
	}
	if n == 0 {
		return 0, false
	}
	return float64(sum) / float64(n), true
}

// DBadge summarises the six depth levels as a min–max range (en-dash), a single
// level when they coincide, or "—" when no dimension carries a real level.
func DBadge(r agent.Report) string {
	min, max, ok := DLevels(r)
	if !ok {
		return "—"
	}
	if min == max {
		return fmt.Sprintf("L%d", min)
	}
	return fmt.Sprintf("L%d–L%d", min, max) // U+2013 en-dash
}

// ABadge renders AMean to one decimal, or "—" when no signal was supplied.
func ABadge(r agent.Report) string {
	m, ok := AMean(r)
	if !ok {
		return "—"
	}
	return fmt.Sprintf("%.1f", m)
}
