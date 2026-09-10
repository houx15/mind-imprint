package api

import (
	"strings"
	"testing"

	"mindimprint/api/internal/store/sqlc"
)

// 卡片没发出去，下一轮要当面告诉模型。
//
// 🚨 这是「闭环」的失败那一侧。线上实测：印记 连着六轮在说「点这张卡，它会让你
// 从第 2 段里挑一句」，而那张卡每一轮都被跨段落那条规则丢掉 —— 她屏幕上只有
// 一句句指着空气的话。模型自己发现不了：它写完就交出去了，下一轮的上文里只有
// 它说过的话，看不出卡片有没有到。

func aiWithPayload(payload []byte) sqlc.AtomMessage {
	return sqlc.AtomMessage{Role: "ai", Content: "点这张卡。", Payload: payload}
}

func TestDroppedReasonSurvivesTheRoundTrip(t *testing.T) {
	// 存进 payload 的理由，读回来还是同一句。这两半分别写在
	// reading_coach.go 和 reading_coach_card.go 里，很容易只改一半。
	raw := coachCardPayloadWithDrop(nil, cardRejectOneBlock)
	if len(raw) == 0 {
		t.Fatal("卡片被丢掉的时候必须写 payload，否则理由传不到下一轮")
	}
	got := lastDroppedCard([]sqlc.AtomMessage{aiWithPayload(raw)})
	if got != string(cardRejectOneBlock) {
		t.Fatalf("读回来的理由 = %q，想要 %q", got, cardRejectOneBlock)
	}
}

func TestNoPayloadWhenThereWasNoCardAtAll(t *testing.T) {
	// 大多数轮本来就没有卡片，那不是失败，不该写 payload —— 写了的话
	// lastDroppedCard 每一轮都会看到一条空理由，那一节就变成常驻的了。
	if raw := coachCardPayloadWithDrop(nil, cardOK); raw != nil {
		t.Fatalf("没有卡片也没有理由，不该写 payload：%s", raw)
	}
	if raw := coachCardPayloadWithDrop(nil, cardRejectNoCard); raw != nil {
		t.Fatalf("「这一轮没给卡片」不是失败，不该写 payload：%s", raw)
	}
}

func TestCardThatSurvivedReportsNoDrop(t *testing.T) {
	card := &coachCard{Type: coachCardShortText, Prompt: "说说看你的判断。"}
	raw := coachCardPayloadWithDrop(card, cardOK)
	if len(raw) == 0 {
		t.Fatal("卡片发出去了，payload 里要有它")
	}
	if got := lastDroppedCard([]sqlc.AtomMessage{aiWithPayload(raw)}); got != "" {
		t.Fatalf("卡片好好发出去了，不该有理由：%q", got)
	}
}

func TestOnlyTheLastTurnCounts(t *testing.T) {
	// 再往前的那些它已经收到过反馈了 —— 每一轮都翻旧账，那一节就变成常驻的，
	// 而常驻的提示会抢掉这一轮真正该做的事。
	dropped := aiWithPayload(coachCardPayloadWithDrop(nil, cardRejectFewOptions))
	fine := aiWithPayload(coachCardPayloadWithDrop(
		&coachCard{Type: coachCardShortText, Prompt: "说说看。"}, cardOK))
	msgs := []sqlc.AtomMessage{dropped, {Role: "student", Content: "好。"}, fine}
	if got := lastDroppedCard(msgs); got != "" {
		t.Fatalf("上一轮的卡片是好的，不该报上上轮的账：%q", got)
	}
}

func TestBadPayloadIsNotAReason(t *testing.T) {
	// 一条读不动的 payload 不该被当成「卡片被丢了」——那会让 印记 收到一节
	// 关于一张它从没写过的卡片的反馈。
	for _, raw := range [][]byte{nil, []byte(""), []byte("{"), []byte("null")} {
		if got := lastDroppedCard([]sqlc.AtomMessage{aiWithPayload(raw)}); got != "" {
			t.Fatalf("payload=%q 不该产出理由，拿到 %q", raw, got)
		}
	}
}

func TestDropNoticeTellsItNotToMentionTheCard(t *testing.T) {
	// 那一节要说清楚**她看不到那张卡**。只说「卡片没发出去」的话，模型下一轮
	// 会接着说「点上面那张卡」——线上就是这么连着六轮的。
	blocks := []Block{{ID: "b1", Text: "第一段。"}, {ID: "b2", Text: "第二段。"}}
	msgs := []sqlc.AtomMessage{aiWithPayload(coachCardPayloadWithDrop(nil, cardRejectOneBlock))}
	prompt := buildReadingCoachPrompt("标题", blocks, readingOutline{}, nil, msgs, nil, "好的。", nil)
	if !strings.Contains(prompt, "你上一轮那张卡片没有发出去") {
		t.Fatal("prompt 里没有那一节")
	}
	if !strings.Contains(prompt, string(cardRejectOneBlock)) {
		t.Fatal("那一节里没有具体理由 —— 只说「没发出去」它改不动")
	}
	if !strings.Contains(prompt, "不要再提「这张卡」") {
		t.Fatal("那一节没告诉它别再指着一张她看不见的卡")
	}
}

func TestDropNoticeIsNotForHerEars(t *testing.T) {
	// 🚨 线上实测：印记 把这件事原样念给了她听 ——「上一轮那张卡没发出去，
	// 它卡在选项全来自同一段，系统不收」。她不需要知道我们这边有校验、有规则，
	// 说出来只会让她觉得这个房间在出故障。
	blocks := []Block{{ID: "b1", Text: "第一段。"}, {ID: "b2", Text: "第二段。"}}
	msgs := []sqlc.AtomMessage{aiWithPayload(coachCardPayloadWithDrop(nil, cardRejectOneBlock))}
	prompt := buildReadingCoachPrompt("标题", blocks, readingOutline{}, nil, msgs, nil, "好的。", nil)
	if !strings.Contains(prompt, "这件事不要说给她听") {
		t.Fatal("那一节没有交代「别把这件事讲给她」")
	}
}

func TestNoDropNoticeOnAnOrdinaryTurn(t *testing.T) {
	blocks := []Block{{ID: "b1", Text: "第一段。"}}
	msgs := []sqlc.AtomMessage{{Role: "ai", Content: "我们看第一段。"}}
	prompt := buildReadingCoachPrompt("标题", blocks, readingOutline{}, nil, msgs, nil, "好的。", nil)
	if strings.Contains(prompt, "你上一轮那张卡片没有发出去") {
		t.Fatal("这一轮什么都没被丢掉，不该出现那一节")
	}
}
