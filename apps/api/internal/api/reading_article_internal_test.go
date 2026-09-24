package api

import (
	"strings"
	"testing"
)

// 整篇那一层。产品负责人 2026-09-24：「for poems or 记叙文, we also need a
// whole-level view of analysis (actually, all need that, the article
// structure, etc.」

// 🚨 每一块的引文都要回全文里逐字核对。这是这件工具**唯一**可验的东西：
// 「这篇文章分成四块」这种话本身一个字都验不了，而模型在整篇这一层最爱凭印象
// 说话。核不上的整块丢掉，一块都不剩就算整件工具失败。
func TestArticleOutlineDropsPartsNotInTheArticle(t *testing.T) {
	article := "海水里的盐一部分来自陆地。\n\n另一部分来自海底。\n\n水蒸发，盐留下来。"
	body := `{"parts":[
		{"label":"说明对象","quote":"海水里的盐","note":"点出全文要讲的那件事"},
		{"label":"原因","quote":"另一部分来自海底","note":"第二个来源"},
		{"label":"结论","quote":"这句话原文里根本没有","note":"编的"}
	],"spine":"并列","takeaway":"海水为什么是咸的"}`
	got, ok := parseArticleOutline(body, article)
	if !ok {
		t.Fatal("一块都没活下来")
	}
	if len(got.Parts) != 2 {
		t.Fatalf("留下了 %d 块，要的是 2 块（第三块的引文不在原文里，该丢）", len(got.Parts))
	}
	for _, p := range got.Parts {
		if !strings.Contains(article, p.Quote) {
			t.Errorf("引文 %q 不在原文里却留下来了 —— 她点它跳不到任何地方", p.Quote)
		}
	}
	if got.Spine != "并列" || got.Takeaway != "海水为什么是咸的" {
		t.Errorf("spine / takeaway 被改动了：%q / %q", got.Spine, got.Takeaway)
	}
}

// 一块都核不上 = 整件工具失败，绝不返回一张空图。
// 一张没有任何一块的「结构图」比不给更糟：她会以为这篇文章没有结构。
func TestArticleOutlineFailsWhenNothingSurvives(t *testing.T) {
	body := `{"parts":[{"label":"结论","quote":"完全编出来的一句","note":""}],"spine":"总分"}`
	if _, ok := parseArticleOutline(body, "真正的原文在这里。"); ok {
		t.Error("一块都核不上却算成功了")
	}
	if _, ok := parseArticleOutline("这不是 JSON", "原文"); ok {
		t.Error("不是 JSON 却算成功了")
	}
}

// 同一句引文重复只留第一块 —— 两块指着同一处的图读起来像有两块，其实只有一块。
func TestArticleOutlineDropsDuplicateQuotes(t *testing.T) {
	article := "第一句在这里。第二句在这里。"
	body := `{"parts":[
		{"label":"起","quote":"第一句在这里","note":"a"},
		{"label":"承","quote":"第一句在这里","note":"b"}
	]}`
	got, ok := parseArticleOutline(body, article)
	if !ok {
		t.Fatal("一块都没活下来")
	}
	if len(got.Parts) != 1 {
		t.Errorf("同一句引文留下了 %d 块", len(got.Parts))
	}
}

// 🚨 一种体裁一件，而且**体裁认不出来时一件都不给**。
//
// 整篇这一层的每一件工具都带一套这一体裁专属的分类词（论点/分论点 对
// 起因/经过/转折 对 起/承/转/合），拿错一套比不给更糟。
func TestArticleToolsAreOnePerGenre(t *testing.T) {
	want := map[string]string{
		genreArgument:  "argument_map",
		genreExplain:   "explain_map",
		genreReport:    "report_map",
		genreNarrative: "narrative_map",
		genreProse:     "prose_map",
		genrePoem:      "poem_shape",
		genreClassical: "classical_shape",
	}
	for genre, toolID := range want {
		got := readingArticleToolsFor("zh", genre)
		if len(got) != 1 {
			t.Fatalf("体裁 %q 拿到 %d 件整篇工具，要的是 1 件", genre, len(got))
		}
		if got[0].ID != toolID {
			t.Errorf("体裁 %q 拿到的是 %q，要的是 %q", genre, got[0].ID, toolID)
		}
		if got[0].Scope != "article" {
			t.Errorf("%q 的 scope 不是 article —— 它会跑进段落工具条", got[0].ID)
		}
		if got[0].Shape != "article" {
			t.Errorf("%q 的 shape 不是 article —— 它的产物不会被当成一张图解析", got[0].ID)
		}
	}
	// 🚨 散文和记叙文**不许共用同一件**。产品负责人给的那份 how-to 把轴分开了：
	// 「记叙文侧重事件和人物，散文侧重线索、景物或物象和情感」。
	// 共用一套 label 闭表（起因/经过/转折/结果）等于逼一篇《荷塘月色》交出
	// 一件事的来龙去脉，而它根本没有一件贯穿的事。
	if readingArticleToolsFor("zh", genreProse)[0].ID ==
		readingArticleToolsFor("zh", genreNarrative)[0].ID {
		t.Error("散文和记叙文共用了同一件整篇工具 —— 它们的轴不一样")
	}
	if got := readingArticleToolsFor("zh", ""); len(got) != 0 {
		t.Errorf("体裁认不出来时给了 %d 件整篇工具 —— 分类词会拿错一套", len(got))
	}
}

// 🚨 整篇那几件不许出现在划选工具条或段落工具条上。
//
// 判据放在服务端这一侧：它们**没有 subject**，所以只靠 subject 过滤的前端
// 会把它们当成整段工具摆进段落工具条。scope 这一位就是为此存在的，
// 而这条测试守的是「服务端真的发了它」。
func TestArticleToolsCarryScopeAndNoSubject(t *testing.T) {
	for _, tool := range readingArticleTools {
		if tool.Scope != "article" {
			t.Errorf("%s 少了 scope=article", tool.ID)
		}
		if tool.Subject != "" {
			t.Errorf("%s 带了 subject=%q —— 整篇那一层没有「哪一句」", tool.ID, tool.Subject)
		}
	}
}

// 整篇工具用的是哨兵 block_id，它必须是 SplitBlocks 永远产不出的那种。
// 撞上一个真段落，整篇那份图会盖掉那一段的讲解。
func TestArticleBlockIDCannotCollideWithARealBlock(t *testing.T) {
	blocks := SplitBlocks("第一段。\n\n第二段。\n\n第三段。")
	if len(blocks) == 0 {
		t.Fatal("切不出段落")
	}
	for _, b := range blocks {
		if b.ID == readingArticleBlockID {
			t.Fatalf("哨兵 %q 和真段落撞上了", readingArticleBlockID)
		}
	}
}

// 提示词里要带**段号和字数**。
//
// 字数不是装饰：「详略安排」这一维就是靠比各段长短得出来的（来源库把它列为
// 初二的重点考察项），而模型自己数不准。服务端数得出来的事实就别让模型数。
func TestArticlePromptCarriesParagraphLengths(t *testing.T) {
	blocks := SplitBlocks("短的一段。\n\n" + strings.Repeat("很长的一段话。", 20))
	p := buildReadingArticlePrompt("北京的春节", blocks)
	for _, want := range []string{"北京的春节", "第1段", "第2段", "字）"} {
		if !strings.Contains(p, want) {
			t.Errorf("整篇 prompt 里没有 %q：\n%s", want, p)
		}
	}
	// 长段的字数必须真的比短段大 —— 数错了，详略那一维就是反的。
	if !strings.Contains(p, "（140字）") {
		t.Errorf("长段的字数不对，prompt 是：\n%s", p)
	}
}

// 🚨 诗词上撤掉了「结构解析」和「案例」。
//
// 产品负责人走查《江雪》时两件都在工具条上，而它们问的是「把这一段按句子拆成
// 2 到 5 个层次」「这一段举了哪些具体的事例、数据或引用」。
// 十个字里没有层次可拆，也没有数据可举。
func TestPoemsDoNotGetParagraphStructureOrExamples(t *testing.T) {
	ids := map[string]bool{}
	for _, tool := range readingBlockToolsFor("zh", genrePoem) {
		ids[tool.ID] = true
	}
	for _, gone := range []string{"structure", "examples"} {
		if ids[gone] {
			t.Errorf("诗词的工具条上还有 %q", gone)
		}
	}
	// 诗自己那两件还在。
	for _, want := range []string{"poem_images", "classical_translate", "classical_word"} {
		if !ids[want] {
			t.Errorf("诗词的工具条上少了 %q", want)
		}
	}
}

// 🚨 反方向：NotGenres 不许把别的体裁上的通用工具也撤掉，**体裁认不出来时
// 更不许撤**。撤掉一件本来每篇都有的通用工具，等于把老数据上已有的东西拿走。
func TestNotGenresOnlyRemovesFromTheNamedGenre(t *testing.T) {
	for _, genre := range []string{genreArgument, genreReport, genreNarrative, genreExplain, genreClassical, ""} {
		ids := map[string]bool{}
		for _, tool := range readingBlockToolsFor("zh", genre) {
			ids[tool.ID] = true
		}
		for _, want := range []string{"structure", "examples", "rhetoric"} {
			if !ids[want] {
				t.Errorf("体裁 %q 上少了通用工具 %q", genre, want)
			}
		}
	}
}

// 中文的字词卡不共用英文那份契约。产品负责人 2026-09-24：
// 「I think we need to make chinese lookup different from english.」
//
// 差别是可验的：英文那份要一句**新造的英文例句**，中文那份要**类别**和
// **凭什么这么判**。
func TestChineseWordCardContractDiffersFromEnglish(t *testing.T) {
	zh := readingShapedSuffix["hanwords"]
	en := readingShapedSuffix["words"]
	if zh == "" {
		t.Fatal("中文字词卡没有自己的输出约定")
	}
	if zh == en {
		t.Fatal("中文字词卡还在用英文那份约定")
	}
	if strings.Contains(zh, "英文例句") {
		t.Error("中文那份约定里还要一句英文例句 —— 给一个文言字造英文例句没有意义")
	}
	for _, want := range []string{"通假字", "古今异义", "词类活用", "凭什么这么判"} {
		if !strings.Contains(zh, want) {
			t.Errorf("中文字词卡的约定里没有 %q", want)
		}
	}
	// 英文那份一个字都没动。
	if !strings.Contains(en, "新造的**英文例句") {
		t.Error("英文那份约定被改动了")
	}
}

// 查字这件工具要真的走中文那份契约，并且是词一级的。
func TestClassicalWordToolIsWordLevelAndChinese(t *testing.T) {
	tool, ok := findReadingBlockTool("classical_word")
	if !ok {
		t.Fatal("找不到查字这件工具")
	}
	if tool.Subject != "word" {
		t.Errorf("查字的 subject = %q，要的是 word", tool.Subject)
	}
	if tool.Shape != "hanwords" {
		t.Errorf("查字的 shape = %q，要的是 hanwords", tool.Shape)
	}
	if tool.Lang != "zh" {
		t.Errorf("查字的 lang = %q", tool.Lang)
	}
	// 文言文和诗词都要有 —— 一首诗里最常被问的就是一个字是什么意思。
	for _, genre := range []string{genreClassical, genrePoem} {
		if !containsString(tool.Genres, genre) {
			t.Errorf("查字没有服务体裁 %q", genre)
		}
	}
	// 它的产物仍然是词卡，所以荧光笔、报告那一侧都不用另写一份。
	if readingShapedSuffix[tool.Shape] == "" {
		t.Error("hanwords 没有对应的输出约定")
	}
	if !strings.Contains(readingShapedSuffix[tool.Shape], `"words"`) {
		t.Error("hanwords 的产物不是 words 数组 —— 荧光笔和报告都读不到它")
	}
}
