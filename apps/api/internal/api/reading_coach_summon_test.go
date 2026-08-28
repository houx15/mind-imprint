package api_test

// reading_coach_summon_test.go — Task 5: 带读 hands her the lens it just
// named, aimed at the paragraph it just talked about.
//
// This is the one integration point the last two tasks built toward: Task 3
// taught parseReadingCoachReply to keep an aimed `lens` field; Task 4
// extracted summonReadingLens so both the student's own 透镜库 pick and the
// coach's own initiative mint through the same one path. This test is the
// only place that exercises them wired together through the real HTTP
// handler.

import (
	"encoding/json"
	"net/http"
	"testing"
)

// coachSummonArticle is a 3-paragraph article → b1, b2, b3 (SplitBlocks). The
// coach's reply below aims its lens at b2, and the grounding quote is a
// verbatim substring of b2 — the same requirement agent.ProposeCardExample /
// agent.ResolveExampleAnchor enforce everywhere else in this package.
const coachSummonArticle = `中国的太阳能装机量在过去十年增长了十倍。

但同一时期，中国的碳排放总量仍居全球第一。

这篇报道并没有说清楚这些数字的出处。`

// coachSummonScript is ONE reply serving THREE different parsers against the
// same stub provider, the same merged-reply pattern
// TestReadingCoach_StartPlansAndLeadsHerIn already relies on:
//
//  1. planReadingTasks (routineKey/focusBlocks/steps) — 开始 with no plan yet
//     generates one on demand before the coach ever gets a turn.
//  2. parseReadingCoachReply (reply/advance/focusBlock/lens) — the coach's
//     own turn.
//  3. agent.ProposeCardExample (block_id/quote/why) — the grounding call
//     summonReadingLens makes once the parsed reply carries a lens.
//
// Each parser reads only the fields it cares about and ignores the rest.
const coachSummonScript = `{"routineKey":"zh-scan-focus-lens","focusBlocks":["b2"],"steps":[],
  "reply":"这条来源值得查一下","advance":"","focusBlock":"b2","lens":"craap",
  "block_id":"b2","quote":"中国的碳排放总量仍居全球第一。","why":"这句给了一个没有出处的数字。"}`

// coachTurnSummonJSON is the coach turn response shape this test cares about
// — the superset of coachTurnJSON (reading_coach_test.go) plus the two fields
// Task 5 adds: card and nudge.
type coachTurnSummonJSON struct {
	Reply      string `json:"reply"`
	FocusBlock string `json:"focusBlock"`
	Nudge      string `json:"nudge"`
	Card       *struct {
		ID      string  `json:"id"`
		CardID  string  `json:"cardId"`
		BlockID *string `json:"blockId"`
		Status  string  `json:"status"`
		Origin  string  `json:"origin"`
	} `json:"card"`
}

// The coach names a lens and a paragraph; the room gets a card aimed there.
func TestCoachTurnMintsAimedLens(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, writingTextStubProvider(coachSummonScript))
	id := createReadingAtom(t, h, cookie)
	putReadingSourceHTTP(t, h, cookie, id, "碳排放与增长", coachSummonArticle)

	// 开始 — no plan exists yet, so this one turn also generates the plan
	// before the coach itself replies.
	rec := coachTurn(t, h, cookie, id, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("coach turn = %d, want 200; body=%s", rec.Code, rec.Body)
	}

	var out coachTurnSummonJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode coach turn: %v — body=%s", err, rec.Body)
	}

	// 1. the response carries a card with status "proposed"
	if out.Card == nil {
		t.Fatalf("coach turn carried no card at all; body=%s", rec.Body)
	}
	if out.Card.Status != "proposed" {
		t.Errorf("card status = %q, want \"proposed\"", out.Card.Status)
	}

	// 2. the card's block_id is "b2"
	if out.Card.BlockID == nil || *out.Card.BlockID != "b2" {
		t.Errorf("card blockId = %v, want \"b2\"", out.Card.BlockID)
	}

	// 3. origin is "router" — she did not choose this lens
	if out.Card.Origin != "router" {
		t.Errorf("card origin = %q, want \"router\"", out.Card.Origin)
	}

	// 4. the reply text is unchanged by the summon
	if out.Reply != "这条来源值得查一下" {
		t.Errorf("reply = %q, want the coach's own reply untouched by the summon", out.Reply)
	}
}

// focusLensDisagreeScript — F4 (final review): a turn that BOTH advances into
// a focus_block step (routine "zh-scan-focus-lens", focusBlocks=["b2"] — so
// the step she is walking INTO carries block "b2") AND summons a lens aimed
// at a DIFFERENT paragraph ("b4", the one the reply itself just discussed).
// Before the fix, the response's focusBlock was unconditionally overridden
// with the NEXT step's own block id — so the article would scroll to b2
// while the lens card hung under b4.
const focusLensDisagreeScript = `{"routineKey":"zh-scan-focus-lens","focusBlocks":["b2"],"steps":[],
  "reply":"通读完了，来看这一句","advance":"done","focusBlock":"b4","lens":"craap",
  "block_id":"b4","quote":"绿地和水面是相反的力量。","why":"这句提出了一个对比论点。"}`

// TestCoachTurn_LensAgreesWithFocusBlockOverStepsOwnParagraph — the card and
// the scroll must agree. When a lens actually minted THIS turn, the response
// stays aimed at the paragraph the lens is aimed at, even though the newly
// current step names its own (different) paragraph.
func TestCoachTurn_LensAgreesWithFocusBlockOverStepsOwnParagraph(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, writingTextStubProvider(focusLensDisagreeScript))
	id := createReadingAtom(t, h, cookie)
	putReadingSourceHTTP(t, h, cookie, id, "城市为什么比郊区热？", zhArticle)

	rec := coachTurn(t, h, cookie, id, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("coach turn = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var out coachTurnSummonJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode coach turn: %v — body=%s", err, rec.Body)
	}

	if out.Card == nil {
		t.Fatalf("no card minted at all; body=%s", rec.Body)
	}
	if out.Card.BlockID == nil || *out.Card.BlockID != "b4" {
		t.Fatalf("card blockId = %v, want \"b4\" (setup broken, not the thing under test)", out.Card.BlockID)
	}
	// The bug: this used to read "b2" (the newly-current focus_block step's
	// own paragraph) instead of staying on the paragraph the card is aimed
	// at.
	if out.FocusBlock != "b4" {
		t.Errorf("focusBlock = %q, want \"b4\" to agree with the minted card — "+
			"the room would scroll one way while the lens card hangs under the other paragraph", out.FocusBlock)
	}
}
