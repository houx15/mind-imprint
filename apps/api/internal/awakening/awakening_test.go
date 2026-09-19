package awakening

import (
	"strings"
	"testing"

	"mindimprint/api/internal/interest"
)

/* ── 「她等于什么都没说」的判定 ─────────────────────────────────────────── */

// 这一条是整个包里最容易写反的判据，所以它的用例最长。
//
// 判据是「去掉套话之后还剩不剩东西」，不是「包含其中一个」。按包含判，
// 下面 keep 那一组里的每一句都会被当成没说话 —— 而它们恰恰是这一轮里最有
// 价值的回答。
func TestTooThinKeepsRealAnswersThatContainFillerWords(t *testing.T) {
	keep := []string{
		"我不知道为什么，但每次看到猫从高处跳下来我都会停下来看完",
		"说不清是哪一段，大概是他明明可以走却留下来那里",
		"随便刷视频的时候刷到了潮汐发电，结果看了四十分钟",
		"就是喜欢那种把一堆乱七八糟的数据整理成一张表的过程",
	}
	for _, s := range keep {
		if TooThin(0, s) {
			t.Errorf("TooThin(%q) = true, 这是一句真的回答，不该被判成没说话", s)
		}
	}

	drop := []string{
		"", "   ", "不知道", "不知道。", "没想过", "随便",
		"都可以", "说不清", "不知道，说不清", "什么都喜欢",
	}
	for _, s := range drop {
		if !TooThin(0, s) {
			t.Errorf("TooThin(%q) = false, 这句话里没有可摘的原话", s)
		}
	}
}

func TestTooThinUsesTheNodesOwnMinimum(t *testing.T) {
	// NODE 06 的门槛比别的低（6 字），因为它的答案可以是「先补基础概念」这么短；
	// 而第一问要一个具体例子，同样一句话在那里就该再问一次。
	short := "先补基础概念"
	if TooThin(5, short) {
		t.Errorf("TooThin(NODE06, %q) = true，这一节点允许更短的回答", short)
	}
	if !TooThin(0, short) {
		t.Errorf("TooThin(NODE01, %q) = false，第一问要一个具体例子", short)
	}
}

/* ── 幻引 ───────────────────────────────────────────────────────────────── */

func TestHallucinatedQuotesFindsWordsSheNeverWrote(t *testing.T) {
	corpus := "我每次看到潮汐发电的动画都会停下来看完，因为它把看不见的力变成了看得见的东西"

	// 引的是她真写过的话 —— 干净。
	clean := "你提到「停下来看完」，看起来吸引你的是那个瞬间。你还记得第一次是在哪里看到的吗？"
	if bad := HallucinatedQuotes(clean, corpus); len(bad) != 0 {
		t.Errorf("HallucinatedQuotes 把她真写过的话判成了幻引：%v", bad)
	}

	// 说成是她说的、而她没说过 —— 抓住。
	dirty := "你说「我从小就想当工程师」，这条线很清楚。"
	bad := HallucinatedQuotes(dirty, corpus)
	if len(bad) != 1 || bad[0] != "我从小就想当工程师" {
		t.Errorf("HallucinatedQuotes = %v, want [我从小就想当工程师]", bad)
	}
}

// 🚨 这一条来自 2026-09-19 上线前那次 LIVE_LLM：第一版判**所有**成对引号，
// 当场把一条很好的回复判死了 —— 模型给自己提出的一个模式起了个名字，
// 中文里引号本来就有这个用法，而它一个字都没说这是她说的。
//
// 真正的失败是「把她没说过的话说成她说的」，不是「用了引号」。
func TestHallucinatedQuotesIgnoresTheModelsOwnCoinedTerms(t *testing.T) {
	corpus := "最吸引我的是那个闸门的节奏，它不是一直转，而是要等潮水到某个高度才动一次"
	// 真模型回的原话（节选）。「闸门的节奏」是她的，被归给了她，查得到；
	// 「等待然后释放」是模型自己起的名字，没有归给她，不该判。
	reply := "你说的是「闸门的节奏」——让你在意的可能是那个「等待然后释放」的模式。"
	if bad := HallucinatedQuotes(reply, corpus); len(bad) != 0 {
		t.Errorf("模型给自己的说法起名字不该判成幻引：%v", bad)
	}
}

// 没有归属说法的时候，一个她没写过的引文也不判 —— 那是模型在强调，不是在引用。
func TestHallucinatedQuotesOnlyJudgesAttributedQuotes(t *testing.T) {
	corpus := "我喜欢拆开看里面"
	if bad := HallucinatedQuotes("这里有一个「反直觉的地方」值得再看一眼。", corpus); len(bad) != 0 {
		t.Errorf("没有归属的引号不该判：%v", bad)
	}
	if bad := HallucinatedQuotes("你提到「反直觉的地方」，我们再看一眼。", corpus); len(bad) != 1 {
		t.Errorf("归给她、而她没写过的话必须判出来，得到 %v", bad)
	}
}

/* ── 回复的清理 ─────────────────────────────────────────────────────────── */

func TestCleanReplyStripsPrefixesAndWrappingQuotes(t *testing.T) {
	cases := map[string]string{
		"印记助手：你提到潮汐发电。":     "你提到潮汐发电。",
		"「你提到潮汐发电。」":         "你提到潮汐发电。",
		`"你提到潮汐发电。"`:         "你提到潮汐发电。",
		"  你提到潮汐发电。  ":       "你提到潮汐发电。",
	}
	for in, want := range cases {
		if got := CleanReply(in); got != want {
			t.Errorf("CleanReply(%q) = %q, want %q", in, got, want)
		}
	}
}

// 首尾不成对时不能削 —— 否则一句正常的「她说『……』」会被削掉半边。
func TestCleanReplyLeavesUnpairedQuotesAlone(t *testing.T) {
	in := "「停下来看完」是你自己的说法，我们从这里往下问。"
	if got := CleanReply(in); got != in {
		t.Errorf("CleanReply 削掉了一个不成对的引号：%q", got)
	}
}

func TestCleanReplyTruncatesModelText(t *testing.T) {
	long := strings.Repeat("字", dialogueMaxRunes+50)
	if n := runeLen(CleanReply(long)); n != dialogueMaxRunes {
		t.Errorf("CleanReply 长度 = %d, want %d", n, dialogueMaxRunes)
	}
}

/* ── 树简报 ─────────────────────────────────────────────────────────────── */

func TestBuildBriefOnAnEmptyTree(t *testing.T) {
	b := BuildBrief(nil, 1, 0)
	if !b.IsFirstTime() {
		t.Error("空树应当算第一次")
	}
	if len(b.EmptyFields) != len(allFields) {
		t.Errorf("空树上空着的枝 = %d, want %d", len(b.EmptyFields), len(allFields))
	}
	if b.Text() != "" {
		t.Error("空树的简报应当是空串，不该告诉模型一件它不需要知道的事")
	}
	if got := b.OpeningAsk(); got != Nodes[0].Ask {
		t.Errorf("第一次的开场问题应当是 NODE 01 的默认问法，得到 %q", got)
	}
}

// 第二趟的开场必须从她已经有的词出发。这就是旧测试缺的那条闭环：
// 树往回影响协议。
func TestBuildBriefOnAGrownTreeOpensFromHerOwnWord(t *testing.T) {
	known := []Known{
		{InterestID: "game-design", Zh: "游戏", Field: "making", Strength: 4},
		{InterestID: "psychology-dev", Zh: "心理", Field: "self", Strength: 2},
	}
	b := BuildBrief(known, 2, 47)

	if b.IsFirstTime() {
		t.Error("有词的树不是第一次")
	}
	ask := b.OpeningAsk()
	if !strings.Contains(ask, "游戏") {
		t.Errorf("第二趟的开场应当提到她最强的那个词，得到 %q", ask)
	}
	if ask == Nodes[0].Ask {
		t.Error("第二趟不该再问一遍「你最近喜欢什么」")
	}

	txt := b.Text()
	for _, want := range []string{"游戏", "心理", "47 天"} {
		if !strings.Contains(txt, want) {
			t.Errorf("简报里少了 %q：\n%s", want, txt)
		}
	}
	// 她有词的两根枝不该出现在「还空着」那一行里。
	if strings.Contains(txt, "技术与创造、") || strings.Contains(txt, "、自我与成长") {
		t.Errorf("已经有词的枝被列成了空枝：\n%s", txt)
	}
}

// 强度降序，同强度按中文名 —— 一个确定的顺序，两次调用才可比较。
func TestBuildBriefOrdersByStrengthThenName(t *testing.T) {
	// 同强度按中文名的**码位**排（海 U+6D77 < 游 U+6E38），不是按笔画或拼音。
	b := BuildBrief([]Known{
		{InterestID: "b", Zh: "游戏", Field: "arts", Strength: 2},
		{InterestID: "a", Zh: "海洋", Field: "arts", Strength: 2},
		{InterestID: "c", Zh: "密码", Field: "arts", Strength: 5},
	}, 1, 0)
	got := []string{b.Top[0].Zh, b.Top[1].Zh, b.Top[2].Zh}
	want := []string{"密码", "海洋", "游戏"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Top 顺序 = %v, want %v", got, want)
		}
	}
}

func TestBuildBriefCapsTheList(t *testing.T) {
	var known []Known
	for i := 0; i < briefTopN+7; i++ {
		known = append(known, Known{InterestID: "x", Zh: "词", Field: "arts", Strength: 1})
	}
	b := BuildBrief(known, 1, 0)
	if len(b.Top) != briefTopN {
		t.Errorf("Top 长度 = %d, want %d", len(b.Top), briefTopN)
	}
	if !strings.Contains(b.Text(), "另外还有 7 个") {
		t.Errorf("简报应当说清楚被截掉了几个：\n%s", b.Text())
	}
}

// 做过一次、但一个词都没长出来的学生，第二次仍然要从头问起。
// 判据是树上有没有词，不是 attempt_no。
func TestBriefTreatsAWordlessSecondAttemptAsFirstTime(t *testing.T) {
	b := BuildBrief(nil, 3, 12)
	if !b.IsFirstTime() {
		t.Error("attempt_no 是 3 但树上一个词都没有，开场仍然该从头问")
	}
	if got := b.OpeningAsk(); got != Nodes[0].Ask {
		t.Errorf("开场问题 = %q, want NODE 01 的默认问法", got)
	}
}

/* ── 三种写回的判定 ─────────────────────────────────────────────────────── */

func TestClassifySplitsConfirmFromGrow(t *testing.T) {
	// 用真实存在的 id，否则 Classify 会按闭表把它们丢掉。
	hs := []interest.Harvested{
		{InterestID: "cryptography", Note: "n", Evidence: "e"},
		{InterestID: "breeding", Note: "n", Evidence: "e"},
	}
	got := Classify(hs, []string{"cryptography"})
	if len(got) != 2 {
		t.Fatalf("Classify 返回 %d 条, want 2", len(got))
	}
	if got[0].Verdict != VerdictConfirm {
		t.Errorf("树上已有的词应当是 confirm，得到 %q", got[0].Verdict)
	}
	if got[1].Verdict != VerdictGrow {
		t.Errorf("新词应当是 grow，得到 %q", got[1].Verdict)
	}
	// 中文名从词表查，不从模型的回话里读。
	if got[0].Zh == "" || got[0].Field == "" {
		t.Error("Classify 应当把词表里的中文名和主枝填上")
	}
}

func TestClassifyDropsIDsOutsideTheCatalog(t *testing.T) {
	hs := []interest.Harvested{{InterestID: "esports-but-invented", Evidence: "e"}}
	if got := Classify(hs, nil); len(got) != 0 {
		t.Errorf("闭表外的 id 应当被丢掉，得到 %v", got)
	}
}

// 这次在一根空枝上长出了词，那根枝就不再空着。
func TestStillOpenRemovesBranchesThisRunFilled(t *testing.T) {
	wasEmpty := []string{"formal", "arts", "self"}
	planted := []Planted{
		{Field: "arts", Verdict: VerdictGrow},
		{Field: "self", Verdict: VerdictConfirm}, // confirm 说明本来就有词
	}
	got := StillOpen(wasEmpty, planted)
	want := []string{"formal", "self"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("StillOpen = %v, want %v", got, want)
	}
}

func TestDisciplineIDsOrdersByHowOftenTheyAppear(t *testing.T) {
	// cryptography → logic-proof, discrete-networks, algorithms
	// breeding     → genetics, ecology, economics
	ids := DisciplineIDs([]Planted{
		{InterestID: "cryptography"},
		{InterestID: "cryptography"},
		{InterestID: "breeding"},
	})
	if len(ids) == 0 {
		t.Fatal("DisciplineIDs 一个都没给")
	}
	// 出现两次的那三个必须排在只出现一次的前面。
	first := ids[0]
	if first != "algorithms" && first != "discrete-networks" && first != "logic-proof" {
		t.Errorf("排在最前的应当是出现两次的学科，得到 %q（完整：%v）", first, ids)
	}
}

/* ── 报告 ───────────────────────────────────────────────────────────────── */

func TestParseReportReplyDropsThinEvidenceAndClampsConfidence(t *testing.T) {
	raw := "```json\n" + `{"drivers":[
      {"label":"进度看得见","evidence":"每次看到进度条往前走我就想再做一局","confidence":1.7},
      {"label":"敷衍","evidence":"有","confidence":0.5},
      {"label":"掌控感","evidence":"我喜欢自己决定先做哪一步","confidence":-3}
    ],"summary":"你这次谈的是把看不见的过程变成看得见的东西。"}` + "\n```"

	ds, summary, err := ParseReportReply(raw)
	if err != nil {
		t.Fatalf("ParseReportReply 报错：%v", err)
	}
	if len(ds) != 2 {
		t.Fatalf("保留了 %d 条, want 2（「有」太短该丢）", len(ds))
	}
	if ds[0].Confidence != 1 {
		t.Errorf("confidence 应当夹到 1，得到 %v", ds[0].Confidence)
	}
	if ds[1].Confidence != 0 {
		t.Errorf("confidence 应当夹到 0，得到 %v", ds[1].Confidence)
	}
	if summary == "" {
		t.Error("summary 丢了")
	}
}

func TestParseReportReplyErrorsRatherThanInventing(t *testing.T) {
	if _, _, err := ParseReportReply("模型今天不想说话"); err == nil {
		t.Error("解析不出 JSON 时必须报错，绝不返回一条像样的假设")
	}
}

// 模型把 prompt 里的脚手架文字当成她的原话返回过一次。这条守着那件事。
func TestKeepGroundedDriversDropsWordsSheNeverWrote(t *testing.T) {
	corpus := "我每次看到进度条往前走就想再做一局"
	ds := []Driver{
		{Label: "进度看得见", Evidence: "每次看到进度条往前走"},
		{Label: "编的", Evidence: "她喜欢的是《进击的巨人》里的利威尔"},
	}
	got := KeepGroundedDrivers(ds, corpus)
	if len(got) != 1 || got[0].Label != "进度看得见" {
		t.Errorf("KeepGroundedDrivers = %v, 只该留下有出处的那一条", got)
	}
}

// 她的问题和作品设想来自固定的两问，原样取出，不截断。
func TestHerOwnSentencesComeOutVerbatim(t *testing.T) {
	answers := make([]string, NodeCount)
	long := strings.Repeat("为什么潮汐发电在有些海岸能用在另一些不能", 40)
	answers[nodeQuestionIndex] = long
	answers[nodeWorkIndex] = "我想做一个给同学看的图解"

	if got := HerQuestion(answers); got != long {
		t.Errorf("她的问题被改过了：长度 %d, want %d", runeLen(got), runeLen(long))
	}
	if got := HerWorkConcept(answers); got != "我想做一个给同学看的图解" {
		t.Errorf("她的作品设想 = %q", got)
	}
}

func TestBuildDiffIsAbsentOnTheFirstRun(t *testing.T) {
	if BuildDiff(nil, nil, 0) != nil {
		t.Error("第一趟不该有「和上次比」这一块")
	}
}

func TestBuildDiffSeparatesStrongerFromNew(t *testing.T) {
	prev := &Report{Question: "为什么有些海岸适合潮汐发电？"}
	d := BuildDiff([]Planted{
		{Zh: "游戏", Verdict: VerdictConfirm},
		{Zh: "海洋", Verdict: VerdictGrow},
	}, prev, 47)
	if len(d.Stronger) != 1 || d.Stronger[0] != "游戏" {
		t.Errorf("Stronger = %v", d.Stronger)
	}
	if len(d.New) != 1 || d.New[0] != "海洋" {
		t.Errorf("New = %v", d.New)
	}
	if d.PreviousQuestion != prev.Question || d.DaysBetween != 47 {
		t.Errorf("Diff 没带上上一趟的问题或天数：%+v", d)
	}
}

/* ── prompt 的形状 ──────────────────────────────────────────────────────── */

// 选词 prompt 里必须有闭表，否则模型只能自己造 id。
func TestSelectionPromptCarriesTheClosedCatalog(t *testing.T) {
	system, user := BuildSelectionPrompt([]string{"我喜欢潮汐发电"}, "为什么？")
	if !strings.Contains(system, "cryptography") {
		t.Error("选词 prompt 里没有闭表，模型只能自己造 id")
	}
	if !strings.Contains(system, "原样摘录") {
		t.Error("选词 prompt 必须要求原样摘录，否则转述过不了逐字比对")
	}
	if !strings.Contains(user, "我喜欢潮汐发电") {
		t.Error("她的原话没有进 user 段")
	}
}

// 🚨 这一条守着 2026-09-11 那次踩坑的反面：不要把采集那条「材料的话题不算」
// 带进来。这里没有材料，带进来的结果是真模型 0/3 长出词。
func TestSelectionRulesDoNotBorrowTheMaterialRule(t *testing.T) {
	if strings.Contains(selectionRules, "不是这篇材料的话题") {
		t.Error("选词判据借了采集那条反的规则；这里没有材料")
	}
}

// 对话 prompt 不许要求 JSON：流式会丢最后一个分片，而丢了结尾的 JSON 整份作废。
func TestDialoguePromptAsksForPlainText(t *testing.T) {
	system, _ := BuildDialoguePrompt(DialogueInput{
		Guide: Guides[0], NodeIndex: 0, Latest: "我喜欢潮汐发电",
	})
	if strings.Contains(system, "JSON") || strings.Contains(system, "json") {
		t.Error("对话那一次不该要 JSON —— 流式丢一个分片就整份作废")
	}
	if !strings.Contains(system, Nodes[0].Ask) {
		t.Error("当前节点要问的问题必须原样进 prompt")
	}
}

func TestDialoguePromptUsesTheRetryAskWhenSheAnsweredThin(t *testing.T) {
	system, _ := BuildDialoguePrompt(DialogueInput{
		Guide: Guides[0], NodeIndex: 0, Retry: true, Latest: "不知道",
	})
	if !strings.Contains(system, Nodes[0].Retry) {
		t.Error("答得太薄时应当换成 Retry 的问法")
	}
	if strings.Contains(system, "这一步要问的问题："+Nodes[0].Ask) {
		t.Error("换问法时不该还把原问法当成要问的那一句")
	}
}

// 上文按预算从最近往前取，最近那一轮一定在里面。
func TestDialogueUserKeepsTheMostRecentTurns(t *testing.T) {
	var history []Turn
	for i := 0; i < 40; i++ {
		history = append(history, Turn{
			StudentText: strings.Repeat("旧", 100),
			Reply:       strings.Repeat("回", 100),
		})
	}
	history = append(history, Turn{StudentText: "最近这一轮我说的话", Reply: "印记的回应"})

	_, user := BuildDialoguePrompt(DialogueInput{
		Guide: Guides[0], NodeIndex: 3, History: history, Latest: "她刚说的",
	})
	if !strings.Contains(user, "最近这一轮我说的话") {
		t.Error("最近的一轮没有进上文")
	}
	if !strings.Contains(user, "她刚说的") {
		t.Error("她刚敲进去的那句没有进 user 段")
	}
	if runeLen(user) > historyRuneBudget+600 {
		t.Errorf("上文超预算：%d 字", runeLen(user))
	}
}

/* ── 常量之间的一致性 ───────────────────────────────────────────────────── */

func TestEveryGuideHasAnAudioPrefixAndStyle(t *testing.T) {
	seen := map[string]bool{}
	for _, g := range Guides {
		if g.AudioPrefix == "" || g.Style == "" || g.Zh == "" {
			t.Errorf("印记助手 %s 少了字段：%+v", g.ID, g)
		}
		if seen[g.AudioPrefix] {
			t.Errorf("两个助手共用了同一个音频前缀 %q", g.AudioPrefix)
		}
		seen[g.AudioPrefix] = true
	}
}

func TestStagesAreUniqueAndRecognised(t *testing.T) {
	seen := map[Stage]bool{}
	for _, s := range Stages {
		if seen[s] {
			t.Errorf("重复的屏：%s", s)
		}
		seen[s] = true
		if !IsStage(string(s)) {
			t.Errorf("IsStage(%q) = false", s)
		}
	}
	if IsStage("") || IsStage("nope") {
		t.Error("IsStage 放行了一个不存在的屏")
	}
}

func TestEveryNodeHasBothAsksAndAnObjective(t *testing.T) {
	for i, n := range Nodes {
		if n.Ask == "" || n.Retry == "" || n.Objective == "" || n.Code == "" {
			t.Errorf("NODE %d 少了字段：%+v", i+1, n)
		}
		if n.Ask == n.Retry {
			t.Errorf("NODE %d 的换问法和原问法一样，那就不是换一个入口", i+1)
		}
		if n.MinRunes <= 0 {
			t.Errorf("NODE %d 的 MinRunes = %d", i+1, n.MinRunes)
		}
	}
}
