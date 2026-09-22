package api

// reading_coach_consistency_internal_test.go —— 「屏幕上的两句话不许打架」
// 那一组判据，来自产品负责人 2026-09-17 的走查。
//
// 三处，三件同样的事：她眼前同时出现两样东西，而它们说的不是一回事。
//
//  1. 话里问一个问题，同一轮的卡片问**另一个** —— 她不知道该答哪一个。
//  2. 复核当着她的面判了「这一句撑不住」，紧接着 印记 夸她选得准。
//  3. 一块标注板上四句话摆在一起，不说各自从哪一段来 —— 她没法判断哪句撑哪句。

import (
	"strings"
	"testing"

	"mindimprint/api/internal/store/sqlc"
)

func consistencyBlocks() []Block {
	return []Block{
		{ID: "b1", Text: "中国曾有“君子不失色于人”的古训，意思是说，有道德的人待人应该彬彬有礼。"},
		{ID: "b2", Text: "《说岳全传》上有这么一段：牛皋向一位老者问路，他在马上吼道。"},
		{ID: "b3", Text: "礼貌待人可以在人与人之间架起一座理解的桥梁，减少相互间的矛盾。"},
	}
}

// 发了卡片、话又以另一个问句收尾 → 这一轮标记成要重来一次。
//
// 产品负责人的原话：「卡片内容上的要求和对话窗口的文本要求不一致。」
func TestTwoAsks_CardPlusATrailingQuestion(t *testing.T) {
	blocks := consistencyBlocks()
	raw := `{"reply":"你选的是第3段那句。现在请看看这三句——你觉得古训和俗话，跟第3句在让人相信这件事上，有什么不一样？",` +
		`"advance":"","focusBlock":"","tool":"","lens":"",` +
		`"card":{"type":"choose_span","prompt":"哪一句读起来最不像在讲道理，更像在讲一个不变的事实？",` +
		`"options":[{"blockId":"b1","quote":"中国曾有“君子不失色于人”的古训，意思是说，有道德的人待人应该彬彬有礼。"},` +
		`{"blockId":"b3","quote":"礼貌待人可以在人与人之间架起一座理解的桥梁，减少相互间的矛盾。"}]}}`
	got, ok := parseReadingCoachReply(raw, blocks, "zh", func(string) bool { return true })
	if !ok {
		t.Fatal("the fixture must parse — the test is about twoAsks, not parsing")
	}
	if got.Card == nil {
		t.Fatalf("the card itself is fine and must survive; cardWhy=%q", got.cardWhy)
	}
	if !got.twoAsks {
		t.Error("a card plus a trailing question is two asks on one screen; want a retry")
	}
}

// 同一张卡片，话以陈述句收尾 → 不重来。这条守的是「别把正常的一轮也拖去重问」。
func TestTwoAsks_CardWithAStatementIsFine(t *testing.T) {
	blocks := consistencyBlocks()
	raw := `{"reply":"你选的是第3段那句，那是作者直接亮出来的道理。下面这张卡上的两句来自不同的段落。",` +
		`"advance":"","focusBlock":"","tool":"","lens":"",` +
		`"card":{"type":"choose_span","prompt":"作者是怎么让你相信礼貌值得讲的？",` +
		`"options":[{"blockId":"b1","quote":"中国曾有“君子不失色于人”的古训，意思是说，有道德的人待人应该彬彬有礼。"},` +
		`{"blockId":"b3","quote":"礼貌待人可以在人与人之间架起一座理解的桥梁，减少相互间的矛盾。"}]}}`
	got, ok := parseReadingCoachReply(raw, blocks, "zh", func(string) bool { return true })
	if !ok {
		t.Fatal("fixture must parse")
	}
	if got.twoAsks {
		t.Errorf("a card with a statement ending is one ask, not two: %q", got.Reply)
	}
}

// 没有卡片的那一轮照常可以用问句收尾 —— 那时候问题**就是**这一轮要她做的事。
// 🚨 这条是 [[optimizing-a-detector-made-coaching-worse-2026-09-14]] 的护栏：
// 「一段只能有一个问号」那次把陪练问问题这件事整个关掉了，判官分数当场下跌。
// 这里管的只有「卡片已经在问了，话就别再问第二个」这一种。
func TestTwoAsks_NoCardMayStillAskAQuestion(t *testing.T) {
	blocks := consistencyBlocks()
	raw := `{"reply":"第3段那句是作者自己的道理。你觉得它比前面那句古训强在哪儿？",` +
		`"advance":"","focusBlock":"","tool":"","lens":""}`
	got, ok := parseReadingCoachReply(raw, blocks, "zh", func(string) bool { return true })
	if !ok {
		t.Fatal("fixture must parse")
	}
	if got.twoAsks {
		t.Error("a turn with no card must be free to end on its own question")
	}
}

// 卡片上的每个选项都带着「第几段」，而且是服务端数的那个号。
//
// 产品负责人：「对整体拆分时，选择的都是单句，并未标注段落，有时候单独的句子
// 拆出来很难看出属于什么部分。」
func TestCardOptionsCarryTheParagraphNumber(t *testing.T) {
	blocks := consistencyBlocks()
	card := &coachCard{
		Type:   coachCardChooseSpan,
		Prompt: "作者是怎么让你相信礼貌值得讲的？",
		Options: []coachCardOption{
			// 🚨 模型自己塞的 where 必须被覆盖掉：这个号码只能由服务端数。
			{BlockID: "b3", Quote: "礼貌待人可以在人与人之间架起一座理解的桥梁，减少相互间的矛盾。", Where: "第99段"},
			{BlockID: "b1", Quote: "中国曾有“君子不失色于人”的古训，意思是说，有道德的人待人应该彬彬有礼。"},
		},
	}
	out := validateCoachCard(card, blocks)
	if out == nil {
		t.Fatal("the card should have survived validation")
	}
	want := map[string]string{"b1": "第1段", "b3": "第3段"}
	for _, o := range out.Options {
		if o.Where != want[o.BlockID] {
			t.Errorf("option from %s says %q, want %q", o.BlockID, o.Where, want[o.BlockID])
		}
	}
}

// 复核的结论跟着回灌，而且带着「她已经看过了」这句话。
//
// 产品负责人：「句子匹配不通过，但是点击记录发现后，主 ai 又给出了不一样的回答。」
func TestLensDoneCarriesTheVerdict(t *testing.T) {
	done := &readingLensDone{
		CardName:      "伦理学",
		Quote:         "礼貌待人可以在人与人之间架起一座理解的桥梁，减少相互间的矛盾。",
		Finding:       "这一句说的是好处，不是一次价值取舍。",
		Verdict:       "rethink",
		VerdictReason: "这一句里没有「应该」，也没有谁的什么被压下去。",
	}
	prompt := buildReadingCoachPrompt("谈礼貌", consistencyBlocks(), readingOutline{}, nil, nil, nil, "", done, "")
	if !strings.Contains(prompt, verdictWord[done.Verdict]) {
		t.Errorf("the verdict never reached the prompt:\n%s", prompt)
	}
	if !strings.Contains(prompt, done.VerdictReason) {
		t.Errorf("the verdict's reason never reached the prompt:\n%s", prompt)
	}
	if !strings.Contains(prompt, "她屏幕上已经看到了这一句") {
		t.Errorf("the prompt must say she already read it, or 印记 feels free to contradict it:\n%s", prompt)
	}
}

// 客户端没发 verdict（老版本），或者发了一个我们不认识的值 → 一个字都不加。
// 宁可少一句，不要一句编的。
func TestLensDoneWithoutAVerdictSaysNothing(t *testing.T) {
	for _, v := range []string{"", "  ", "brilliant"} {
		done := &readingLensDone{
			CardName: "伦理学",
			Quote:    "礼貌待人可以在人与人之间架起一座理解的桥梁，减少相互间的矛盾。",
			Finding:  "这一句说的是好处。",
			Verdict:  v,
		}
		prompt := buildReadingCoachPrompt("谈礼貌", consistencyBlocks(), readingOutline{}, nil, nil, nil, "", done, "")
		if strings.Contains(prompt, "你当时给出的结论") {
			t.Errorf("verdict %q must produce no verdict line:\n%s", v, prompt)
		}
	}
}

// 透镜开着的那一轮，prompt 要当面说「不要领新的一步」。
//
// 产品负责人：「找不到句子的时候，没法在聊天框打字求助 ai。」输入框放开之后，
// 模型必须知道屏幕上还开着一副透镜 —— 否则她一开口它就往下走。
func TestOpenLensSectionStopsTheCoachFromMovingOn(t *testing.T) {
	line := openLensLine(true, "伦理学：哪些价值正在冲突？")
	if !strings.Contains(line, "伦理学") {
		t.Errorf("the open lens must be named:\n%s", line)
	}
	if !strings.Contains(line, "不要领新的一步") {
		t.Errorf("the section must stop the coach advancing:\n%s", line)
	}
	if !strings.Contains(line, "先信她") {
		t.Errorf("「文章里没有这种句子」时先信她，是这一节存在的理由之一:\n%s", line)
	}
	if openLensLine(false, "伦理学") != "" {
		t.Error("no open lens must add nothing at all")
	}

	tasks := []sqlc.ReadingTask{{Status: "pending", Kind: "lens", Label: "深入思考"}}
	prompt := buildReadingCoachPrompt("谈礼貌", consistencyBlocks(), readingOutline{}, tasks, nil, nil,
		"我找不到这样的句子", nil, openLensLine(true, "伦理学"))
	// 🚨 它必须排在推进判据**后面**：那一节说「她做完了就 done」，而这一节
	// 取消它。最后一节才是这一轮真正的指令。
	iRule := strings.Index(prompt, "【本轮推进判据】")
	iOpen := strings.Index(prompt, "【她屏幕上正开着")
	if iRule < 0 || iOpen < 0 || iOpen < iRule {
		t.Errorf("the open-lens section must come AFTER the advance rule (rule=%d open=%d)", iRule, iOpen)
	}
}
