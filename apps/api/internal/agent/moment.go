package agent

import (
	"context"
	"fmt"
	"strings"

	"mindimprint/api/internal/gateway"
)

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

// EligibleMomentsScoped mirrors EligibleMoments over ScopedCard — the Chat/
// Course scoped-graph projection (chat_step.go), rather than the project
// GraphView's CardInstanceView. Same rule: a moment is ineligible as soon as
// its target card has a card_instance in ANY status, including "skipped".
func EligibleMomentsScoped(cards []ScopedCard) []Moment {
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

// MinClassifyRunes is the floor below which a student turn is never
// classified. Cost discipline AND product: 「嗯」 carries no moment, and paying
// a model call to be told so is waste.
const MinClassifyRunes = 12

// momentSystemPrompt is the classifier's posture. It is deliberately unlike
// every other prompt in this package: the classifier does not talk to the
// student, does not coach, and does not write prose. Its entire output is one
// identifier from a closed set.
const momentSystemPrompt = `# 角色
你是「思维印记」的时机识别器。你不与学生对话，也不给任何建议——你唯一的任务，是判断学生刚写下的这段话里，是否正在发生下面列出的某一个「思考时机」。

# 规则
- 只能从下面给出的候选 id 中选**一个**，或者回答 none。
- 拿不准就回答 none。宁可错过，也不要打断学生。
- 只输出那个 id 本身，不要解释、不要标点、不要任何其他文字。`

// ClassifyMoment asks the chaperone-tier model whether the student's latest
// text exhibits one of the eligible moments.
//
// Contract:
//   - Empty eligible set → MomentNone with NO model call (cost discipline).
//   - The reply is matched by EXACT equality against the eligible ids after
//     trimming. A chatty reply, an unknown id, or an id that is real but NOT
//     eligible all collapse to MomentNone — the last case matters most: it is
//     what stops a suppressed card from being re-offered by a model that
//     ignored its instructions.
//   - Usage is returned whenever gateway.Collect succeeded, INCLUDING when the
//     answer is `none` or unparseable. That call cost real money and the caller
//     must meter it (AGENTS.md 记录档位 + token + 成本). Usage is the zero
//     value only when Collect itself errored.
//   - An error is returned ONLY for a failed model call. Callers treat it as
//     silence, never as a turn failure.
func ClassifyMoment(ctx context.Context, prov gateway.Provider, r gateway.Resolved, text string, eligible []Moment) (Moment, gateway.ChatUsage, error) {
	if len(eligible) == 0 {
		return MomentNone, gateway.ChatUsage{}, nil
	}

	var b strings.Builder
	b.WriteString("# 候选时机\n")
	for _, m := range eligible {
		fmt.Fprintf(&b, "- %s：%s\n", m, momentCard[m].Desc)
	}
	b.WriteString("\n# 学生刚写下的话\n")
	b.WriteString(text)
	b.WriteString("\n\n只输出一个 id，或 none。")

	res, err := gateway.Collect(ctx, prov, r, gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: momentSystemPrompt},
			{Role: gateway.RoleUser, Content: b.String()},
		},
	})
	if err != nil {
		return MomentNone, gateway.ChatUsage{}, err
	}

	answer := strings.TrimSpace(res.Text)
	for _, m := range eligible {
		if answer == string(m) {
			return m, res.Usage, nil
		}
	}
	return MomentNone, res.Usage, nil
}
