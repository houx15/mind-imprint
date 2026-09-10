package api

import "testing"

// 两块新板（label_roles / word_bank）和那张被禁问法的表。
//
// 全部走的是同一条思路：**保证靠核对，不靠嘱咐**。板上的每一句、每一个词都得
// 是文章里真有的东西 —— 模型顺手写出来的一个词，在板上和一个真词长得一模一样，
// 而她会把它背下来。

func boardBlocks() []Block {
	return []Block{
		{ID: "b1", Text: "Aid groups are scrambling to help people caught in the war."},
		{ID: "b2", Text: "The agency said it had delivered 40 trucks of supplies last week."},
		{ID: "b3", Text: "Officials cautioned that the figure could not be independently verified."},
	}
}

func TestLabelRolesGetsItsLabelsFromTheServer(t *testing.T) {
	// 🚨 格子是闭表，而且由服务端填。模型编出来的第六个角色在板上没有格子，
	// 在带读规矩里也没有对应的说法。
	got := validateCoachCard(&coachCard{
		Type:   coachCardLabelRoles,
		Prompt: "这几句在文章里各自在干什么？",
		Options: []coachCardOption{
			{BlockID: "b2", Quote: "The agency said it had delivered 40 trucks of supplies last week."},
			{BlockID: "b3", Quote: "Officials cautioned that the figure could not be independently verified."},
		},
		Labels: []string{"作者自己编的一个角色"},
	}, boardBlocks())
	if got == nil {
		t.Fatal("这张标注板应该通过")
	}
	if len(got.Labels) != len(coachCardRoleLabels) || got.Labels[0] != coachCardRoleLabels[0] {
		t.Fatalf("格子应该被服务端那份覆盖，拿到 %v", got.Labels)
	}
}

func TestLabelRolesMayTakeBothSentencesFromOneParagraph(t *testing.T) {
	// 🚨 这条测试原来断言的是**相反**的事，那是我照抄 choose_span 的规则时犯的
	// 设计错误。跨段落那一条的理由是「选项全在一段里 = 选项就是那一段，扫一眼
	// 名词就能点」—— 而那条推理在标注板上不成立：角色（主张/证据/限制/背景/
	// 对比）是扫名词扫不出来的。
	//
	// 线上第一次跑就撞上了：印记 连着两轮想给她一块板（「哪一句是马上会发生的
	// 后果，哪一句是原因？」），两次都被丢掉，她只看到 印记 在描述一块从来没
	// 出现过的板。同一段里的「后果」和「原因」恰恰是最值得让她分辨的一对。
	got := validateCoachCard(&coachCard{
		Type:   coachCardLabelRoles,
		Prompt: "哪一句是后果，哪一句是原因？",
		Options: []coachCardOption{
			{BlockID: "b4", Quote: "Supplies ran out within days."},
			{BlockID: "b4", Quote: "The border crossing stayed shut."},
		},
	}, sameParagraphBlocks())
	if got == nil {
		t.Fatal("同一段里的两句话应该能摆成一块标注板")
	}
	if len(got.Options) != 2 {
		t.Fatalf("两句都该留下，拿到 %v", got.Options)
	}
}

// choose_span 那一侧的规则**没有**跟着放松。
func TestChooseSpanStillNeedsTwoParagraphs(t *testing.T) {
	got := validateCoachCard(&coachCard{
		Type:   coachCardChooseSpan,
		Prompt: "哪一句让你最清楚地看到封锁的后果？",
		Options: []coachCardOption{
			{BlockID: "b4", Quote: "Supplies ran out within days."},
			{BlockID: "b4", Quote: "The border crossing stayed shut."},
		},
	}, sameParagraphBlocks())
	if got != nil {
		t.Fatal("choose_span 的选项全在同一段，仍然应该整张丢掉")
	}
}

// 一段里有两句完整的句子。
func sameParagraphBlocks() []Block {
	return []Block{
		{ID: "b4", Text: "Supplies ran out within days. The border crossing stayed shut."},
	}
}

func TestWordBankTermsMustBeWholeWordsInTheirBlock(t *testing.T) {
	got := validateCoachCard(&coachCard{
		Type:   coachCardWordBank,
		Prompt: "这几个词，哪些你已经认识？",
		Words: []coachCardWord{
			{BlockID: "b1", Term: "scrambling"},
			{BlockID: "b2", Term: "delivered"},
			{BlockID: "b3", Term: "cautioned"},
			// 🚨 `veri` 是 `verified` 的子串，但它不是这一段里出现过的**词**。
			// 子串核对会放它过去，词边界核对不会。
			{BlockID: "b3", Term: "veri"},
			// 这个词根本不在第一段里。
			{BlockID: "b1", Term: "negotiated"},
			// 挂错段落：它在第三段，不在第二段。
			{BlockID: "b2", Term: "cautioned"},
		},
	}, boardBlocks())
	if got == nil {
		t.Fatal("三个真词够一块板了")
	}
	if len(got.Words) != 3 {
		t.Fatalf("只有三个词站得住，拿到 %d：%v", len(got.Words), got.Words)
	}
}

func TestWordBankNeedsThreeSurvivors(t *testing.T) {
	got := validateCoachCard(&coachCard{
		Type:   coachCardWordBank,
		Prompt: "这几个词你认识吗？",
		Words: []coachCardWord{
			{BlockID: "b1", Term: "scrambling"},
			{BlockID: "b1", Term: "notinthearticle"},
		},
	}, boardBlocks())
	if got != nil {
		t.Fatal("只剩一个词不成一块板，应该整张丢掉")
	}
}

func TestWordBankIsCaseInsensitiveButDedupes(t *testing.T) {
	got := validateCoachCard(&coachCard{
		Type:   coachCardWordBank,
		Prompt: "这几个词你认识吗？",
		Words: []coachCardWord{
			// 句首那个词首字母大写，而板上摆的是词本身。
			{BlockID: "b1", Term: "aid"},
			{BlockID: "b1", Term: "Aid"},
			{BlockID: "b2", Term: "supplies"},
			{BlockID: "b3", Term: "verified"},
		},
	}, boardBlocks())
	if got == nil {
		t.Fatal("这块板应该通过")
	}
	if len(got.Words) != 3 {
		t.Fatalf("aid / Aid 是同一个词，应该只留一个，拿到 %v", got.Words)
	}
}

func TestBannedQuestionFormsDropTheWholeCard(t *testing.T) {
	// 来自 ljg-qa 的 QuestionDesign。它顺带点出了模型的默认毛病：
	// 「AI 默认会写「什么是 X」型问题 —— 教科书腔」。
	banned := []string{
		"什么是援助走廊？",
		"援助走廊是什么",
		"这个过程有几个步骤？",
		"这件事重要吗？",
		"我们应当如何看待这次停火",
		"这个方案的优缺点是什么？",
		"这段讲了什么？",
	}
	for _, p := range banned {
		if !rejectBannedQuestion(p) {
			t.Errorf("没拦住：%q", p)
		}
		got := validateCoachCard(&coachCard{
			Type:   coachCardChooseSpan,
			Prompt: p,
			Options: []coachCardOption{
				{BlockID: "b2", Quote: "The agency said it had delivered 40 trucks of supplies last week."},
				{BlockID: "b3", Quote: "Officials cautioned that the figure could not be independently verified."},
			},
		}, boardBlocks())
		if got != nil {
			t.Errorf("整张卡应该被丢掉：%q", p)
		}
	}
}

func TestGoodQuestionsSurvive(t *testing.T) {
	// 一张网如果把好问题也拦下来，它比没有网更糟 —— 她会连着几轮什么卡片都
	// 收不到，而屏幕上不会有任何东西说明为什么。
	ok := []string{
		"作者是怎么让你相信这个数字的？",
		"为什么是第三段这句话，不是第二段那句？",
		"哪一句你读着最不服气？",
		"这两句之间发生了什么变化？",
		"它在什么时候就不成立了？",
		"这份数据重要在哪里？", // 「重要」在句中，不是「…重要吗」那种问法
	}
	for _, p := range ok {
		if rejectBannedQuestion(p) {
			t.Errorf("误伤了一个好问题：%q", p)
		}
	}
}
