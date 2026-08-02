package agent

const (
	proposalBreathingTurns = 2
	skipCooldownTurns      = 3
)

// containsStr is a tiny linear membership check — kept here (its only
// remaining caller) after course_step.go's retirement (Task 5, course v2:
// migration 0050 dropped the course_session/course_step tables the old
// CourseStore-backed runtime needed; nothing in the course v2 surface uses
// this pacing-gate helper).
func containsStr(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

// OrderingGuard is the source-check ordering prior, derived from the graph by
// the caller (Task 5) using SurfaceCardCandidates' rules: don't summon SIFT
// before the material has a CRAAP evaluation; don't summon CRAAP on a material
// already evaluated. The router may not override this.
type OrderingGuard struct {
	AllowCraap bool
	AllowSift  bool
}

// ApplyReadingGate is the deterministic, authoritative restraint layer. It can
// only DOWNGRADE the model's decision (summon→hint→respond), never upgrade.
// Order matters: hard suppressions first, breathing-room softening last.
func ApplyReadingGate(d ReadingDecision, pacing PacingState, ordering OrderingGuard) ReadingDecision {
	if d.Decision == "respond" {
		return d
	}
	// One-active mutex: while a card is open, nothing new fires.
	if pacing.OpenCard {
		return respond
	}
	if d.Decision == "summon" {
		// Skip cooldown.
		if containsStr(pacing.RecentlySkipped, d.CardID) {
			return respond
		}
		// New-span reuse: a completed card won't re-open without new focus.
		if containsStr(pacing.CompletedCards, d.CardID) && !pacing.HasNewFocus {
			return respond
		}
		// Source-check ordering guard.
		if d.CardID == "sift" && !ordering.AllowSift {
			return respond
		}
		if d.CardID == "craap" && !ordering.AllowCraap {
			return respond
		}
		// Breathing room: no NEW proposal within the window — soften to a hint.
		if pacing.TurnsSinceLastPropose < proposalBreathingTurns {
			return ReadingDecision{Decision: "hint", CardID: d.CardID, Reason: d.Reason}
		}
	}
	return d
}

// ResolveExampleAnchor turns a summon's example (block_id + quote) into a real,
// verbatim-validated L1 anchor. ok=false means the quote is not a verbatim
// substring of the named block — the caller must retry the router once, then
// degrade to respond. NEVER build a (0,0) anchor here (that is the "lights up
// nothing" bug this replaces).
func ResolveExampleAnchor(d ReadingDecision, materialID string, blocks []MaterialBlock) (Anchor, bool) {
	if d.Decision != "summon" || d.ExampleQuote == "" {
		return Anchor{}, false
	}
	var text string
	found := false
	for _, b := range blocks {
		if b.ID == d.ExampleBlockID {
			text, found = b.Text, true
			break
		}
	}
	if !found {
		return Anchor{}, false
	}
	start, end := computeOffsets(text, d.ExampleQuote)
	if end <= start { // (0,0) means "not a substring" — reject.
		return Anchor{}, false
	}
	q := d.ExampleWhy
	if q == "" {
		q = "先看这处示范，再换你在文章里找一句自己的证据。"
	}
	return Anchor{
		ID: "ex0", MaterialID: materialID, BlockID: d.ExampleBlockID,
		Start: start, End: end, Quote: d.ExampleQuote,
		Dimension: d.CardID, Author: "ai", Question: q,
	}, true
}
