package cards

import "math"

// Proficiency is the per-(student, card) mastery snapshot for the 工具卡图鉴.
// It is descriptive, never a grade or a rank (铁律②/RL-5): it summarizes what the
// student has actually done with the card. See the design spec §D3.
type Proficiency struct {
	Encountered bool `json:"encountered"` // colored (true) vs greyscale (false)
	Score       int  `json:"score"`       // 0..100
	Stars       int  `json:"stars"`       // 0 when not encountered, else 1..5
}

// proficiencySaturation is the number of genuine project-scope completions at
// which the "practice" axis is considered mastered (useComp = 1). A log curve to
// this ceiling gives diminishing returns — the 8th real use matters far less than
// the 2nd, and you cannot climb forever by grinding one card.
const proficiencySaturation = 8

// ComputeProficiency derives the snapshot from three real signals:
//   - projCompletions: genuine PROJECT-scope completions (gated on the card's
//     completion predicate) — the practice axis, log-saturating.
//   - surfaces: distinct surfaces the card was completed on ("project"/"course"/
//     "chat") — the transfer axis (breadth).
//   - courseLearned: the teaching course is finished, or the card was completed
//     in a course — the "learned the method" foundation.
//
// score = 100 * (0.35*course + 0.50*use + 0.15*breadth). A student is
// "encountered" (colored) if she has ANY completion or has finished the course;
// stars are 0 when not encountered, else 1 + floor(score/20) clamped to 1..5, so
// any real encounter is worth at least one star.
func ComputeProficiency(projCompletions int, surfaces []string, courseLearned bool) Proficiency {
	encountered := courseLearned || len(surfaces) > 0
	if !encountered {
		return Proficiency{Encountered: false, Score: 0, Stars: 0}
	}

	useComp := 0.0
	if projCompletions > 0 {
		useComp = math.Log2(1+float64(projCompletions)) / math.Log2(1+float64(proficiencySaturation))
		if useComp > 1 {
			useComp = 1
		}
	}

	breadth := float64(distinctCount(surfaces)) / 3.0
	if breadth > 1 {
		breadth = 1
	}

	courseComp := 0.0
	if courseLearned {
		courseComp = 1
	}

	score := int(math.Round(100 * (0.35*courseComp + 0.50*useComp + 0.15*breadth)))
	if score < 0 {
		score = 0
	} else if score > 100 {
		score = 100
	}

	stars := 1 + score/20
	if stars > 5 {
		stars = 5
	}
	return Proficiency{Encountered: true, Score: score, Stars: stars}
}

func distinctCount(xs []string) int {
	seen := map[string]struct{}{}
	for _, x := range xs {
		seen[x] = struct{}{}
	}
	return len(seen)
}
