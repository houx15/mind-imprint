// Package ability projects a student's per-session DualAxis reports into a
// current-standing 能力素养 model. Pure — no store, no LLM (RL-5: the person-level
// view is a merge of many sessions' evidence, never a single-session 档位).
package ability

import (
	"math"
	"sort"
	"strconv"
	"time"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/rubric"
)

// decay is the per-session recency weight base: the newest contributing session
// weighs 1, each older one 0.6× the next.
const decay = 0.6

type Sample struct {
	Report    agent.Report
	CreatedAt time.Time
}

// DepthAbility is one scored depth dim's merged current standing. Level -1 means
// "证据不足 · 需更多任务" (fewer than 2 contributing sessions) — the axiom made literal.
type DepthAbility struct {
	Code          string `json:"code"`
	Name          string `json:"name"`
	Level         int    `json:"level"`
	LevelLabel    string `json:"levelLabel"`
	EvidenceCount int    `json:"evidenceCount"`
}

// AutonomyAbility is 智识自主 aggregated as observation counts — never a level.
// OpportunitiesTaken/Missed count A-signals by Opportunity (机会供给先于判定):
// given_taken = an opportunity offered and taken; given_not_taken = offered and
// not taken (a miss, not a low score). not_supplied counts toward neither.
type AutonomyAbility struct {
	Sessions            int `json:"sessions"`
	BoundarySettings    int `json:"boundarySettings"`
	AdversaryInvites    int `json:"adversaryInvites"`
	OpportunitiesTaken  int `json:"opportunitiesTaken"`
	OpportunitiesMissed int `json:"opportunitiesMissed"`
}

type Model struct {
	TotalSessions int             `json:"totalSessions"`
	Depth         []DepthAbility  `json:"depth"`
	Autonomy      AutonomyAbility `json:"autonomy"`
}

// levelToInt maps a depth dim's L1..L4 level to its ordinal 1..4. "NA" (no
// evidence) and any unrecognized value return ok=false — the caller must skip
// it, exactly as the old shape skipped a Score<1 (no evidence, never a low
// score).
func levelToInt(level string) (int, bool) {
	switch level {
	case "L1":
		return 1, true
	case "L2":
		return 2, true
	case "L3":
		return 3, true
	case "L4":
		return 4, true
	default:
		return 0, false
	}
}

// Aggregate merges the samples into a current-standing model. Defensive: sorts by
// CreatedAt ascending so recency weighting holds regardless of input order.
func Aggregate(samples []Sample) Model {
	sorted := make([]Sample, len(samples))
	copy(sorted, samples)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].CreatedAt.Before(sorted[j].CreatedAt) })

	m := Model{TotalSessions: len(sorted)}

	// Depth: one merged level per depth dim, in rubric order (always length 6).
	for _, dim := range rubric.DepthDims() {
		var scores []int // contributing (L1-L4, i.e. NOT NA) in oldest->newest order
		for _, s := range sorted {
			for _, d := range s.Report.DepthAxis {
				if d.Code != dim.ID {
					continue
				}
				if lvl, ok := levelToInt(d.Level); ok {
					scores = append(scores, lvl)
				}
			}
		}
		da := DepthAbility{Code: dim.ID, Name: dim.Name, EvidenceCount: len(scores), Level: -1}
		if len(scores) >= 2 {
			k := len(scores)
			var num, den float64
			for i, sc := range scores {
				w := math.Pow(decay, float64(k-1-i))
				num += w * float64(sc)
				den += w
			}
			da.Level = int(math.Round(num / den))
			da.LevelLabel = dim.Anchors["L"+strconv.Itoa(da.Level)]
		}
		m.Depth = append(m.Depth, da)
	}

	// Autonomy: A3 ("边界主权"-shaped signal) sums into BoundarySettings, A4
	// ("对抗与检验"-shaped signal) into AdversaryInvites; OpportunitiesTaken/
	// Missed count signals by Opportunity.
	for _, s := range sorted {
		m.Autonomy.Sessions++
		for _, a := range s.Report.AutonomyAxis {
			switch a.Code {
			case "A3":
				m.Autonomy.BoundarySettings += a.Level
			case "A4":
				m.Autonomy.AdversaryInvites += a.Level
			}
			switch a.Opportunity {
			case "given_taken":
				m.Autonomy.OpportunitiesTaken++
			case "given_not_taken":
				m.Autonomy.OpportunitiesMissed++
			}
		}
	}

	return m
}
