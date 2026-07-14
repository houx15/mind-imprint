// Package studio builds the read-path StudioProjection wire DTO (Slice 5b).
// The DTO's JSON shape mirrors packages/contracts/src/studioState.ts exactly.
package studio

import "encoding/json"

type StudioProjection struct {
	Project       ProjectHeader `json:"project"`
	Stations      []StationDTO  `json:"stations"`
	ActiveStation string        `json:"activeStation"`
	Coach         CoachDTO      `json:"coach"`
	Onboarding    OnboardingDTO `json:"onboarding"`
	Materials     []MaterialDTO `json:"materials"`
}

type ProjectHeader struct {
	Title     string `json:"title"`
	QualLabel string `json:"qualLabel"`
}

type StationDTO struct {
	Code     string   `json:"code"`
	Name     string   `json:"name"`
	View     string   `json:"view"`
	State    string   `json:"state"`
	Gate     *GateDTO `json:"gate,omitempty"`
	Backflow bool     `json:"backflow,omitempty"`
}

type GateDTO struct {
	Total  int `json:"total"`
	Passed int `json:"passed"`
}

type CoachDTO struct {
	Anchor    string            `json:"anchor"`
	Messages  []CoachMessageDTO `json:"messages"`
	Equipment []EquipCardDTO    `json:"equipment"`
}

// CoachMessageDTO carries the union tag in `kind`. For kind="ai": Body + optional
// Tag/Anchor. For kind="flag": Label + Body. For kind="student": Body.
type CoachMessageDTO struct {
	Kind   string `json:"kind"`
	Body   string `json:"body"`
	Tag    string `json:"tag,omitempty"`
	Anchor string `json:"anchor,omitempty"`
	Label  string `json:"label,omitempty"`
}

type EquipCardDTO struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Spont string `json:"spont"`
	Meth  string `json:"meth"`
	// MaterialID is the material this card_instance evaluates (its
	// card_instance--evaluates-->material graph edge target), when one
	// exists. Carried so the client never has to guess which material an
	// equipment-bar card is about (whole-branch review finding [5]).
	MaterialID string `json:"materialId"`
}

type RubricRowDTO struct {
	Official string `json:"official"`
	Plain    string `json:"plain"`
	Weak     bool   `json:"weak"`
}

type OnboardingDTO struct {
	RestatePrompt string         `json:"restatePrompt"`
	RubricRows    []RubricRowDTO `json:"rubricRows"`
	PlanSteps     []string       `json:"planSteps"`
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
}
