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

// 🚨 2026-09-17：**这一步没有了，所以这两条断言也换了。**
//
// 产品负责人逐字：「at this stage, I think we can skip the 透镜 part. it is
// really not applicable in many papers. and difficult for students to
// understand. the above mentioned critical thinking can be a better
// replacement of lens.」
//
// 读法库里不再排 lens 那一步，而判据放在 lensOK 上：**清单里没有 lens 这一步，
// 就一副透镜都不给**（reading_coach.go 的 planHasLens）。整套透镜的机器没删 ——
// 哪天读法库里再排上它，原样就能用。
//
// 这两条原来断言的是「递出去的透镜落在它刚讲的那一段上」和「卡片和滚动位置
// 要一致」。那两件事现在**结构上到不了**：cardOut 永远是 nil。断言一件到不了
// 的事，绿着也什么都不证明（[[fixture-told-coach-session-over-2026-09-14]]），
// 所以换成断言这一轮真正该发生的事。
func TestCoachTurnDoesNotMintALensWhenThePlanHasNoLensStep(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, writingTextStubProvider(coachSummonScript))
	id := createReadingAtom(t, h, cookie)
	putReadingSourceHTTP(t, h, cookie, id, "碳排放与增长", coachSummonArticle)

	rec := coachTurn(t, h, cookie, id, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("coach turn = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var out coachTurnSummonJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode coach turn: %v — body=%s", err, rec.Body)
	}
	// 模型在这份回话里明明白白点了 "lens":"craap"。清单里没有那一步，所以
	// 一副都不给。
	if out.Card != nil {
		t.Errorf("清单里没有透镜那一步，却还是递了一副：%+v", out.Card)
	}
	// 🚨 这一轮仍然要成立：她拿到 印记 的话。透镜被拒掉不是一次失败，
	// 是这一步本来就不存在。
	if out.Reply != "这条来源值得查一下" {
		t.Errorf("reply = %q，透镜被拒掉不该动它", out.Reply)
	}
}

// 「读法库里一套带 lens 的都不该剩下」那一条在 reading_routines_internal_test.go
// 里（readingRoutines 是未导出的，这个文件是 package api_test）。
