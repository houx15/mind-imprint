// Package studio builds the read-path material-dossier projection.
package studio

import "encoding/json"

type RubricRowDTO struct {
	Official string `json:"official"`
	Plain    string `json:"plain"`
	Weak     bool   `json:"weak"`
}

type OnboardingDTO struct {
	RestatePrompt    string         `json:"restatePrompt"`
	RubricRows       []RubricRowDTO `json:"rubricRows"`
	PlanSteps        []string       `json:"planSteps"`
	AssignmentText   string         `json:"assignmentText"`
	StudentRestate   string         `json:"studentRestate"`
	StudentWeakPicks []int          `json:"studentWeakPicks"`
}

type MaterialBlockDTO struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

// MaterialDTO is one source in the 素材 dossier. locked/role are DERIVED from
// the CRAAP mint (an evaluated-as edge → an evidence node's
// source_quality.risk_note), tier/takeaway from the student's source-log entry,
// anchors from the persisted card_instances.anchors targeting this material.
// There is no verdict field — see spec §3: a 可信/存疑 judgment has no honest
// producer and would have to be fabricated.
type MaterialDTO struct {
	ID        string             `json:"id"`
	Title     string             `json:"title"`
	SourceURL string             `json:"sourceUrl"`
	Kind      string             `json:"kind"`
	Origin    string             `json:"origin"`
	Blocks    []MaterialBlockDTO `json:"blocks"`
	Locked    bool               `json:"locked"`
	Role      string             `json:"role"`
	Tier      string             `json:"tier"`
	Takeaway  string             `json:"takeaway"`
	Anchors   []json.RawMessage  `json:"anchors"`
	// TimeSpentS is the source-log entry's accumulated reading time
	// (seconds) — spec §5's ledger row binds title/URL, 停留 Nm, the
	// takeaway, and the tier chip together. 0 for a freshly ingested source.
	TimeSpentS int32 `json:"timeSpentS"`
	// LateralRead mirrors source_log_entry.lateral_read (Slice 6c): true only
	// once a cross_check mint has flipped it on THIS source (the one that was
	// checked, not the lateral source used to check it). A fact about what
	// happened, not a credibility verdict — there is still none of those.
	LateralRead bool `json:"lateralRead"`
	// IsLateralInstrument is true when this material is itself the lateral
	// source some OTHER cross_check cited (a "cites" edge from a cross_check
	// node to this material) — the exact graph fact
	// agent.SurfaceCardCandidates' treadmill guard already uses to exclude a
	// lateral instrument from ever being proposed for its own SIFT
	// (agent/classifier.go). The dossier's chip must derive from this SAME
	// fact, never recompute graph reachability independently — otherwise the
	// chip can promise a lateral-read workflow the summon rule will never
	// actually offer (whole-branch review finding [4]).
	IsLateralInstrument bool `json:"isLateralInstrument"`
	// SiftSkipped is true when a SIFT card_instance targeting this material
	// was explicitly skipped (agent/classifier.go's siftSurfaced treats a
	// skip the same as any other in-flight/terminal status, so once skipped
	// SIFT is never re-proposed on this material — the dossier's
	// 需横向阅读 chip must not keep promising a lateral-read workflow the
	// summon rule will in fact never offer again; FIX 3, whole-branch
	// review). A recorded decision, not an erased one: the skip itself still
	// lives in the process tree (the card_instance row + its event_trace) —
	// this field only stops the LIVE chip from mis-describing the state as
	// "currently being verified" once nothing is.
	SiftSkipped bool `json:"siftSkipped"`
	// LateralRelation/LateralJudgment are derived from the cross_check node
	// reached by this material's own "cross-checked-by" edge (Slice 6c's SIFT
	// mint) — her own chosen relation (印证/反驳/限定) and her own revised-
	// judgment sentence, read back exactly as she wrote them (card_effects.go
	// crossCheckBody). "" when no cross_check exists for this material yet —
	// nothing is invented, following riskNote()'s exact derive-never-decorate
	// pattern; not omitempty, same as role/tier/takeaway (this DTO's
	// convention: a field with no producer yet is present-and-empty, never
	// hidden behind an optional key). Nothing else in the product ever read
	// this node's body before (whole-branch review: "written but never read").
	LateralRelation string `json:"lateralRelation"`
	LateralJudgment string `json:"lateralJudgment"`
}
