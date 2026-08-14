package agent

// ---- Report: the canonical DualAxis growth report (RL-5: the two axes never
// combine into a total score; depth carries no subtotal; the only percentage
// anywhere is OfficialProjection.Readiness.Score, project-surface only).
//
// These types are the surviving scoring shape from the retired assessment
// generator (agent.AssessReport, deleted 2026-08-14): the kept
// teacher-roster/weekly cluster (teacher.DBadge/ABadge) and studio.ReportDTO
// still read/unmarshal agent.Report, so the shape must persist even though
// nothing in this package generates a Report anymore. ----

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

// AnchoredNilGuards keeps every JSON output array non-null (nil slice → []).
// Called by studio.ToReportDTO at read time.
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
