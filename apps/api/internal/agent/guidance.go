package agent

// GuidanceLevel is the scaffold level a student gets on an annotate card,
// derived from how many times she has already completed that card
// (spec §3). It is never stored: it decides only what the AI fills in at
// surface time, and the anchors themselves then carry the level (§2).
type GuidanceLevel int

const (
	GuidanceL1 GuidanceLevel = 1 // AI elicits the question AND locates the span
	GuidanceL2 GuidanceLevel = 2 // AI elicits; she locates
	GuidanceL3 GuidanceLevel = 3 // she elicits AND locates
)

// GuidanceFor maps completed uses of a card onto the ladder: 0 → L1, 1 → L2,
// 2+ → L3. The fade is SILENT — no badge, no level-up, nothing celebratory
// anywhere in the UI (铁律 2). It only changes what the card asks for.
func GuidanceFor(completedUses int) GuidanceLevel {
	switch {
	case completedUses <= 0:
		return GuidanceL1
	case completedUses == 1:
		return GuidanceL2
	default:
		return GuidanceL3
	}
}
