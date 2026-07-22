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
	// Framing projects the S1 立题 view: the research question, the student's
	// term definitions/answers/search plan. See projectFraming (projection.go).
	Framing FramingDTO `json:"framing"`
	// Perspectives projects the S2 视角与素材 view's perspective list: rows the
	// station view wrote (editable) alongside rows the perspective-matrix tool
	// card minted (read-only), plus the recorded sources_per_perspective
	// attestation. See projectPerspectives (projection.go).
	Perspectives PerspectivesDTO `json:"perspectives"`
	Materials    []MaterialDTO   `json:"materials"`
	// ActiveCard projects the one project-wide card_instance that is
	// "proposed" or "active". agent.SurfaceCardCandidates (classifier.go)
	// makes at most one in-flight card_instance the INTENDED steady state,
	// but that is not a hard invariant: nothing (no unique constraint, no
	// lock spanning its perceive-time read and the SurfaceCard insert it
	// gates) actually prevents two concurrent turns from both perceiving
	// "nothing in flight" and each minting one — see projectActiveCard
	// (projection.go), which accordingly takes the first match rather than
	// assuming exactly one exists (FIX 6, whole-branch review). nil when none
	// is open. Fixes the reload-bricks-the-workspace bug: without this, a
	// page reload loses the client's only reference to the open card while
	// the row stays proposed/active server-side, and FIX-D's own suppression
	// (which is otherwise exactly right) then blocks every future card from
	// ever surfacing again — permanently, project-wide.
	ActiveCard *ActiveCardDTO `json:"activeCard"`
	// Structure projects the five Toulmin argument slots (S4 论证构建) once the
	// argument has been minted; empty until then so the pane keeps its
	// placeholder. See projectStructure (projection.go).
	Structure []StructureCardDTO `json:"structure"`
	// Writing projects the S5 写作 view: the silent edit buffer, the latest
	// immutable snapshot, the word budget, the citations attestation, and the
	// whole-draft review work-order. See projectWriting (projection.go).
	Writing WritingDTO `json:"writing"`
	// Readiness projects the 评估 view's 就绪度 gauge (Slice 9): one card per 0457
	// review table (表D/E/F/H), lit from the latest board-voice whole-draft
	// review ⋈ skill config. Always the full config set (4 unlit cards
	// pre-review). See projectReadiness (projection.go).
	Readiness []GaugeDTO `json:"readiness"`
	// SelfScore projects the 先自己评一评 card (S6): one dim per review criterion
	// with the student's own picked band. See projectSelfScore (projection.go).
	SelfScore SelfScoreDTO `json:"selfScore"`
	// Reflection projects the S6 写一段研究回顾 card: the student's latest
	// reflection text + the prompt chips. See projectReflection (projection.go).
	Reflection ReflectionDTO `json:"reflection"`
	// Prediction projects the S0↔S6 reveal: the criteria the student predicted
	// weakest at S0 vs. the criteria the board review actually found weak. See
	// projectPrediction (projection.go).
	Prediction PredictionDTO `json:"prediction"`
	// SpotChecks projects the S3/S4 station spot-check panels (信源体检 /
	// 论证体检), keyed by station — a flat, required top-level member (the
	// same move N3d used for Framing/Perspectives), since Materials/Structure
	// are arrays and cannot host a keyed member. See projectSpotChecks
	// (projection.go).
	SpotChecks SpotChecksDTO `json:"spotChecks"`
	// Finished is true once the project's terminal has run (project.status ==
	// "finished"). CanFinish is true when the S5 整稿体检 gate item
	// whole_draft_review is "solid" AND the project is not already finished —
	// the studio shows 完成任务·归档 exactly then (A3). Derived, never stored.
	Finished  bool `json:"finished"`
	CanFinish bool `json:"canFinish"`
	// Declaration projects the S6 AI 使用申报单: four counters read from data
	// that already exists (asks, dispositions, the 自发/提示后 card split)
	// plus whether the student has signed. Before signing the counters are
	// computed live; after signing they are read back from the persisted
	// `declaration` node so the screen never drifts from what was actually
	// signed. See projectDeclaration (projection.go).
	Declaration DeclarationDTO `json:"declaration"`
}

// ActiveCardDTO is the wire shape StudioContainer/conversation.ts hydrate a
// live CardState from on load — the same shape the SSE "card" event carries
// (cardId + status + anchors + materialId), minus the CardSpec itself: the
// client already looks that up from CARD_REGISTRY by cardId, so it is not
// duplicated here.
type ActiveCardDTO struct {
	CardInstanceID string `json:"cardInstanceId"`
	CardID         string `json:"cardId"`
	// Status is "proposed" (offered, not yet opened) or "active" (open,
	// being filled). SurfaceCardCandidates' suppression is DESIGNED to leave
	// at most one such row project-wide, but nothing enforces that as a hard
	// invariant (see StudioProjection.ActiveCard's comment) — this is the
	// intended value, not a guaranteed one.
	Status     string            `json:"status"`
	Anchors    []json.RawMessage `json:"anchors"`
	MaterialID string            `json:"materialId"`
}

// StructureCardDTO is one role in the S4 argument (论证构建). status/preview
// are DERIVED from the Toulmin mint: a card is "done" (with the student's own
// sentence as preview) only because a graph node typed for its slot carries a
// body.text she wrote; otherwise "empty". Nothing here invents an argument —
// same derive-never-decorate contract as MaterialDTO. The list is present only
// once the argument exists in the graph; before that it is empty and the 结构
// pane keeps its placeholder.
type StructureCardDTO struct {
	ID      string `json:"id"`      // slot id: claim/warrant/evidence/counter/concession
	Role    string `json:"role"`    // 核心主张 / 理据 · 推理 / 支撑证据 / 反方 · 钢人 / 让步 · 转折
	Status  string `json:"status"`  // "done" | "empty"
	Preview string `json:"preview"` // the student's sentence (body.text); "" when empty
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
	RestatePrompt    string         `json:"restatePrompt"`
	RubricRows       []RubricRowDTO `json:"rubricRows"`
	PlanSteps        []string       `json:"planSteps"`
	AssignmentText   string         `json:"assignmentText"`
	StudentRestate   string         `json:"studentRestate"`
	StudentWeakPicks []int          `json:"studentWeakPicks"`
}

// TermDefinitionDTO is one key-term row in S1's glossary: the student's own
// term + her own definition (spec §6.1's term_definition node), never AI-authored.
type TermDefinitionDTO struct {
	Term       string `json:"term"`
	Definition string `json:"definition"`
}

// FramingDTO projects S1 立题. ResearchQuestion is the research_question node's
// text (minted from the project title at creation), not the project row's title
// — so the banner shows what the graph actually asserts.
type FramingDTO struct {
	ResearchQuestion string              `json:"researchQuestion"`
	Terms            []TermDefinitionDTO `json:"terms"`
	Answers          []string            `json:"answers"`
	SearchPlan       []string            `json:"searchPlan"`
}

// PerspectiveRowDTO is one row of S2's list. Editable is false for rows minted
// by the perspective-matrix card: they carry no level and this view must not
// rewrite them (the S2 write is origin-scoped for exactly this reason).
type PerspectiveRowDTO struct {
	Text     string `json:"text"`
	Level    string `json:"level"`
	Editable bool   `json:"editable"`
}

// PerspectivesDTO projects S2 视角与素材's perspective list + the recorded
// sources_per_perspective attestation (spec §6.2).
type PerspectivesDTO struct {
	Rows                  []PerspectiveRowDTO `json:"rows"`
	SourcesPerPerspective bool                `json:"sourcesPerPerspective"`
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

// WritingBudgetDTO is the deterministic count-vs-band verdict (agent.BudgetVerdict).
// state ∈ {in, over, under}; delta is a non-negative magnitude (words past max
// when over, short of min when under, 0 when in) — the client reads direction
// from state and renders delta directly.
type WritingBudgetDTO struct {
	State string `json:"state"`
	Delta int    `json:"delta"`
}

// WritingSnapshotDTO is the latest immutable draft_snapshot (S5 写作): the
// committed content is not itself carried here — the client already has it
// from the commit response / re-fetches it — only the metadata a reload
// needs (which snapshot, when, and whether it lands in the word budget).
type WritingSnapshotDTO struct {
	ID          string           `json:"id"`
	Seq         int              `json:"seq"`
	CommittedAt string           `json:"committedAt"`
	WordCount   int              `json:"wordCount"`
	InBand      bool             `json:"inBand"`
	Budget      WritingBudgetDTO `json:"budget"`
}

// DispositionDTO is the three-key disposition (accept/reject/rewrite + a
// reason) a student recorded on an intervention — here, one review_item.
type DispositionDTO struct {
	Action string `json:"action"`
	Reason string `json:"reason"`
}

// WritingReviewItemDTO is one whole-draft-review work-order row: which
// criterion, the band the draft sits in, the evidence/missing/fix the model
// produced, and the student's own disposition of it once recorded (nil
// until then — never invented).
type WritingReviewItemDTO struct {
	InterventionID string          `json:"interventionId"`
	Criterion      string          `json:"criterion"`
	Band           string          `json:"band"`
	Evidence       string          `json:"evidence"`
	Missing        string          `json:"missing"`
	Fix            string          `json:"fix"`
	Voice          string          `json:"voice"`
	Disposition    *DispositionDTO `json:"disposition"`
}

// WritingReviewDTO carries every review work-order row anchored to the CURRENT
// latest snapshot, across all voices the student has run. Each item is
// self-describing via its Voice; the client derives the current-voice
// work-order and the cached-voice set by filtering.
type WritingReviewDTO struct {
	Items []WritingReviewItemDTO `json:"items"`
}

// WordBudgetDTO is the per-qualification legal word band for S5, straight
// off the skill's sk.WordBudget — needs no persisted state.
type WordBudgetDTO struct {
	Min int `json:"min"`
	Max int `json:"max"`
}

// WritingDTO is the S5 写作 view: the silent buffer, the latest immutable
// snapshot, the word budget, the citations attestation (from the
// draft_polish gate's recorded citations_matched item), and the whole-draft
// review work-order joined with dispositions. Derive-never-decorate: every
// field is read back from persisted rows — see projectWriting
// (projection.go).
type WritingDTO struct {
	Buffer           string              `json:"buffer"`
	LatestSnapshot   *WritingSnapshotDTO `json:"latestSnapshot"`
	WordBudget       WordBudgetDTO       `json:"wordBudget"`
	CitationsMatched bool                `json:"citationsMatched"`
	Review           WritingReviewDTO    `json:"review"`
}

// SelfScoreDimDTO is one review criterion the student rates against the mark
// scheme. Band is 0..2 (还需努力/基本达到/稳了), or -1 when not yet picked.
// RL-5: the student's own per-criterion judgement, never an aggregate grade.
type SelfScoreDimDTO struct {
	Code string `json:"code"`
	Name string `json:"name"`
	Band int    `json:"band"`
}

type SelfScoreDTO struct {
	Dims  []SelfScoreDimDTO `json:"dims"`
	Bands []string          `json:"bands"`
}

// ReflectionDTO is the S6 研究回顾: the student's own text (RL-4, AI never
// authors it) + the prompt chips the platform offers.
type ReflectionDTO struct {
	Text    string   `json:"text"`
	Prompts []string `json:"prompts"`
}

// DeclarationDTO is the S6 AI 使用申报单: four counters read from data that
// already exists, plus whether the student has signed. Its five non-Signed
// fields mirror internal/api/declaration.go's DeclarationCounts exactly
// (same JSON tags) so json.Unmarshal of a persisted `declaration` node body
// into this type just works.
type DeclarationDTO struct {
	Asks             int `json:"asks"`
	Dispositions     int `json:"dispositions"`
	CardsSpontaneous int `json:"cardsSpontaneous"`
	CardsPrompted    int `json:"cardsPrompted"`
	// AiWrittenProse is always 0 — see internal/api/declaration.go's
	// DeclarationCounts field of the same name for why (RL-1: the
	// whole-draft review's `fix` field is advice, never written into the
	// draft).
	AiWrittenProse int `json:"aiWrittenProse"`
	// Signed is read from the recorded reflect_archive gate_state's
	// declaration_signed item — the same "solid" check agent.CheckGate
	// applies to every human/student_written item.
	Signed bool `json:"signed"`
}

// PredCritDTO is one review criterion by code+name (used in the prediction reveal).
type PredCritDTO struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// PredictionDTO is the S0↔S6 reveal: the criteria the student predicted weakest
// at S0 vs. the criteria the board review actually found weak. Overlap is a
// descriptive self-knowledge count, NEVER a score (RL-3/RL-5). Actual/Revealed
// are empty/false until a whole-draft review has informed the gauge.
type PredictionDTO struct {
	Predicted []PredCritDTO `json:"predicted"`
	Actual    []PredCritDTO `json:"actual"`
	Overlap   int           `json:"overlap"`
	Revealed  bool          `json:"revealed"`
}

// SpotCheckItemDTO is one station spot-check work-order row: which target,
// the evidence/missing/fix the model produced, and the student's own
// disposition of it once recorded (nil until then — never invented). Exactly
// the relationship WritingReviewItemDTO has with agent.ReviewItem:
// agent.SpotCheckItem (marshalled into the intervention body) plus
// InterventionID (from the row) and Disposition (from d.Dispositions) —
// agent.SpotCheckItem itself gains neither field. No band: see
// agent.SpotCheckItem's own comment on why a spot-check carries no verdict.
type SpotCheckItemDTO struct {
	InterventionID string          `json:"interventionId"`
	TargetID       string          `json:"targetId"`
	TargetName     string          `json:"targetName"`
	Evidence       string          `json:"evidence"`
	Missing        string          `json:"missing"`
	Fix            string          `json:"fix"`
	Disposition    *DispositionDTO `json:"disposition"`
}

// SpotCheckFxDTO is one station's spot-check panel: its current work order
// (the latest ordered batch's items) plus whether ordering is CURRENTLY
// possible. Orderable is computed server-side (projectSpotChecks,
// projection.go) — the button's enabled state must never be a client guess
// about whether pressing it would cost money.
type SpotCheckFxDTO struct {
	Items     []SpotCheckItemDTO `json:"items"`
	Orderable bool               `json:"orderable"`
}

// SpotChecksDTO carries both stations that have a spot-check
// (agent.SpotCheckSources "evaluate_sources" / agent.SpotCheckArgument
// "build_argument") as StudioProjection's flat, required spotChecks member.
type SpotChecksDTO struct {
	EvaluateSources SpotCheckFxDTO `json:"evaluateSources"`
	BuildArgument   SpotCheckFxDTO `json:"buildArgument"`
}

// GaugeDTO is one 0457 mark-scheme table on the 就绪度 readiness display: the
// descriptor cell the latest BOARD-voice review placed the draft in. Lit/Total
// are lamp counts (which cell), never a predicted grade (RL-3). Note is the
// review's own `missing` — what's absent, so the next step is legible; "" when
// the table is full. Level is derived from lit/total.
type GaugeDTO struct {
	Code  string `json:"code"`  // 表D..表H
	Name  string `json:"name"`  // 来源与证据 / 分析 / 评估 / 表达与组织
	Lit   int    `json:"lit"`   // clamp(review.points, 0, total); 0 when no board review
	Total int    `json:"total"` // skill config points
	Note  string `json:"note"`  // review.missing; "" when full / no review
	Level string `json:"level"` // "full" | "partial" | "empty"
}
