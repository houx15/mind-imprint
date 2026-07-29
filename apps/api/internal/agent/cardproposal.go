package agent

import (
	"context"
	"strings"

	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/gateway"
)

// cardproposal.go — S4 · cross-phase card proposing. The always-reply coach may,
// at the 克制 ladder's summon rung, OFFER a student card mid-conversation. This
// reuses the exact semantic-moment classifier + card-id table the silence-
// default studio loop uses (moment.go) — no new vocabulary, no invented ids, so
// the coach can only ever offer a card the placement map already sanctions.
// Opening is the student's tap (铁律: triggering automatic, opening confirmed);
// this returns an OFFER, never an auto-open.

// CardProposal is the coach's offer attached to a reply. Wire shape matches
// packages/contracts's CardProposal (camelCase). nil on the respond/hint rungs.
type CardProposal struct {
	CardID    string `json:"cardId"`
	Reason    string `json:"reason"`
	NudgeText string `json:"nudgeText"`
}

// ProposeCoachCard runs the moment classifier over the student's latest text and,
// if a still-eligible moment fires, returns the card to OFFER. Spends one
// chaperone call ONLY when there is something to classify (text over the rune
// floor AND a non-empty eligible set); the caller meters the returned usage and
// enforces the per-project classifier cap. Returns (nil, zero, nil) — no spend —
// when there's nothing to classify, and (nil, usage, nil) when the classifier
// says none.
func ProposeCoachCard(ctx context.Context, prov gateway.Provider, r gateway.Resolved, studentText string, eligible []Moment) (*CardProposal, gateway.ChatUsage, error) {
	if len([]rune(strings.TrimSpace(studentText))) < MinClassifyRunes || len(eligible) == 0 {
		return nil, gateway.ChatUsage{}, nil // nothing to classify → no spend
	}
	m, usage, err := ClassifyMoment(ctx, prov, r, studentText, eligible)
	if err != nil {
		return nil, gateway.ChatUsage{}, err
	}
	if m == MomentNone {
		return nil, usage, nil
	}
	e := momentCard[m]
	name := e.CardID
	if spec, ok := cards.ByID(e.CardID); ok && strings.TrimSpace(spec.Name) != "" {
		name = spec.Name
	}
	return &CardProposal{
		CardID:    e.CardID,
		Reason:    e.Flag,
		NudgeText: "要不要打开「" + name + "」这张卡？",
	}, usage, nil
}
