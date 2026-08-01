package cards

import "testing"

func TestComputeProficiency_NotEncountered(t *testing.T) {
	p := ComputeProficiency(0, nil, false)
	if p.Encountered || p.Score != 0 || p.Stars != 0 {
		t.Fatalf("never-touched card should be grey/0/0, got %+v", p)
	}
}

func TestComputeProficiency_CourseOnly(t *testing.T) {
	// Finished the course, never practiced in a project: learned the method,
	// low practice. 0.35*1 = 0.35 -> 35 -> 2 stars.
	p := ComputeProficiency(0, nil, true)
	if !p.Encountered {
		t.Fatalf("course-learned card must be encountered")
	}
	if p.Score != 35 {
		t.Fatalf("course-only score want 35, got %d", p.Score)
	}
	if p.Stars != 2 {
		t.Fatalf("course-only stars want 2, got %d", p.Stars)
	}
}

func TestComputeProficiency_ChatOnlyIsOneStar(t *testing.T) {
	// One chat completion, no project practice, no course: breadth 1/3 only.
	// 0.15 * (1/3) = 0.05 -> 5 -> 1 star. Any real encounter is >=1 star.
	p := ComputeProficiency(0, []string{"chat"}, false)
	if !p.Encountered || p.Stars != 1 {
		t.Fatalf("chat-only want encountered/1 star, got %+v", p)
	}
}

func TestComputeProficiency_SaturatesAndClimbsWithRealUse(t *testing.T) {
	// Increasing genuine project completions must not decrease the score, and
	// must saturate (the log ceiling) rather than run away.
	prev := -1
	for _, n := range []int{1, 2, 4, 8, 16, 64} {
		p := ComputeProficiency(n, []string{"project"}, false)
		if p.Score < prev {
			t.Fatalf("score must be monotonic non-decreasing in use; n=%d score=%d prev=%d", n, p.Score, prev)
		}
		prev = p.Score
	}
	// At saturation (8) useComp == 1: 0.5 + 0.15*(1/3) = 0.55 -> 55.
	at8 := ComputeProficiency(8, []string{"project"}, false)
	at64 := ComputeProficiency(64, []string{"project"}, false)
	if at8.Score != 55 {
		t.Fatalf("useComp should saturate at 8 -> 55, got %d", at8.Score)
	}
	if at64.Score != at8.Score {
		t.Fatalf("beyond saturation score must not climb: at8=%d at64=%d", at8.Score, at64.Score)
	}
}

func TestComputeProficiency_MasteryIsFiveStars(t *testing.T) {
	// Course-learned + saturated practice + all three surfaces: the ceiling.
	// 0.35 + 0.5 + 0.15 = 1.0 -> 100 -> 5 stars.
	p := ComputeProficiency(8, []string{"project", "course", "chat"}, true)
	if p.Score != 100 || p.Stars != 5 {
		t.Fatalf("full mastery want 100/5, got %+v", p)
	}
}

func TestComputeProficiency_BreadthRewardsTransfer(t *testing.T) {
	one := ComputeProficiency(4, []string{"project"}, false)
	two := ComputeProficiency(4, []string{"project", "chat"}, false)
	if two.Score <= one.Score {
		t.Fatalf("cross-surface transfer should score higher: one=%d two=%d", one.Score, two.Score)
	}
}

func TestCoverKey(t *testing.T) {
	// craap -> T01 exists in the manifest for every theme.
	for _, th := range Themes {
		key, ok := CoverKey("T01", th)
		if !ok || key == "" {
			t.Fatalf("T01/%s should resolve to a key, got %q %v", th, key, ok)
		}
	}
	// Unknown theme falls back to default (still resolves).
	if _, ok := CoverKey("T01", "not-a-theme"); !ok {
		t.Fatalf("unknown theme should fall back to default and resolve")
	}
	// No asset id -> no cover.
	if _, ok := CoverKey("", "light"); ok {
		t.Fatalf("empty asset id must not resolve")
	}
	// A T-number with no cover art -> no cover.
	if _, ok := CoverKey("T99", "light"); ok {
		t.Fatalf("unknown asset id must not resolve")
	}
}
