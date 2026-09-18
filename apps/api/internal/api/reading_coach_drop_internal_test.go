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
	prompt := buildReadingCoachPrompt("标题", blocks, readingOutline{}, nil, msgs, nil, "好的。", nil, "")
	if !strings.Contains(prompt, "你上一轮递出去的东西没有到她屏幕上") {
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
	prompt := buildReadingCoachPrompt("标题", blocks, readingOutline{}, nil, msgs, nil, "好的。", nil, "")
	if !strings.Contains(prompt, "这件事不要说给她听") {
		t.Fatal("那一节没有交代「别把这件事讲给她」")
	}
}

func TestEveryRejectReasonSaysHowToFixIt(t *testing.T) {
	// 🚨 光把理由喂回去不够。第一版喂的是那句英文标识，线上实测 印记 收到之后
	// 连着六轮出同一张卡：它知道自己错了，不知道该改哪儿。
	//
	// 所以每一种理由都要有一句中文的「怎么改」。加一种理由却忘了加这一句，
	// 这条测试会红 —— 而线上只会表现成 印记 又在原地打转。
	all := []cardReject{
		cardRejectUnknownType, cardRejectPromptLen, cardRejectBannedForm,
		cardRejectFewWords, cardRejectFewOptions, cardRejectOneBlock,
		cardRejectPromised, cardRejectLensWon,
	}
	for _, why := range all {
		if strings.TrimSpace(cardFixIt[why]) == "" {
			t.Errorf("%q 没有对应的「怎么改」", why)
		}
	}
	// cardOK / cardRejectNoCard 不是失败，不该有这一句。
	for _, why := range []cardReject{cardOK, cardRejectNoCard} {
		if cardFixIt[why] != "" {
			t.Errorf("%q 不是失败，不该有「怎么改」", why)
		}
	}
}

func TestTheNoticeCarriesTheFixNotJustTheReason(t *testing.T) {
	blocks := []Block{{ID: "b1", Text: "第一段。"}, {ID: "b2", Text: "第二段。"}}
	msgs := []sqlc.AtomMessage{aiWithPayload(coachCardPayloadWithDrop(nil, cardRejectOneBlock))}
	prompt := buildReadingCoachPrompt("标题", blocks, readingOutline{}, nil, msgs, nil, "好的。", nil, "")
	if !strings.Contains(prompt, "怎么改：") {
		t.Fatal("那一节只说了理由，没说怎么改 —— 它会照着原样再出一张")
	}
	if !strings.Contains(prompt, cardFixIt[cardRejectOneBlock]) {
		t.Fatal("「怎么改」那一句和理由对不上")
	}
}

func TestReplyPromisingACardWithNoCard(t *testing.T) {
	// 🚨 另一种失败，日志里干净得可怕：模型压根没在 JSON 里给 card，却在 reply
	// 里说「我给你一张标注板」。线上实测（OSIRIS-REx 那篇）连着三轮说
	// 「标注板还在屏幕上，四句话等着你」，而它一次都没真的发出去过。
	// 对她来说这和「卡片被丢掉」长得一模一样：一句指着空气的话。
	promises := []string{
		"我给你一张标注板，把段里的四句话拖到它们该在的格子里。",
		"点这张卡，它会让你从第 2 段里挑一句。",
		"标注板还在屏幕上，四句话等着你。",
		"下面这张卡片上有三句话。",
		"把它们拖进对应的格子。",
		// 🚨 线上实测漏掉过这一句：「拖进」不是「拖句子进去」的子串。
		"我给你一块五格的板，让你把句子拖进去。",
		"下面这块板有五个格子，你把三句话各自拖到该去的那一格。",
	}
	for _, r := range promises {
		if !replyPromisesACard(r) {
			t.Errorf("没抓到指着卡片说话：%q", r)
		}
	}
}

func TestOrdinaryRepliesDoNotCountAsPromises(t *testing.T) {
	// 一张网如果把正常的话也拦下来，印记 每一轮都会收到一节关于卡片的反馈，
	// 那一节就变成常驻的了。「选一句」「找一句」在没有卡片的时候也成立 ——
	// 她可以直接在正文里划选。
	fine := []string{
		"在文章里划出你觉得最有力的那一句。",
		"回到第 3 段，找一句能说明这件事的话。",
		"你选的这一句很准，它把总量和人均分开了。",
		"这一段里有三个数字，先看带百分号的那个。",
		"用你自己的话说说，作者到底想让你接受什么。",
	}
	for _, r := range fine {
		if replyPromisesACard(r) {
			t.Errorf("误伤了一句正常的话：%q", r)
		}
	}
}

func TestReplyCutOffMidSentence(t *testing.T) {
	// 🚨 线上实测她看到的那一句，逐字：159 个字，断在「不是」上。
	cut := []string{
		"他们同样在讲封锁的后果，但说得更具体：不是",
		"现在我们往下走，看第 8 段里作者是怎么",
		"这一句的关键在于",
	}
	for _, r := range cut {
		if !replyLooksCutOff(r) {
			t.Errorf("没认出这是半句话：%q", r)
		}
	}
}

func TestWholeSentencesAreNotCutOff(t *testing.T) {
	// 一张网如果把好好说完的话也判成断句，印记 每一轮都要被多问一次 ——
	// 白烧一次旗舰调用，还会把对的那句换掉。
	whole := []string{
		"这一句选得准，它把总量和人均分开了。",
		"你觉得作者为什么要在这里放一个数字？",
		"先别看正文，只看标题！",
		"在文章里点出最能撑住他观点的那一句（不用整段）。",
		"作者说的是「可能」，不是「已经」。",
		"往下走吧……",
	}
	for _, r := range whole {
		if replyLooksCutOff(r) {
			t.Errorf("误判成半句话：%q", r)
		}
	}
}

// 她屏幕上现在摆着的那张卡片，要原样给模型看。
//
// 🚨 模型只看得见自己说过的**话**，看不见随那句话发出去的 card —— 而那张卡有时
// 根本不是它写的（标注论证那一步由服务端兜底摆板）。实测：它对着一块自己没见过
// 的板说「把主张那张换成文章里某个人亲口说的话」，而板上四句全是叙述句，一句
// 引语都没有，她照着做不到，当场卡死。
func TestOpenCardIsShownToTheCoach(t *testing.T) {
	card := &coachCard{
		Type:   coachCardLabelRoles,
		Prompt: "这几句各自在论证里扮演什么角色？",
		Options: []coachCardOption{
			{BlockID: "b1", Quote: "第一段那句话。"},
			{BlockID: "b2", Quote: "第二段那句话。"},
		},
		Labels: coachArgueBinsBasic,
	}
	blocks := []Block{{ID: "b1", Text: "第一段那句话。"}, {ID: "b2", Text: "第二段那句话。"}}
	msgs := []sqlc.AtomMessage{aiWithPayload(coachCardPayloadWithDrop(card, cardOK))}
	prompt := buildReadingCoachPrompt("标题", blocks, readingOutline{}, nil, msgs, nil, "好的。", nil, "")

	if !strings.Contains(prompt, "她屏幕上现在摆着这张卡片") {
		t.Fatal("prompt 里没有那一节 —— 模型看不见自己递出去的东西")
	}
	for _, o := range card.Options {
		if !strings.Contains(prompt, o.Quote) {
			t.Errorf("板上这一句没给它看：%q", o.Quote)
		}
	}
	if !strings.Contains(prompt, "只能要求她用板上真有的东西") {
		t.Error("没告诉它别让她去找板上没有的东西")
	}
	// 段号要说出来 —— 它对她说话时只能说「第几段」。
	if !strings.Contains(prompt, "第1段") {
		t.Error("没标出这一句在第几段")
	}
}

func TestAnsweredCardIsNotShownAgain(t *testing.T) {
	// 她答过了，那一轮的作答本来就在转写里；再把卡片贴一遍只会让它重提旧事。
	card := &coachCard{Type: coachCardShortText, Prompt: "说说看。"}
	msgs := []sqlc.AtomMessage{
		aiWithPayload(coachCardPayloadWithDrop(card, cardOK)),
		{
			Role:    "student",
			Content: "我说完了。",
			Payload: coachCardAnswerPayload(&coachCardAnswer{Type: coachCardShortText, Choice: "我说完了。"}),
		},
	}
	if got := lastOpenCard(msgs); got != nil {
		t.Fatalf("这张卡她已经答过了，不该再贴给模型：%+v", got)
	}
}

func TestNoOpenCardOnAPlainTurn(t *testing.T) {
	msgs := []sqlc.AtomMessage{{Role: "ai", Content: "我们看第三段。"}}
	if got := lastOpenCard(msgs); got != nil {
		t.Fatalf("这一轮没有卡片：%+v", got)
	}
}

func TestNoDropNoticeOnAnOrdinaryTurn(t *testing.T) {
	blocks := []Block{{ID: "b1", Text: "第一段。"}}
	msgs := []sqlc.AtomMessage{{Role: "ai", Content: "我们看第一段。"}}
	prompt := buildReadingCoachPrompt("标题", blocks, readingOutline{}, nil, msgs, nil, "好的。", nil, "")
	if strings.Contains(prompt, "你上一轮递出去的东西没有到她屏幕上") {
		t.Fatal("这一轮什么都没被丢掉，不该出现那一节")
	}
}

// 🚨 给了卡、但卡被校验刷掉的时候，要说**真正的**那个原因。
//
// 实测日志里出现过这么一行：「the reply promises a card but none was attached」，
// 同一行里却印着那张卡的 type 和 prompt。真原因（句子不在原文里）被盖掉了，
// 喂回给模型的修正话术也跟着说错，它下一轮只会照着错的方向改。
func TestPromiseVerdictDoesNotMaskTheRealReason(t *testing.T) {
	blocks := SplitBlocks("第一段说了一件事。\n\n第二段说了另一件事。")
	// 话里指着一张卡片说，卡也真的给了 —— 但句子是它自己编的，不在原文里。
	raw := `{"reply":"点下面这张卡片，把你的想法写下来。",
	          "card":{"type":"choose_span","prompt":"哪一句更像主张？",
	                  "options":[{"blockId":"b1","quote":"这句话原文里没有。"},
	                             {"blockId":"b2","quote":"这句也没有。"}]}}`
	got, ok := parseReadingCoachReply(raw, blocks, "en", func(string) bool { return true })
	if !ok {
		t.Fatal("这份 JSON 本身是好的，应该解析得出来")
	}
	if got.cardWhy == cardRejectPromised {
		t.Fatal("真原因被「提了卡却没给」盖掉了 —— 卡是给了的")
	}
	if got.cardWhy == cardOK {
		t.Fatal("原文里没有的句子应该被刷掉")
	}
}

func TestPromiseVerdictStillFiresWhenNoCardCame(t *testing.T) {
	blocks := SplitBlocks("第一段说了一件事。\n\n第二段说了另一件事。")
	got, ok := parseReadingCoachReply(`{"reply":"点下面这张卡片，把你的想法写下来。"}`, blocks, "en", func(string) bool { return true })
	if !ok {
		t.Fatal("解析失败")
	}
	if got.cardWhy != cardRejectPromised {
		t.Fatalf("话里指着一张不存在的卡片，应该判 promised，拿到 %q", got.cardWhy)
	}
}

// 🚨 讲完就停、什么也没请她做的那一轮，要被认出来。
//
// 实测她逐字报的：「它说完了 scramble 的意思，显示了 1/8，但没有告诉我下一步
// 要做什么，发送按钮也是灰的，我不知道该继续等还是要点别的地方。」
func TestDeadTurnIsCaught(t *testing.T) {
	blocks := SplitBlocks("第一段说了一件事。\n\n第二段说了另一件事。")
	dead := `{"reply":"scramble 在这里是「手忙脚乱地赶着做」的意思。它常用来写救援现场。"}`
	got, ok := parseReadingCoachReply(dead, blocks, "en", func(string) bool { return true })
	if !ok {
		t.Fatal("解析失败")
	}
	if got.cardWhy != cardRejectDeadTurn {
		t.Fatalf("这一轮什么都没请她做，应该判 dead turn，拿到 %q", got.cardWhy)
	}
}

func TestATurnThatAsksForSomethingIsFine(t *testing.T) {
	blocks := SplitBlocks("第一段说了一件事。\n\n第二段说了另一件事。")
	for _, reply := range []string{
		`{"reply":"scramble 是「手忙脚乱地赶着做」。你觉得第 2 段里谁在 scramble？"}`,
		`{"reply":"scramble 是「手忙脚乱地赶着做」。请在第 2 段里找出那个动作。"}`,
		`{"reply":"这一段讲完了，请接着读第 3 段，读完告诉我它在回应上一段的哪一句。","advance":"done"}`,
	} {
		got, ok := parseReadingCoachReply(reply, blocks, "en", func(string) bool { return true })
		if !ok {
			t.Fatalf("解析失败：%s", reply)
		}
		if got.cardWhy == cardRejectDeadTurn {
			t.Errorf("这一轮是有下文的，不该判 dead turn：%s", reply)
		}
	}
}

// 🚨 推进了一步不算给了她事做。
//
// 实测：「它说我选得准、推进到下一段了，但是下面没有任何新题目或者按钮让我
// 继续，发送也按不动。」下一步要她先开口，而她手上没有任何东西可说。
func TestAdvancingWithoutAnAskIsStillADeadTurn(t *testing.T) {
	blocks := SplitBlocks("第一段说了一件事。\n\n第二段说了另一件事。")
	raw := `{"reply":"你选得准。我们进到下一段。","advance":"done"}`
	got, ok := parseReadingCoachReply(raw, blocks, "en", func(string) bool { return true })
	if !ok {
		t.Fatal("解析失败")
	}
	if got.cardWhy != cardRejectDeadTurn {
		t.Fatalf("推进了但什么都没请她做，应该判 dead turn，拿到 %q", got.cardWhy)
	}
}

// 🚨 递透镜的那一轮，话里要当着她的面把这套看法做一遍 —— 拿原文的一句。
//
// 实测她逐字报的：「它一直让我用一副『透镜』去拆句子，但从来没给我看过这副
// 透镜是什么、怎么用。前面说要先演示一遍给我看，结果什么都没有。」
// 以及：「我真的不懂这些词是什么意思，我只是个高中生。」
//
// 方法名照说（印记 要用真的方法名），但光有名字没有示范，那个名字对她就是
// 一个生词。判据取能验的那一个：这一轮的话里有没有一段逐字来自落点段的原文。
func TestLensTurnMustDemonstrateOnARealSentence(t *testing.T) {
	blocks := SplitBlocks("The agency said the blockade had made every delivery slower. " +
		"Officials cautioned that the figure could not be independently verified." +
		"\n\n第二段讲了别的事情。")
	lensOK := func(string) bool { return true }

	// 只说了方法名，没引原文 —— 空谈。
	empty := `{"reply":"我们用传播学的角度看第 1 段，注意表达方式怎么影响判断。",
	            "lens":"lens-communication","focusBlock":"b1"}`
	got, ok := parseReadingCoachReply(empty, blocks, "en", lensOK)
	if !ok {
		t.Fatal("解析失败")
	}
	if !got.lensRetry {
		t.Fatal("这一轮没有示范，应该认出来")
	}
	// 🚨 透镜不能因此被丢掉 —— 丢了她就只剩几个生词而没有工具。
	if got.Lens == "" {
		t.Fatal("透镜被丢掉了；这一条只该让这一轮重来，不该丢东西")
	}

	// 引了原文那一句 —— 这才是示范。
	demo := `{"reply":"看第 1 段这句：Officials cautioned that the figure could not be independently verified。cautioned 和 could not be verified 把这条数字的分量压下来了。",
	           "lens":"lens-communication","focusBlock":"b1"}`
	got2, ok2 := parseReadingCoachReply(demo, blocks, "en", lensOK)
	if !ok2 {
		t.Fatal("解析失败")
	}
	if got2.lensRetry {
		t.Fatal("这一轮引了原句，是做过示范的")
	}
}

// 🚨 一轮里递了透镜，话里却在说板 —— 她照着话去做，做不成。
//
// 铁律③ 一次只交给她一件事：透镜在的时候卡片会被丢掉，于是「把这句挪到证据
// 那个格子里」指向的东西根本不存在。实测她逐字报的：「它让我把句子挪到『证据』
// 那个格子里，但我现在看不到任何可以拖拽的板子或卡片，只有文本框。」
func TestLensTurnThatTalksAboutABoardIsRetried(t *testing.T) {
	blocks := SplitBlocks("The agency said the blockade had made every delivery slower. " +
		"Officials cautioned that the figure could not be independently verified." +
		"\n\n第二段讲了别的事情。")
	raw := `{"reply":"看第 1 段这句：Officials cautioned that the figure could not be independently verified。现在把它挪到「证据」那个格子里。",
	          "lens":"lens-methods","focusBlock":"b1"}`
	got, ok := parseReadingCoachReply(raw, blocks, "en", func(string) bool { return true })
	if !ok {
		t.Fatal("解析失败")
	}
	if !got.lensRetry {
		t.Fatal("递了透镜却在说板，应该重来一次")
	}
	if got.Lens == "" {
		t.Fatal("透镜不该被丢掉 —— 这一条只让这一轮重来")
	}
}

// 🚨 递透镜的那一轮，话不要以一个问句收尾。
//
// 透镜自己就是那句「请她做什么」。话里再抛一个问题，屏幕上就有了两件事，而
// 它们要的动作不一样。实测她逐字报的：「我不知道到底是要我从第12段 pick 一句
// 英文，还是在下面那个框里用中文写答案。」
func TestLensTurnMustNotEndOnAQuestion(t *testing.T) {
	blocks := SplitBlocks("The agency said the blockade had made every delivery slower. " +
		"Officials cautioned that the figure could not be independently verified." +
		"\n\n第二段讲了别的事情。")
	lensOK := func(string) bool { return true }
	quote := "Officials cautioned that the figure could not be independently verified"

	asks := `{"reply":"看第 1 段这句：` + quote + `。cautioned 把这条数字的分量压下来了。你觉得哪个成本被漏掉了？",
	           "lens":"lens-economics","focusBlock":"b1"}`
	got, ok := parseReadingCoachReply(asks, blocks, "en", lensOK)
	if !ok {
		t.Fatal("解析失败")
	}
	if !got.lensRetry {
		t.Fatal("以问句收尾，应该重来一次")
	}

	hands := `{"reply":"看第 1 段这句：` + quote + `。cautioned 把这条数字的分量压下来了。现在换你，在文章别处找一句这样的。",
	           "lens":"lens-economics","focusBlock":"b1"}`
	got2, ok2 := parseReadingCoachReply(hands, blocks, "en", lensOK)
	if !ok2 {
		t.Fatal("解析失败")
	}
	if got2.lensRetry {
		t.Fatalf("这一轮做了示范、用陈述句收尾，不该重来：%q", got2.lensRetryWhy)
	}
}

func TestReplyEndsOnAQuestion(t *testing.T) {
	for reply, want := range map[string]bool{
		"你觉得哪个成本被漏掉了？":            true,
		"Which cost is missing?":  true,
		"现在换你，在文章别处找一句这样的。":       false,
		// 中间的问号是讲解的一部分，不算。
		"这句在问什么？它在说成本。现在换你找一句。": false,
		// 收尾的引号不算数，要看引号前面那个字。
		"他问的是「哪个成本被漏掉了？」":         true,
	} {
		if got := replyEndsOnAQuestion(reply); got != want {
			t.Errorf("replyEndsOnAQuestion(%q) = %v，想要 %v", reply, got, want)
		}
	}
}

// 🚨 让她把一张卡挪到它**已经在**的那一格，是一条她做不到的指令。
//
// 实测她逐字报的：「它让我把发电机那句拖到限制格，但那句已经在限制格里了……
// 屏幕上显示的摆放和它文字描述的矛盾了，我没法确定该怎么挪。」
// 她摆完的结果原样在转写里，所以这是它没读，不是我们没给。
func TestNoOpMoveIsCaught(t *testing.T) {
	placed := map[string]string{
		"The generator has fuel for three more days.": "限制",
		"Cutting off fuel stopped the pumps.":         "证据",
	}
	if !replyAsksForANoOpMove("把 The generator has fuel for three more days 这句拖到「限制」那一格。", placed) {
		t.Error("这是一条挪不动的指令，应该抓出来")
	}
	// 挪到**别的**格子是正常的教学动作。
	if replyAsksForANoOpMove("把 The generator has fuel for three more days 这句拖到「证据」那一格。", placed) {
		t.Error("挪到别的格子是正常的，不该拦")
	}
	// 🚨 肯定她摆得对，不是指令 —— 这一条最容易误伤。
	if replyAsksForANoOpMove("你把 The generator has fuel for three more days 放在限制，这个判断很准。", placed) {
		t.Error("误伤了一句肯定")
	}
	// 没有板的时候什么都不判。
	if replyAsksForANoOpMove("把那句拖到限制。", nil) {
		t.Error("没有摆放记录时不该判")
	}
}

// 🚨 没有卡片的那一轮，断句也要判出来。
//
// 这道闸原来写的是 cardWhy == cardOK，而没有卡片的那一轮 cardWhy 是
// cardRejectNoCard —— 加上它自己要求 Card == nil，两个条件永远不会同时成立，
// 这道闸从写下来那天起一次都没响过。
//
// 线上逐字证据（atom 609f3910，2026-09-11）：「对，调查数据是一个方向。**但」，
// payload 里 dropped 是空的。产品负责人报的第 1 条就是它。
func TestCutOffIsCaughtOnACardlessTurn(t *testing.T) {
	blocks := SplitBlocks("第一段说了一件事。\n\n第二段说了另一件事。")
	raw := `{"reply":"对，调查数据是一个方向。**但"}`
	got, ok := parseReadingCoachReply(raw, blocks, "en", func(string) bool { return true })
	if !ok {
		t.Fatal("解析失败")
	}
	if got.cardWhy != cardRejectCutOff {
		t.Fatalf("半句话没被判出来，cardWhy = %q", got.cardWhy)
	}
}

// 🚨 「你先把全文读一遍」不是一件她在屏幕上交得出来的事。
//
// 实测她连着四轮说同一句：「它说『先通读一遍全文』，但我读完了不知道接下来要
// 干嘛，没有下一步的按钮。」第五轮她放弃了，整条走查停在第 6 步。
//
// prompt 里早就写着「不要以『先通读全文，读完告诉我』收尾」，它照样这么收尾 ——
// 写第三遍不如做成判据。
func TestReadOnlyTurnIsADeadTurn(t *testing.T) {
	blocks := SplitBlocks("第一段说了一件事。\n\n第二段说了另一件事。")
	lensOK := func(string) bool { return true }

	raw := `{"reply":"我们先通读一遍全文，读完跟我说一声。"}`
	got, ok := parseReadingCoachReply(raw, blocks, "en", lensOK)
	if !ok {
		t.Fatal("解析失败")
	}
	if got.cardWhy != cardRejectDeadTurn {
		t.Fatalf("只让她读、没有落点，应该判 dead turn，拿到 %q", got.cardWhy)
	}
}

func TestReadingPlusSomethingToDoIsFine(t *testing.T) {
	blocks := SplitBlocks("第一段说了一件事。\n\n第二段说了另一件事。")
	lensOK := func(string) bool { return true }
	for _, reply := range []string{
		// 读 + 一个她交得出来的动作。
		`{"reply":"先通读一遍全文，然后在文章里划出你最不服气的那一句。"}`,
		// 带着卡片说「先读一遍再点」是正常的 —— 那一轮她手上有东西。
		`{"reply":"先通读一遍全文，再看下面这张卡。","card":{"type":"short_text","prompt":"读完之后，你最想问作者什么？"}}`,
	} {
		got, ok := parseReadingCoachReply(reply, blocks, "en", lensOK)
		if !ok {
			t.Fatalf("解析失败：%s", reply)
		}
		if got.cardWhy == cardRejectDeadTurn {
			t.Errorf("这一轮她有东西可做，不该判 dead turn：%s", reply)
		}
	}
}

// 🚨 它说了有卡，她屏幕上就必须有卡。
//
// 产品负责人 2026-09-12 定的线：「不应该让用户有 bug 的感觉。要么不满足自己
// 不调用，要么就是有兜底策略。」她逐字说过的那一幕是 印记 连着两轮道歉
// 「卡没送到你手里」，她连着两轮回「没有卡啊」。
func TestFallbackCardCannotBeRejected(t *testing.T) {
	blocks := SplitBlocks("第一段说了一件事，句子够长可以上卡。\n\n第二段说了另一件事，也够长。")

	// 兜底那张必须**过得了**校验 —— 它要是也能被驳回，就不叫兜底。
	got := fallbackCardFor("哪一句最能说明援助进不去？", "我们来看这几段。", "")
	if kept, why := validateCoachCardWhy(got, blocks); kept == nil {
		t.Fatalf("兜底卡被驳回了，理由 %q —— 那它就不是兜底", why)
	}
	if got.Type != coachCardPickInArticle {
		t.Errorf("兜底只能是 pick_in_article / short_text（没有 options 就没有对不上原文这回事），拿到 %q", got.Type)
	}
	// 印记 自己那道题要留住 —— 她看到的是它问的话，不是我们编的。
	if got.Prompt != "哪一句最能说明援助进不去？" {
		t.Errorf("它自己那道题没留住：%q", got.Prompt)
	}
}

// 兜底卡没有选项；一道「分析下列句子，判断它们各自属于……」照抄上去，她面对的是
// 一张要她分类、却一句话都没摆的卡（2026-09-17 入口走查）。
func TestFallbackCardDropsAPromptThatNeedsOptions(t *testing.T) {
	got := fallbackCardFor("分析下列句子，判断它们各自属于AI做的还是人做的。", "我们来看第7段。", "")
	if got != nil && (strings.Contains(got.Prompt, "下列") || strings.Contains(got.Prompt, "各自")) {
		t.Errorf("兜底卡照抄了一道要选项的题：%q", got.Prompt)
	}
	if kept := fallbackCardFor("哪一句最能说明援助进不去？", "我们来看这几段。", ""); kept.Prompt != "哪一句最能说明援助进不去？" {
		t.Errorf("不需要选项的题应该留着：%q", kept.Prompt)
	}
}

// 它没写题：用这一步说明里的那个问句；这一步也没有问句，就不兜。
//
// 🚨 2026-09-18 以前这里兜的是一句中性的「请在文章里点出你想说的那一句。」同事：
// 「完全没看懂这个卡片在干嘛」—— 一张不问任何事的卡比没有卡更糟。
func TestFallbackCardWhenItNeverWroteAQuestion(t *testing.T) {
	blocks := SplitBlocks("第一段说了一件事，句子够长可以上卡。\n\n第二段说了另一件事，也够长。")
	step := &sqlc.ReadingTask{Kind: "hunt", Detail: "回到文章里：作者直接表明中心论点的是哪一句？把它点出来，对照你刚才的总结。"}
	got := fallbackCardFor("", "我们来看这几段。", stepQuestion(step))
	if got == nil {
		t.Fatal("这一步的说明里有问句，应该拿它当题")
	}
	if got.Prompt != "作者直接表明中心论点的是哪一句？" {
		t.Errorf("题目应该是说明里那一问，拿到 %q", got.Prompt)
	}
	if kept, why := validateCoachCardWhy(got, blocks); kept == nil {
		t.Fatalf("兜底卡必须过得了校验，理由 %q", why)
	}
	if fb := fallbackCardFor("", "我们来看这几段。", stepQuestion(&sqlc.ReadingTask{Detail: "请打开段落工具，把它拆开。"})); fb != nil {
		t.Errorf("没有任何一道题时不该兜一张卡，拿到 %q", fb.Prompt)
	}
	// 超长的那一道也不能原样塞回去 —— 它自己会被 promptLen 驳回。
	long := strings.Repeat("很", 200)
	if fb := fallbackCardFor(long, "我们来看这几段。", ""); fb != nil {
		t.Errorf("题目超长、又没有步骤问句时不该兜卡，拿到 %q", fb.Prompt)
	}
}

// 🚨 兜底那张卡要她做的事，必须和话里说的是同一件事。
//
// 线上实测（2026-09-17，那一轮部署完之后立刻走的）：印记 说
//
//	「下面那张卡上写几个字就行：你觉得这篇报道接下来会讲哪几类消息？」
//
// 而兜出来的卡片写着「请在文章里点出你想说的那一句。」—— 一个要她打字，
// 一个要她点句子。产品负责人报的第 2 条是「对话框指令和动手部分的指令不一致」，
// 而这一句不一致是**我们自己写的**，不是模型写的。
func TestFallbackCardFollowsTheVerbInTheReply(t *testing.T) {
	blocks := SplitBlocks("第一段说了一件事，句子够长可以上卡。\n\n第二段说了另一件事，也够长。")
	cases := []struct {
		name  string
		reply string
		want  string
	}{
		{"请她写", "下面那张卡上写几个字就行：你觉得接下来会讲哪几类消息？", coachCardShortText},
		{"请她用自己的话说", "在卡片上用你自己的话说一遍。", coachCardShortText},
		{"请她点句子", "在下面那张卡片上挑一句。", coachCardPickInArticle},
		{"没说清楚", "我们来看这几段。", coachCardPickInArticle},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := fallbackCardFor("", tc.reply, "文中哪一句最能说明这件事？")
			if got.Type != tc.want {
				t.Errorf("话里说的是「%s」，兜出来的却是 %q —— 她照着话去做，做不成",
					tc.reply, got.Type)
			}
			// 两种形状都必须仍然过得了校验，否则它就不是兜底。
			if kept, why := validateCoachCardWhy(got, blocks); kept == nil {
				t.Errorf("兜底卡被驳回了，理由 %q", why)
			}
		})
	}
}
