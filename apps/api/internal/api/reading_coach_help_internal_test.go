package api

import (
	"strings"
	"testing"

	"mindimprint/api/internal/store/sqlc"
)

func aiWithCard(seq int32, c *coachCard) sqlc.AtomMessage {
	return sqlc.AtomMessage{Seq: seq, Role: "ai", Content: "印记说的话", Payload: coachCardPayload(c)}
}

func TestHelpRequestSectionLadder(t *testing.T) {
	// 她自己打的一句话不是那颗按钮。
	if s := helpRequestSection("能不能给点提示啊", 1, nil); s != "" {
		t.Errorf("一句普通的话被当成了按钮：%q", s)
	}
	// 三级梯子，每一级说的不是同一件事。
	open := &coachCard{Type: coachCardShortText, Prompt: "写一句"}
	one := helpRequestSection(" 给点提示 ", 1, open)
	two := helpRequestSection(coachAskHint, 2, open)
	three := helpRequestSection(coachAskHint, 3, open)
	for i, want := range []struct {
		got, key string
	}{{one, "方向"}, {two, "位置"}, {three, "局部线索"}} {
		if !strings.Contains(want.got, want.key) {
			t.Errorf("第 %d 级没说「%s」：%q", i+1, want.key, want.got)
		}
	}
	if one == two || two == three {
		t.Error("两级提示写的是同一段话，梯子就没有级")
	}
	// 每一级都要说「卡片留着、不推进」—— 这是这一节存在的理由。
	for i, s := range []string{one, two, three} {
		if !strings.Contains(s, "不要再发新卡片") || !strings.Contains(s, "advance 留空") {
			t.Errorf("第 %d 级没说「卡片留着、不推进」：%q", i+1, s)
		}
	}
	// 给满三次之后不再给提示。
	if s := helpRequestSection(coachAskHint, 4, open); !strings.Contains(s, "不要再给新的提示") {
		t.Errorf("第 4 次还在给提示：%q", s)
	}
	// 🚨 屏幕上没有卡片的时候不许说「那张卡片还开着」—— 模型会照着去指一张
	// 不存在的卡。
	if s := helpRequestSection(coachAskHint, 1, nil); strings.Contains(s, "还开着") {
		t.Errorf("没有卡片却说卡片开着：%q", s)
	}
}

func TestHelpRequestSectionOffersAnAssistCard(t *testing.T) {
	open := &coachCard{Type: coachCardShortText, Prompt: "用一句话说说作者为什么这么写"}
	// 前两级不提「换成选择题」—— 她才按了一次，不该马上降难度。
	if s := helpRequestSection(coachAskHint, 1, open); strings.Contains(s, "choose_span") {
		t.Errorf("第 1 级就提出换成选择题：%q", s)
	}
	if s := helpRequestSection(coachAskHint, coachHintCap, open); !strings.Contains(s, "choose_span") {
		t.Errorf("第 3 级没提出换成选择题：%q", s)
	}
	// 选择题本来就是选择题，不降。
	closed := &coachCard{Type: coachCardChooseSpan, Prompt: "哪一句是作者的结论"}
	if s := helpRequestSection(coachAskHint, coachHintCap, closed); strings.Contains(s, "choose_span") {
		t.Errorf("一张选择题被提议换成选择题：%q", s)
	}
}

func TestCoachHintRoundCountsPerCard(t *testing.T) {
	card := &coachCard{Type: coachCardShortText, Prompt: "写一句"}
	hint := sqlc.AtomMessage{Role: "student", Content: coachAskHint}
	msgs := []sqlc.AtomMessage{aiWithCard(1, card)}
	if got := coachHintRound(msgs); got != 1 {
		t.Errorf("刚发了卡还没按过提示，应该是第 1 次，得到 %d", got)
	}
	msgs = append(msgs, hint, sqlc.AtomMessage{Seq: 3, Role: "ai", Content: "方向那一级"})
	if got := coachHintRound(msgs); got != 2 {
		t.Errorf("按过一次，这一轮应该是第 2 次，得到 %d", got)
	}
	msgs = append(msgs, hint, sqlc.AtomMessage{Seq: 5, Role: "ai", Content: "位置那一级"})
	if got := coachHintRound(msgs); got != 3 {
		t.Errorf("按过两次，这一轮应该是第 3 次，得到 %d", got)
	}
	// 🚨 换了一张卡，梯子从头开始 —— 不然一篇文章读到后面每张新卡开局就是最后一级。
	msgs = append(msgs, aiWithCard(6, &coachCard{Type: coachCardShortText, Prompt: "再写一句"}))
	if got := coachHintRound(msgs); got != 1 {
		t.Errorf("换了一张卡之后没有从头数：%d", got)
	}
	// 她打字说的话不是按钮，不计数。
	msgs = append(msgs, sqlc.AtomMessage{Seq: 7, Role: "student", Content: "我觉得是第三段那句"})
	if got := coachHintRound(msgs); got != 1 {
		t.Errorf("一句普通的话被数成了一次提示：%d", got)
	}
}

// 🚨 一张卡被丢掉之后，她屏幕上摆着的仍然是**再往前**那张。不接着往前找的话，
// 提示那一道闸（helpHoldsCard）从第二次提示起就失效了。
func TestLastOpenCardSurvivesADroppedCard(t *testing.T) {
	card := &coachCard{Type: coachCardShortText, Prompt: "用一句话说说作者为什么这么写"}
	msgs := []sqlc.AtomMessage{
		aiWithCard(1, card),
		{Seq: 2, Role: "student", Content: coachAskHint},
		// 这一轮 印记 写了一张卡，被丢掉了 —— payload 里只有理由。
		{Seq: 3, Role: "ai", Content: "方向那一级", Payload: coachCardPayloadWithDrop(nil, cardRejectOneBlock)},
	}
	got := lastOpenCard(msgs)
	if got == nil || got.Prompt != card.Prompt {
		t.Fatalf("丢掉一张卡之后就看不见她手上那张了：%+v", got)
	}
	// 她答了，那张卡就不开着了。
	msgs = append(msgs, sqlc.AtomMessage{
		Seq: 4, Role: "student", Content: "> 原文那一句",
		Payload: coachCardAnswerPayload(&coachCardAnswer{Type: coachCardShortText, Choice: "我的一句话"}),
	})
	if got := lastOpenCard(msgs); got != nil {
		t.Errorf("她答过了，卡片还被当成开着的：%+v", got)
	}
}
