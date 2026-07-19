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
type AutonomyAbility struct {
	Sessions         int `json:"sessions"`
	BoundarySettings int `json:"boundarySettings"`
	AdversaryInvites int `json:"adversaryInvites"`
	AnchoredSignals  int `json:"anchoredSignals"`
	PromptedSignals  int `json:"promptedSignals"`
}

// Metacognition is 跨轴 SOLO aggregated as a distribution — never a single level.
type Metacognition struct {
	HighestSolo  string         `json:"highestSolo"`
	Distribution map[string]int `json:"distribution"`
	Spontaneous  int            `json:"spontaneous"`
	Prompted     int            `json:"prompted"`
}

type Model struct {
	TotalSessions int             `json:"totalSessions"`
	Depth         []DepthAbility  `json:"depth"`
	Autonomy      AutonomyAbility `json:"autonomy"`
	Metacognition Metacognition   `json:"metacognition"`
}

var soloLevels = []string{"L1", "L2", "L3", "L4"}

// Aggregate merges the samples into a current-standing model. Defensive: sorts by
// CreatedAt ascending so recency weighting holds regardless of input order.
func Aggregate(samples []Sample) Model {
	sorted := make([]Sample, len(samples))
	copy(sorted, samples)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].CreatedAt.Before(sorted[j].CreatedAt) })

	m := Model{TotalSessions: len(sorted)}
	m.Metacognition.Distribution = map[string]int{"L1": 0, "L2": 0, "L3": 0, "L4": 0}

	// Depth: one merged level per depth dim, in rubric order (always length 4).
	for _, dim := range rubric.DepthDims() {
		var scores []int // contributing (>=1) in oldest->newest order
		for _, s := range sorted {
			for _, d := range s.Report.DepthAxis.Dims {
				if d.Code == dim.ID && d.Score >= 1 {
					scores = append(scores, d.Score)
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
			da.LevelLabel = dim.Anchors[strconv.Itoa(da.Level)]
		}
		m.Depth = append(m.Depth, da)
	}

	// Autonomy + metacognition: sum/collect across all sessions.
	for _, s := range sorted {
		m.Autonomy.Sessions++
		m.Autonomy.BoundarySettings += s.Report.PromptLens.BoundarySettings
		m.Autonomy.AdversaryInvites += s.Report.AutonomyAxis.AdversaryInvites
		m.Autonomy.AnchoredSignals += len(s.Report.AutonomyAxis.AnchoredSignals)
		m.Autonomy.PromptedSignals += len(s.Report.AutonomyAxis.PromptedSignals)
		for _, row := range s.Report.Solo {
			if _, ok := m.Metacognition.Distribution[row.Level]; ok {
				m.Metacognition.Distribution[row.Level]++
			}
			if row.Initiative == "自发" {
				m.Metacognition.Spontaneous++
			} else {
				m.Metacognition.Prompted++
			}
		}
	}
	// highest SOLO present
	for i := len(soloLevels) - 1; i >= 0; i-- {
		if m.Metacognition.Distribution[soloLevels[i]] > 0 {
			m.Metacognition.HighestSolo = soloLevels[i]
			break
		}
	}
	return m
}
