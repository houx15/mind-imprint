package api

import (
	"strings"
	"testing"
)

// 服务端兜底摆出来的那块板。
//
// 🚨 它存在的理由：「标注论证」那一步的全部内容就是那块板，而模型一遍遍在话里
// 说「把这三句拖到格子里」却不附 card。检测、重试、把理由喂回去都只是降低概率。
// 这一步不需要模型来决定「有没有板」—— 读法库已经规定了它是标注论证。

func buildBlocks() []Block {
	return SplitBlocks(strings.Join([]string{
		"Aid groups are scrambling to help people caught in the war. They say the blockade has made every delivery slower.",
		"The agency said it had delivered 40 trucks of supplies last week. Officials cautioned that the figure could not be independently verified.",
		"短。",
	}, "\n\n"))
}

func TestBuiltBoardQuotesTheArticleVerbatim(t *testing.T) {
	blocks := buildBlocks()
	got := buildLabelBoard(blocks, "b2")
	if got == nil {
		t.Fatal("这篇文章里挑得出句子，不该返回 nil")
	}
	if got.Type != coachCardLabelRoles {
		t.Fatalf("type = %q", got.Type)
	}
	// 🚨 每一句都必须逐字在它标的那一段里 —— 兜底出来的板要和模型给的板守同一条
	// 规矩，否则它就成了一条绕过校验的后门。
	byID := map[string]string{}
	for _, b := range blocks {
		byID[b.ID] = b.Text
	}
	for _, o := range got.Options {
		if !strings.Contains(byID[o.BlockID], o.Quote) {
			t.Errorf("这一句不在 %s 里：%q", o.BlockID, o.Quote)
		}
	}
	// 它还要真的过得了那道校验 —— 调用点就是这么用的。
	if validateCoachCard(got, blocks) == nil {
		t.Fatal("兜底摆出来的板过不了 validateCoachCard")
	}
}

func TestBuiltBoardPrefersTheFocusParagraph(t *testing.T) {
	// 印记 刚讲的是第 2 段，板上第一句就该来自第 2 段 —— 否则她得满篇去找。
	got := buildLabelBoard(buildBlocks(), "b2")
	if got == nil || len(got.Options) == 0 {
		t.Fatal("没摆出板")
	}
	if got.Options[0].BlockID != "b2" {
		t.Fatalf("第一句来自 %s，想要落点段 b2", got.Options[0].BlockID)
	}
}

func TestBuiltBoardSkipsSentencesTooShortToLabel(t *testing.T) {
	// 「短。」贴不上任何角色。
	got := buildLabelBoard(buildBlocks(), "")
	if got == nil {
		t.Fatal("没摆出板")
	}
	for _, o := range got.Options {
		if o.Quote == "短。" {
			t.Fatal("太短的句子不该上板")
		}
	}
}

func TestBuiltBoardGivesUpRatherThanShipEmpty(t *testing.T) {
	// 挑不出两句就不要板 —— 一块空板比没有板更糟。
	thin := SplitBlocks("短。\n\n也短。")
	if got := buildLabelBoard(thin, ""); got != nil {
		t.Fatalf("挑不出句子时应该返回 nil，拿到 %+v", got.Options)
	}
	if got := buildLabelBoard(nil, ""); got != nil {
		t.Fatal("没有正文时应该返回 nil")
	}
}

// 🚨 板要摆在 印记 嘴上说的那一段。
//
// focusBlock 和这一步自带的 BlockID 经常都是空的，兜底就从第 1 段取句子 ——
// 而它那句话说的是「我们来摆第 5 段的这几句」。实测她逐字报的：
// 「它说让我摆第5段的句子，但板上给的卡片全是第1段的。」
func TestSpokenParagraphWinsOverEmptyFields(t *testing.T) {
	blocks := SplitBlocks(strings.Join([]string{
		"一段。", "二段。", "三段。", "四段。", "五段。", "六段。",
	}, "\n\n"))
	cases := []struct{ reply, want string }{
		{"我们来看第 5 段，把这几句摆一摆。", "b5"},
		{"第二段说了封锁，现在我们看第五段。", "b5"},   // 一句话提到两段 → 取最后一个
		{"翻到第十段看看。", ""},                       // 只有六段 —— 它数错了，当没说
		{"我们继续往下读。", ""},                       // 没提段号
		{"回到第1段。", "b1"},
	}
	for _, c := range cases {
		if got := spokenParagraph(c.reply, blocks); got != c.want {
			t.Errorf("spokenParagraph(%q) = %q，想要 %q", c.reply, got, c.want)
		}
	}
}

func TestParseChineseOrdinal(t *testing.T) {
	for in, want := range map[string]int{
		"3": 3, "12": 12, "三": 3, "十": 10, "十二": 12, "二十": 20, "二十一": 21,
	} {
		if got := parseChineseOrdinal(in); got != want {
			t.Errorf("parseChineseOrdinal(%q) = %d，想要 %d", in, got, want)
		}
	}
}

func TestSplitSentences(t *testing.T) {
	got := splitSentences("他来了。她说：「真的吗？」然后走了。")
	if len(got) != 3 {
		t.Fatalf("切成了 %d 句：%q", len(got), got)
	}
	// 🚨 引号跟在句号后面时要一起带走：「……吗？」是一句，不是一句加一个引号。
	if !strings.HasSuffix(got[1], "」") {
		t.Errorf("收尾的引号没跟着走：%q", got[1])
	}
	// 🚨 英文缩写里的点不是句末。
	one := splitSentences("The U.S. sent aid to the region last week.")
	if len(one) != 1 {
		t.Errorf("U.S. 里的点被当成了句末：%q", one)
	}
}

// 🚨 印记 在话里点了名的那一句，必须在板上。
//
// 实测她卡住的那一幕：印记 说「把『以色列下令空袭』那句挪到对的格子里」，
// 而板上四张卡片全是第 1 段的话 —— 她逐字报的是「根本没有以色列空袭那句，
// 我找不到要挪的那张卡片」。
func TestBuiltBoardIncludesTheSentenceTheReplyNames(t *testing.T) {
	blocks := SplitBlocks(strings.Join([]string{
		"Aid groups are scrambling to help people caught in the war. They say the blockade has made every delivery slower.",
		"The agency said it had delivered 40 trucks of supplies last week. Officials cautioned that the figure could not be independently verified.",
		"以色列下令空袭加沙北部，要求当地居民立刻向南撤离。救援车队因此停在了半路上。",
	}, "\n\n"))
	// 落点段是第 1 段，但它嘴上点的是第 3 段那一句。
	reply := "我们把「以色列下令空袭加沙北部，要求当地居民立刻向南撤离」这句挪到对的格子里。"
	got := buildLabelBoardFromReply(blocks, "b1", reply)
	if got == nil {
		t.Fatal("没摆出板")
	}
	named := false
	for _, o := range got.Options {
		if strings.Contains(o.Quote, "以色列下令空袭") {
			named = true
		}
	}
	if !named {
		t.Fatalf("它点名的那一句不在板上：%+v", got.Options)
	}
	if got.Options[0].Quote == "" || !strings.Contains(got.Options[0].Quote, "以色列下令空袭") {
		t.Errorf("点名的那一句应该排在最前面，拿到 %q", got.Options[0].Quote)
	}
}

func TestReplyMentionsSentence(t *testing.T) {
	sent := "以色列下令空袭加沙北部，要求当地居民立刻向南撤离。"
	// 它引原文时常常截短、换标点。
	if !replyMentionsSentence("我们看「以色列下令空袭加沙北部」这一句。", sent) {
		t.Error("截短了的引用应该算数")
	}
	if replyMentionsSentence("我们继续看第三段讲了什么。", sent) {
		t.Error("没引到原文的话不该算数")
	}
}
