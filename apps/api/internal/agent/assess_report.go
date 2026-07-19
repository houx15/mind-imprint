package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"mindimprint/api/internal/agent/enforcement"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/rubric"
)

// ---- Report: the whole DualAxis growth report (RL-5: the ONLY number is
// DepthAxis.Subtotal, within-axis; no field sums across axes). ----

type DepthDimScore struct {
	Code           string `json:"code"`
	Name           string `json:"name"`
	Score          int    `json:"score"` // 0..3
	Evidence       string `json:"evidence"`
	PromptEvidence string `json:"promptEvidence"`
}

type DepthAxis struct {
	Dims     []DepthDimScore `json:"dims"`
	Subtotal int             `json:"subtotal"` // Σ Dims.Score, 0..12
}

type AutonomyAxis struct {
	Code             string   `json:"code"`
	Name             string   `json:"name"`
	Observation      string   `json:"observation"`
	AnchoredSignals  []string `json:"anchoredSignals"`
	PromptedSignals  []string `json:"promptedSignals"`
	AdversaryInvites int      `json:"adversaryInvites"`
	PromptEvidence   string   `json:"promptEvidence"`
}

type CrossAxis struct {
	Code           string `json:"code"`
	Name           string `json:"name"`
	DepthLevel     string `json:"depthLevel"` // L1..L4|NA
	Initiative     string `json:"initiative"`
	Prose          string `json:"prose"`
	PromptEvidence string `json:"promptEvidence"`
}

type SoloRow struct {
	Round      int    `json:"round"`
	Excerpt    string `json:"excerpt"`
	Level      string `json:"level"` // L1..L4
	Rationale  string `json:"rationale"`
	Initiative string `json:"initiative"` // 自发 | 引导后
}

type LensQuestion struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

type PromptSample struct {
	Round      int    `json:"round"`
	Quote      string `json:"quote"`
	Annotation string `json:"annotation"`
}

type PerRoundTier struct {
	Round int    `json:"round"`
	Tier  string `json:"tier"` // P0..P3
	Label string `json:"label"`
}

type PromptLens struct {
	DirectiveRounds  int            `json:"directiveRounds"`
	TotalRounds      int            `json:"totalRounds"`
	BoundarySettings int            `json:"boundarySettings"`
	AdversaryInvites int            `json:"adversaryInvites"`
	Questions        []LensQuestion `json:"questions"`
	BestPrompt       PromptSample   `json:"bestPrompt"`
	Takeaway         PromptSample   `json:"takeaway"`
	PerRound         []PerRoundTier `json:"perRound"`
}

type TimelineRow struct {
	Round   int      `json:"round"`
	Task    string   `json:"task"`
	Prompt  string   `json:"prompt"`
	PTag    string   `json:"pTag"`
	DimTags []string `json:"dimTags"`
}

type KeyEvidence struct {
	Label string `json:"label"`
	Quote string `json:"quote"`
}

type NextStep struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

type Guidance struct {
	Anchored  string     `json:"anchored"`
	Prompted  string     `json:"prompted"`
	Risk      string     `json:"risk"`
	NextSteps []NextStep `json:"nextSteps"`
}

type Report struct {
	DepthAxis    DepthAxis     `json:"depthAxis"`
	AutonomyAxis AutonomyAxis  `json:"autonomyAxis"`
	CrossAxis    CrossAxis     `json:"crossAxis"`
	Solo         []SoloRow     `json:"solo"`
	PromptLens   PromptLens    `json:"promptLens"`
	Timeline     []TimelineRow `json:"timeline"`
	KeyEvidence  []KeyEvidence `json:"keyEvidence"`
	Guidance     Guidance      `json:"guidance"`
	Narrative    string        `json:"narrative"`
	Axiom        string        `json:"axiom"`
}

// reportWire is what the model returns: depth dims keyed by code (no Name),
// no Axiom (engine fills it). Everything else mirrors Report.
type reportWire struct {
	DepthAxis struct {
		Dims []struct {
			Code           string `json:"code"`
			Score          int    `json:"score"`
			Evidence       string `json:"evidence"`
			PromptEvidence string `json:"promptEvidence"`
		} `json:"dims"`
	} `json:"depthAxis"`
	AutonomyAxis struct {
		Observation      string   `json:"observation"`
		AnchoredSignals  []string `json:"anchoredSignals"`
		PromptedSignals  []string `json:"promptedSignals"`
		AdversaryInvites int      `json:"adversaryInvites"`
		PromptEvidence   string   `json:"promptEvidence"`
	} `json:"autonomyAxis"`
	CrossAxis struct {
		DepthLevel     string `json:"depthLevel"`
		Initiative     string `json:"initiative"`
		Prose          string `json:"prose"`
		PromptEvidence string `json:"promptEvidence"`
	} `json:"crossAxis"`
	Solo        []SoloRow     `json:"solo"`
	PromptLens  PromptLens    `json:"promptLens"`
	Timeline    []TimelineRow `json:"timeline"`
	KeyEvidence []KeyEvidence `json:"keyEvidence"`
	Guidance    Guidance      `json:"guidance"`
	Narrative   string        `json:"narrative"`
}

func clampScore(s int) int {
	if s < 0 {
		return 0
	}
	if s > 3 {
		return 3
	}
	return s
}

var validSolo = map[string]bool{"L1": true, "L2": true, "L3": true, "L4": true, "NA": true}

func normSolo(l string) string {
	if validSolo[l] {
		return l
	}
	return "NA"
}

// validTier is the closed set of prompt-tier values the config/output-format/
// Zod contract all agree on: P0..P3. (The posture prose historically drifted
// to "P0–P4"; that drift is fixed separately, but a model can still emit an
// out-of-set value, so this guard stays regardless.)
var validTier = map[string]bool{"P0": true, "P1": true, "P2": true, "P3": true}

// normTier normalizes a model-emitted perRound prompt tier to the config's
// P0..P3 set — mirrors normSolo's guard for SOLO levels. Any out-of-set value
// (e.g. a stray "P4") normalizes to "P0", the neutral 应答轮, rather than
// persisting a value every web DualAxisReport.parse (strict P0–P3 enum) would
// reject — for the one-time project finish (no regenerate), an unnormalized
// tier would permanently brick that report.
func normTier(t string) string {
	if validTier[t] {
		return t
	}
	return "P0"
}

// AssessReport makes ONE isolated flagship call emitting the entire DualAxis
// report, runs banned-phrasing over every free-text field, fills dim names +
// axiom from the model, computes the depth subtotal (Σ scores), and emits every
// depth dim in model order (missing → score 0). Never in the coach loop.
func AssessReport(ctx context.Context, prov gateway.Provider, r gateway.Resolved, m rubric.DualAxis, in AssessmentInput) (Report, gateway.ChatUsage, error) {
	res, err := gateway.Collect(ctx, prov, r, gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: assessReportSystemPrompt(m)},
			{Role: gateway.RoleUser, Content: assessReportUserInput(in)},
		},
	})
	if err != nil {
		return Report{}, gateway.ChatUsage{}, err
	}
	usage := res.Usage

	var wire reportWire
	if err := json.Unmarshal([]byte(strings.TrimSpace(res.Text)), &wire); err != nil {
		return Report{}, usage, fmt.Errorf("agent: report output not JSON: %w", err)
	}

	// Enforcement over every free-text field. Any hit rejects the whole report.
	texts := []string{wire.Narrative, wire.AutonomyAxis.Observation, wire.AutonomyAxis.PromptEvidence,
		wire.CrossAxis.Prose, wire.CrossAxis.PromptEvidence,
		wire.Guidance.Anchored, wire.Guidance.Prompted, wire.Guidance.Risk}
	for _, d := range wire.DepthAxis.Dims {
		texts = append(texts, d.Evidence, d.PromptEvidence)
	}
	texts = append(texts, wire.AutonomyAxis.AnchoredSignals...)
	texts = append(texts, wire.AutonomyAxis.PromptedSignals...)
	for _, s := range wire.Solo {
		texts = append(texts, s.Excerpt, s.Rationale)
	}
	for _, q := range wire.PromptLens.Questions {
		texts = append(texts, q.Title, q.Body)
	}
	texts = append(texts, wire.PromptLens.BestPrompt.Quote, wire.PromptLens.BestPrompt.Annotation,
		wire.PromptLens.Takeaway.Quote, wire.PromptLens.Takeaway.Annotation)
	for _, tl := range wire.Timeline {
		texts = append(texts, tl.Task, tl.Prompt)
	}
	for _, pr := range wire.PromptLens.PerRound {
		texts = append(texts, pr.Label)
	}
	for _, ke := range wire.KeyEvidence {
		texts = append(texts, ke.Label, ke.Quote)
	}
	for _, ns := range wire.Guidance.NextSteps {
		texts = append(texts, ns.Title, ns.Body)
	}
	for _, f := range texts {
		if f == "" {
			continue
		}
		if rule := enforcement.BannedPhrasing(f); rule != nil {
			return Report{}, usage, fmt.Errorf("agent: report rejected by banned-phrasing rule %q", rule.Name)
		}
	}

	// Depth dims: index the model's scores by code; emit every depth dim in
	// model order (missing → 0), Name from the rubric, score clamped 0..3.
	got := map[string]struct {
		score        int
		evidence, pe string
	}{}
	for _, d := range wire.DepthAxis.Dims {
		got[d.Code] = struct {
			score        int
			evidence, pe string
		}{clampScore(d.Score), d.Evidence, d.PromptEvidence}
	}
	depth := DepthAxis{}
	for _, dim := range rubric.DepthDims() {
		g := got[dim.ID]
		depth.Dims = append(depth.Dims, DepthDimScore{
			Code: dim.ID, Name: dim.Name, Score: g.score, Evidence: g.evidence, PromptEvidence: g.pe,
		})
		depth.Subtotal += g.score
	}

	auto := rubric.AutonomyDim()
	cross := rubric.CrossDim()

	// Normalize perRound tiers into the P0..P3 contract before they're
	// persisted — timeline[].pTag stays a lenient z.string() on the Zod side
	// and is deliberately left alone.
	for i := range wire.PromptLens.PerRound {
		wire.PromptLens.PerRound[i].Tier = normTier(wire.PromptLens.PerRound[i].Tier)
	}

	rep := Report{
		DepthAxis: depth,
		AutonomyAxis: AutonomyAxis{
			Code: auto.ID, Name: auto.Name,
			Observation: wire.AutonomyAxis.Observation, AnchoredSignals: wire.AutonomyAxis.AnchoredSignals,
			PromptedSignals: wire.AutonomyAxis.PromptedSignals, AdversaryInvites: wire.AutonomyAxis.AdversaryInvites,
			PromptEvidence: wire.AutonomyAxis.PromptEvidence,
		},
		CrossAxis: CrossAxis{
			Code: cross.ID, Name: cross.Name,
			DepthLevel: normSolo(wire.CrossAxis.DepthLevel), Initiative: wire.CrossAxis.Initiative,
			Prose: wire.CrossAxis.Prose, PromptEvidence: wire.CrossAxis.PromptEvidence,
		},
		Solo:        normSoloRows(wire.Solo),
		PromptLens:  wire.PromptLens,
		Timeline:    wire.Timeline,
		KeyEvidence: wire.KeyEvidence,
		Guidance:    wire.Guidance,
		Narrative:   wire.Narrative,
		Axiom:       m.Axiom,
	}
	rep.AnchoredNilGuards()
	return rep, usage, nil
}

func normSoloRows(rows []SoloRow) []SoloRow {
	for i := range rows {
		rows[i].Level = normSolo(rows[i].Level)
	}
	return rows
}

// AnchoredNilGuards keeps JSON output arrays non-null (nil slice → []).
func (r *Report) AnchoredNilGuards() {
	if r.AutonomyAxis.AnchoredSignals == nil {
		r.AutonomyAxis.AnchoredSignals = []string{}
	}
	if r.AutonomyAxis.PromptedSignals == nil {
		r.AutonomyAxis.PromptedSignals = []string{}
	}
	if r.Solo == nil {
		r.Solo = []SoloRow{}
	}
	if r.Timeline == nil {
		r.Timeline = []TimelineRow{}
	}
	if r.KeyEvidence == nil {
		r.KeyEvidence = []KeyEvidence{}
	}
	if r.PromptLens.Questions == nil {
		r.PromptLens.Questions = []LensQuestion{}
	}
	if r.PromptLens.PerRound == nil {
		r.PromptLens.PerRound = []PerRoundTier{}
	}
	if r.Guidance.NextSteps == nil {
		r.Guidance.NextSteps = []NextStep{}
	}
	if r.DepthAxis.Dims == nil {
		r.DepthAxis.Dims = []DepthDimScore{}
	}
}
