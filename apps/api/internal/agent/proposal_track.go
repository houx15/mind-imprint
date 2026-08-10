package agent

// proposal_track.go — slice 3a · the proposal-writing guide-step track. The
// track is a DIMENSION of the proposal document (not a new FlowStatus): the
// student picks free / guided; in guided mode 印记 hands out one guide card per
// part in turn. The research plan is NOT one card — it is a "define 2–4
// sub-questions" step followed by ONE card per sub-question (all-statuses.md §4).
// So the step list is content-derived and DYNAMIC: its length grows with the
// number of sub-questions the student defines. Everything here is pure — model
// calls (guide-card generation) live in proposal_guide.go.

// WriteMode is how the student is writing a document. "" = not chosen yet.
type WriteMode string

const (
	ModeUnset  WriteMode = ""
	ModeFree   WriteMode = "free"
	ModeGuided WriteMode = "guided"
)

// SubQuestion is one of the 2–4 sub-questions the student decomposes the key
// research question into. ID is minted server-side and keys the per-sub-question
// guide card + snippet; Text is the student's own wording.
type SubQuestion struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

// WritingTrack is the per-document guide-step track. StepIndex walks the DERIVED
// step list (see DeriveProposalSteps), whose length is 9 + len(SubQuestions).
// StepGuides caches generated guide cards keyed by the stable step KEY (not the
// int index, which shifts as sub-questions change).
type WritingTrack struct {
	Mode         WriteMode         `json:"mode"`
	Started      bool              `json:"started"`
	StepIndex    int               `json:"stepIndex"`
	SubQuestions []SubQuestion     `json:"subQuestions,omitempty"`
	StepGuides   map[string]string `json:"stepGuides,omitempty"`
}

// StepKind distinguishes a fixed golden-standard part, the sub-question define
// step, and a per-sub-question card.
type StepKind string

const (
	KindFixed      StepKind = "fixed"
	KindSubqDefine StepKind = "subq-define"
	KindSubq       StepKind = "subq"
)

// Step is one entry in the derived guide-step list.
type Step struct {
	Key           string   `json:"key"`
	Title         string   `json:"title"`
	Kind          StepKind `json:"kind"`
	SubQuestionID string   `json:"subQuestionId,omitempty"`
}

// ProposalFixedParts returns the 9 golden-standard parts in order
// (all-statuses.md §4): the three Introduction parts, the research-plan
// (sub-question define) step, then the five tail parts. Motivation is NOT a
// step — it is carried by the framework's 缘由 dimension.
func ProposalFixedParts() []Step {
	return []Step{
		{Key: "understanding", Title: "对题目的理解", Kind: KindFixed},
		{Key: "question-scope", Title: "研究问题与范围", Kind: KindFixed},
		{Key: "thesis", Title: "暂定论点", Kind: KindFixed},
		{Key: "research-plan", Title: "研究计划 · 定子问题", Kind: KindSubqDefine},
		{Key: "resources", Title: "资源", Kind: KindFixed},
		{Key: "challenges", Title: "可能的挑战", Kind: KindFixed},
		{Key: "method", Title: "研究方法", Kind: KindFixed},
		{Key: "feasibility", Title: "可行性 / 限制 / 伦理", Kind: KindFixed},
		{Key: "expected", Title: "预期结果", Kind: KindFixed},
		// §4 gap G9 · the final assembly/polish step: read the whole proposal in
		// order, polish it, get a whole-proposal 批注, then 完成提案.
		{Key: "polish", Title: "通读与润色", Kind: KindFixed},
	}
}

// subqCardTitle names the per-sub-question card (1-based for the student).
func subqCardTitle(n int) string {
	return "子问题 " + itoa(n)
}

// itoa is a tiny local int→string (avoids importing strconv for one call site
// in a file that is otherwise dependency-free).
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// DeriveProposalSteps builds the guided step list from the fixed parts with the
// research-plan part EXPANDED into the define step + one KindSubq step per
// defined sub-question, inserted right after research-plan and before resources.
// With no sub-questions it yields the 9 fixed steps.
func DeriveProposalSteps(t WritingTrack) []Step {
	fixed := ProposalFixedParts()
	out := make([]Step, 0, len(fixed)+len(t.SubQuestions))
	for _, s := range fixed {
		out = append(out, s)
		if s.Kind == KindSubqDefine {
			for i, sq := range t.SubQuestions {
				out = append(out, Step{
					Key:           "subq:" + sq.ID,
					Title:         subqCardTitle(i + 1),
					Kind:          KindSubq,
					SubQuestionID: sq.ID,
				})
			}
		}
	}
	return out
}

// ClampStepIndex keeps i within [0, len-1] for a derived list of length n
// (n==0 → 0). Used by the REST layer after re-derivation.
func ClampStepIndex(i, n int) int {
	if n <= 0 {
		return 0
	}
	if i < 0 {
		return 0
	}
	if i >= n {
		return n - 1
	}
	return i
}
