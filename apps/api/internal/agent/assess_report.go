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

// ---- Report: the canonical DualAxis growth report (RL-5: the two axes never
// combine into a total score; depth carries no subtotal; the only percentage
// anywhere is OfficialProjection.Readiness.Score, project-surface only). ----

// DepthDim is one of the six depth-axis dimensions (D1-D6), judged L1-L4 (or
// NA when no evidence is available — NA is "no evidence", never a low score).
type DepthDim struct {
	Code           string `json:"code"`
	Name           string `json:"name"`
	Level          string `json:"level"` // L1|L2|L3|L4|NA
	LevelRange     string `json:"levelRange,omitempty"`
	Evidence       string `json:"evidence"`
	PromptEvidence string `json:"promptEvidence"`
}

// AutonomySignal is one of the six autonomy-axis signals (A1-A6): a
// behavior-count signal (0-5), never a quality score.
type AutonomySignal struct {
	Code           string `json:"code"`
	Name           string `json:"name"`
	Level          int    `json:"level"`       // 0..5
	Opportunity    string `json:"opportunity"` // given_taken|given_not_taken|not_supplied
	Evidence       string `json:"evidence"`
	PromptEvidence string `json:"promptEvidence"`
}

// LensStat is one of the three summary stat cards in the prompt lens.
type LensStat struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// Lens is one of the six prompt-lens process indicators (reads AI
// interaction traces only; never a third scoring axis).
type Lens struct {
	Code     string `json:"code"`
	Name     string `json:"name"`
	Level    int    `json:"level"` // 0..5
	Evidence string `json:"evidence"`
}

// PromptLens is the process-evidence lens over the student's prompts: three
// summary stats plus six per-lens judgements, never combined into any score.
type PromptLens struct {
	Stats  []LensStat `json:"stats"`
	Lenses []Lens     `json:"lenses"`
	Note   string     `json:"note"`
}

// InteractionRow is one round of grounded interaction evidence: the student's
// own prompt, the AI's summarized response, and the signal it evidences.
type InteractionRow struct {
	Round     int    `json:"round"`
	Student   string `json:"student"`
	AiSummary string `json:"aiSummary"`
	Signal    string `json:"signal"`
}

// NextStep is one guidance item: a title plus a concrete task.
type NextStep struct {
	Title string `json:"title"`
	Task  string `json:"task"`
}

// Guidance is the report's forward-looking section: next steps only (no
// anchored/prompted/risk fields — those belonged to the retired shape).
type Guidance struct {
	NextSteps []NextStep `json:"nextSteps"`
}

// OfficialComponent is one judged component of an official external standard
// (e.g. AP Research's Academic Paper / POD / 训练用折算 / 诚信).
type OfficialComponent struct {
	Name      string `json:"name"`
	Judgement string `json:"judgement"`
	Reason    string `json:"reason"`
}

// OfficialAlignment is one alignment item between the project and the
// official standard's requirements.
type OfficialAlignment struct {
	Item        string `json:"item"`
	Standard    string `json:"standard"`
	Performance string `json:"performance"`
	Impact      string `json:"impact"`
}

// OfficialStandardRef identifies the official standard the projection aligns
// to (e.g. "ap-research" / "AP Research").
type OfficialStandardRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// OfficialReadiness is the ONLY numeric aggregate anywhere in Report: a
// project-surface-only work-readiness percentage. Its Note must say this
// never combines with the depth/autonomy axes.
type OfficialReadiness struct {
	Score int    `json:"score"` // 0..100
	Note  string `json:"note"`
}

// OfficialProjection is the project-surface-only projection of the dual-axis
// evidence onto an official external standard. nil on chat/course surfaces.
type OfficialProjection struct {
	Standard   OfficialStandardRef `json:"standard"`
	Components []OfficialComponent `json:"components"`
	Alignment  []OfficialAlignment `json:"alignment"`
	Readiness  OfficialReadiness   `json:"readiness"`
}

// WorkSample is one excerpt of the student's actual work product.
type WorkSample struct {
	Title string `json:"title"`
	Text  string `json:"text"`
}

// ProcessMaterial is one process artifact (e.g. a SIFT record) with its
// completion status and a diagnosis of what it shows.
type ProcessMaterial struct {
	Name      string `json:"name"`
	Status    string `json:"status"`
	Diagnosis string `json:"diagnosis"`
}

// WorkAndProcess is the project-surface-only work-product + process-artifact
// section. nil on chat/course surfaces. The evidence map is deliberately NOT
// here — it is a deterministic projection of the process graph (Spec D), not
// an assessor output.
type WorkAndProcess struct {
	WorkSamples      []WorkSample      `json:"workSamples"`
	ProcessMaterials []ProcessMaterial `json:"processMaterials"`
}

// Report is the canonical assessment object, persisted verbatim into
// evaluations.scores. It has NO generatedAt (that is added by
// studio.ToReportDTO at read time) and no cross-axis aggregate field anywhere
// except OfficialReadiness.Score.
type Report struct {
	DepthAxis           []DepthDim       `json:"depthAxis"`
	AutonomyAxis        []AutonomySignal `json:"autonomyAxis"`
	PromptLens          PromptLens       `json:"promptLens"`
	InteractionEvidence []InteractionRow `json:"interactionEvidence"`
	Narrative           string           `json:"narrative"`
	Guidance            Guidance         `json:"guidance"`
	Axiom               string           `json:"axiom"`

	OfficialProjection *OfficialProjection `json:"officialProjection,omitempty"`
	WorkAndProcess     *WorkAndProcess     `json:"workAndProcess,omitempty"`
}

// ---- reportWire: what the model returns over the wire. Depth/autonomy/lens
// items are keyed by code (no Name — the engine fills it from the rubric);
// no Axiom, no generatedAt (both engine-supplied). ----

type depthWire struct {
	Code           string `json:"code"`
	Level          string `json:"level"`
	LevelRange     string `json:"levelRange"`
	Evidence       string `json:"evidence"`
	PromptEvidence string `json:"promptEvidence"`
}

type autonomyWire struct {
	Code           string `json:"code"`
	Level          int    `json:"level"`
	Opportunity    string `json:"opportunity"`
	Evidence       string `json:"evidence"`
	PromptEvidence string `json:"promptEvidence"`
}

type lensWire struct {
	Code     string `json:"code"`
	Level    int    `json:"level"`
	Evidence string `json:"evidence"`
}

type promptLensWire struct {
	Stats  []LensStat `json:"stats"`
	Lenses []lensWire `json:"lenses"`
}

type officialProjectionWire struct {
	Standard   OfficialStandardRef `json:"standard"`
	Components []OfficialComponent `json:"components"`
	Alignment  []OfficialAlignment `json:"alignment"`
	Readiness  OfficialReadiness   `json:"readiness"`
}

type reportWire struct {
	DepthAxis           []depthWire             `json:"depthAxis"`
	AutonomyAxis        []autonomyWire          `json:"autonomyAxis"`
	PromptLens          promptLensWire          `json:"promptLens"`
	InteractionEvidence []InteractionRow        `json:"interactionEvidence"`
	Narrative           string                  `json:"narrative"`
	Guidance            Guidance                `json:"guidance"`
	OfficialProjection  *officialProjectionWire `json:"officialProjection"`
	WorkAndProcess      *WorkAndProcess         `json:"workAndProcess"`
}

var validDepthLevel = map[string]bool{"L1": true, "L2": true, "L3": true, "L4": true, "NA": true}

// clampDepthLevel normalizes a model-emitted depth level to the
// {L1,L2,L3,L4,NA} contract. Anything outside the set (including an empty
// string, i.e. a missing dimension) normalizes to "NA" — no evidence, never a
// low score.
func clampDepthLevel(l string) string {
	if validDepthLevel[l] {
		return l
	}
	return "NA"
}

// clampAxisLevel clamps an autonomy-signal or lens level to 0..5.
func clampAxisLevel(l int) int {
	if l < 0 {
		return 0
	}
	if l > 5 {
		return 5
	}
	return l
}

var validOpportunity = map[string]bool{"given_taken": true, "given_not_taken": true, "not_supplied": true}

// normOpportunity normalizes a model-emitted opportunity tag to the closed
// three-value set. An out-of-set value defaults to "given_taken" (the brief's
// rule for a present-but-invalid tag — distinct from a wholly missing signal,
// which defaults to "not_supplied").
func normOpportunity(o string) string {
	if validOpportunity[o] {
		return o
	}
	return "given_taken"
}

// clampReadiness clamps the official-projection readiness score to 0..100.
func clampReadiness(s int) int {
	if s < 0 {
		return 0
	}
	if s > 100 {
		return 100
	}
	return s
}

// normalizeDepth guarantees full 6-dimension coverage in rubric order: a
// dimension the model omitted gets Level "NA" and empty evidence; Name always
// comes from the rubric, never the model.
func normalizeDepth(wire []depthWire) []DepthDim {
	got := map[string]depthWire{}
	for _, d := range wire {
		got[d.Code] = d
	}
	out := make([]DepthDim, 0, len(rubric.DepthDims()))
	for _, dim := range rubric.DepthDims() {
		g, ok := got[dim.ID]
		level := "NA"
		if ok {
			level = clampDepthLevel(g.Level)
		}
		out = append(out, DepthDim{
			Code: dim.ID, Name: dim.Name, Level: level, LevelRange: g.LevelRange,
			Evidence: g.Evidence, PromptEvidence: g.PromptEvidence,
		})
	}
	return out
}

// normalizeAutonomy guarantees full 6-signal coverage in rubric order: a
// signal the model omitted gets Level 0 and Opportunity "not_supplied" (a
// platform gap, never a student shortfall); Name always comes from the
// rubric.
func normalizeAutonomy(wire []autonomyWire) []AutonomySignal {
	got := map[string]autonomyWire{}
	for _, a := range wire {
		got[a.Code] = a
	}
	out := make([]AutonomySignal, 0, len(rubric.AutonomySignals()))
	for _, sig := range rubric.AutonomySignals() {
		g, ok := got[sig.ID]
		level := 0
		opportunity := "not_supplied"
		if ok {
			level = clampAxisLevel(g.Level)
			opportunity = normOpportunity(g.Opportunity)
		}
		out = append(out, AutonomySignal{
			Code: sig.ID, Name: sig.Name, Level: level, Opportunity: opportunity,
			Evidence: g.Evidence, PromptEvidence: g.PromptEvidence,
		})
	}
	return out
}

// normalizeLenses guarantees full 6-lens coverage in rubric order: a lens the
// model omitted gets Level 0; Name always comes from the rubric.
func normalizeLenses(wire []lensWire) []Lens {
	got := map[string]lensWire{}
	for _, l := range wire {
		got[l.Code] = l
	}
	out := make([]Lens, 0, len(rubric.Lenses()))
	for _, lensCfg := range rubric.Lenses() {
		g, ok := got[lensCfg.ID]
		level := 0
		if ok {
			level = clampAxisLevel(g.Level)
		}
		out = append(out, Lens{Code: lensCfg.ID, Name: lensCfg.Name, Level: level, Evidence: g.Evidence})
	}
	return out
}

// normalizeStats truncates to the first 3 stats when the model over-produces,
// and pads with blank-label/value entries when it under-produces — the report
// always carries exactly 3 stat cards.
func normalizeStats(stats []LensStat) []LensStat {
	out := make([]LensStat, 3)
	for i := 0; i < len(stats) && i < 3; i++ {
		out[i] = stats[i]
	}
	return out
}

// normalizeOfficialProjection passes the project-surface projection through,
// clamping Readiness.Score to 0..100 and filling Standard from the rubric's
// "ap-research" entry when the model left it blank.
func normalizeOfficialProjection(w *officialProjectionWire) *OfficialProjection {
	if w == nil {
		w = &officialProjectionWire{} // degenerate: model omitted it in project mode — still back-fill standard below
	}
	standard := w.Standard
	if standard.ID == "" && standard.Name == "" {
		if std, ok := rubric.Standard("ap-research"); ok {
			standard = OfficialStandardRef{ID: std.ID, Name: std.Name}
		}
	}
	return &OfficialProjection{
		Standard:   standard,
		Components: w.Components,
		Alignment:  w.Alignment,
		Readiness:  OfficialReadiness{Score: clampReadiness(w.Readiness.Score), Note: w.Readiness.Note},
	}
}

// AssessReport makes ONE isolated flagship call emitting the entire canonical
// report, runs banned-phrasing over every free-text field (rejecting the
// whole report on any hit — cost is recorded by the caller regardless), fills
// dim/signal/lens names + axiom from the rubric, guarantees full 6-coverage
// on all three closed-set axes, and emits the project-only superset
// (officialProjection / workAndProcess) ONLY when in.ProjectProjection is
// true. Never in the coach loop.
func AssessReport(ctx context.Context, prov gateway.Provider, r gateway.Resolved, m rubric.DualAxis, in AssessmentInput) (Report, gateway.ChatUsage, error) {
	res, err := gateway.Collect(ctx, prov, r, gateway.ChatRequest{
		// The canonical report is a large structured JSON object (6 depth dims +
		// 6 autonomy signals + 6 lenses + interaction evidence + narrative +
		// guidance, plus the project-only official-projection/work superset).
		// The gateway default (1024) truncates it mid-JSON → "unexpected end of
		// JSON input" → the whole assessment 422s. Give it room for the full
		// report (flagship, never downgraded).
		MaxTokens: 8000,
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: assessReportSystemPrompt(m, in.ProjectProjection)},
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

	rep := Report{
		DepthAxis:    normalizeDepth(wire.DepthAxis),
		AutonomyAxis: normalizeAutonomy(wire.AutonomyAxis),
		PromptLens: PromptLens{
			Stats:  normalizeStats(wire.PromptLens.Stats),
			Lenses: normalizeLenses(wire.PromptLens.Lenses),
			Note:   m.LensNote,
		},
		InteractionEvidence: wire.InteractionEvidence,
		Narrative:           wire.Narrative,
		Guidance:            wire.Guidance,
		Axiom:               m.Axiom,
	}
	if in.ProjectProjection {
		rep.OfficialProjection = normalizeOfficialProjection(wire.OfficialProjection)
		wap := wire.WorkAndProcess
		if wap == nil {
			wap = &WorkAndProcess{}
		}
		rep.WorkAndProcess = wap
	}
	rep.AnchoredNilGuards()

	if err := enforceReport(rep); err != nil {
		return Report{}, usage, err
	}

	return rep, usage, nil
}

// enforceReport runs enforcement.BannedPhrasing over every free-text field in
// the assembled report. Any hit rejects the whole report (the caller has
// already recorded the llm_call cost — reject-on-banned-phrasing is
// cost-on-reject by construction).
func enforceReport(rep Report) error {
	texts := []string{rep.Narrative, rep.PromptLens.Note}
	for _, d := range rep.DepthAxis {
		texts = append(texts, d.Evidence, d.PromptEvidence)
	}
	for _, a := range rep.AutonomyAxis {
		texts = append(texts, a.Evidence, a.PromptEvidence)
	}
	for _, l := range rep.PromptLens.Lenses {
		texts = append(texts, l.Evidence)
	}
	for _, s := range rep.PromptLens.Stats {
		texts = append(texts, s.Label, s.Value)
	}
	for _, ie := range rep.InteractionEvidence {
		texts = append(texts, ie.Student, ie.AiSummary, ie.Signal)
	}
	for _, ns := range rep.Guidance.NextSteps {
		texts = append(texts, ns.Title, ns.Task)
	}
	if rep.OfficialProjection != nil {
		for _, c := range rep.OfficialProjection.Components {
			texts = append(texts, c.Reason)
		}
		for _, al := range rep.OfficialProjection.Alignment {
			texts = append(texts, al.Performance, al.Impact)
		}
		texts = append(texts, rep.OfficialProjection.Readiness.Note)
	}
	if rep.WorkAndProcess != nil {
		for _, ws := range rep.WorkAndProcess.WorkSamples {
			texts = append(texts, ws.Text)
		}
		for _, pm := range rep.WorkAndProcess.ProcessMaterials {
			texts = append(texts, pm.Diagnosis)
		}
	}
	for _, t := range texts {
		if t == "" {
			continue
		}
		if rule := enforcement.BannedPhrasing(t); rule != nil {
			return fmt.Errorf("agent: report rejected by banned-phrasing rule %q", rule.Name)
		}
	}
	return nil
}

// AnchoredNilGuards keeps every JSON output array non-null (nil slice → []).
func (r *Report) AnchoredNilGuards() {
	if r.DepthAxis == nil {
		r.DepthAxis = []DepthDim{}
	}
	if r.AutonomyAxis == nil {
		r.AutonomyAxis = []AutonomySignal{}
	}
	if r.PromptLens.Stats == nil {
		r.PromptLens.Stats = []LensStat{}
	}
	if r.PromptLens.Lenses == nil {
		r.PromptLens.Lenses = []Lens{}
	}
	if r.InteractionEvidence == nil {
		r.InteractionEvidence = []InteractionRow{}
	}
	if r.Guidance.NextSteps == nil {
		r.Guidance.NextSteps = []NextStep{}
	}
	if r.OfficialProjection != nil {
		if r.OfficialProjection.Components == nil {
			r.OfficialProjection.Components = []OfficialComponent{}
		}
		if r.OfficialProjection.Alignment == nil {
			r.OfficialProjection.Alignment = []OfficialAlignment{}
		}
	}
	if r.WorkAndProcess != nil {
		if r.WorkAndProcess.WorkSamples == nil {
			r.WorkAndProcess.WorkSamples = []WorkSample{}
		}
		if r.WorkAndProcess.ProcessMaterials == nil {
			r.WorkAndProcess.ProcessMaterials = []ProcessMaterial{}
		}
	}
}
