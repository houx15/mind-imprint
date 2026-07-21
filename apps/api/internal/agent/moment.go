package agent

// Moment is one member of the CLOSED SET of semantic card-moments the
// classifier may name. It is a closed enum on purpose: the classifier picks
// from a fixed vocabulary and can never invent a card id, which is the same
// principle that makes the C1 interaction primitives a hand-built library
// rather than model-generated UI (agent-spec §3).
type Moment string

const (
	// MomentNone is the classifier's "nothing here" answer, and the zero value.
	MomentNone Moment = ""

	// MomentFactOpinion: the student stated an opinion that needs arguing as
	// if it were a checkable fact.
	MomentFactOpinion Moment = "fact_opinion"

	// MomentOverclaim: the student's expressed certainty outruns the evidence
	// he has actually given.
	MomentOverclaim Moment = "overclaim"

	// MomentOneSided: the student argues from one side only, never engaging
	// the strongest opposing case.
	MomentOneSided Moment = "one_sided"
)

// AllMoments is the vocabulary in stable priority order — when the classifier
// is offered several, this is the order it sees them in, and the order
// EligibleMoments returns.
var AllMoments = []Moment{MomentFactOpinion, MomentOverclaim, MomentOneSided}

// momentEntry is everything the rest of the system needs to know about one
// moment. This table is the SINGLE SOURCE for a moment's card id, its
// description in the classifier prompt, the coach-facing flag sentence, the
// candidate's internal reason, and its CT criterion. No other file may
// hardcode these card ids (Global Constraints).
type momentEntry struct {
	CardID string
	// Desc is how the moment is described TO THE CLASSIFIER (Chinese, one line).
	Desc string
	// Flag is the sentence handed to the COACH as context ("刚刚发生的思考时机").
	Flag string
	// Reason is the internal-only candidate reason (never shown to a student).
	Reason string
	// Criterion is the CT dimension tag (internal/rubric/dualaxis.json).
	Criterion string
}

var momentCard = map[Moment]momentEntry{
	MomentFactOpinion: {
		CardID:    "fact-opinion-value",
		Desc:      "学生把一个需要论证的观点，当成可以直接查证的事实陈述来用。",
		Flag:      "学生把一个需要论证的观点当成了事实——这是分辨「事实/观点/价值判断」的时机。",
		Reason:    "student stated an arguable opinion as a checkable fact",
		Criterion: "D3",
	},
	MomentOverclaim: {
		CardID:    "certainty-spectrum",
		Desc:      "学生表达的确定程度，高于他给出的证据所能支撑的程度。",
		Flag:      "学生的语气比他的证据更确定——这是给结论标定「确定度」的时机。",
		Reason:    "student's certainty outruns the evidence given",
		Criterion: "D3",
	},
	MomentOneSided: {
		CardID:    "steelman",
		Desc:      "学生只从一侧论证，没有处理最强的反面意见。",
		Flag:      "学生只从一侧论证——这是构造对方最强版本的时机。",
		Reason:    "student argues from one side only",
		Criterion: "D4",
	},
}

// EligibleMoments returns the moments still open for a project, in AllMoments
// order. A moment is ineligible as soon as its target card has a card_instance
// in ANY status — including "skipped". An offer is never a wall: once she has
// said no, we do not ask again (铁律 2 · 不操纵).
func EligibleMoments(cards []CardInstanceView) []Moment {
	seen := make(map[string]bool, len(cards))
	for _, c := range cards {
		seen[c.CardID] = true
	}
	out := make([]Moment, 0, len(AllMoments))
	for _, m := range AllMoments {
		if !seen[momentCard[m].CardID] {
			out = append(out, m)
		}
	}
	return out
}
