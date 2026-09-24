package api

import (
	"strings"
	"testing"
)

// reading_block_words_internal_test.go —— 关键单词那组词卡。
//
// 这里守的是**荧光笔能不能落下去**那条判据：卡片上的 term 必须逐字出现在它
// 那一段里。这条不是洁癖 ——
//
//   - 模型很爱把 scrambling 还原成 scramble（词典形）。还原之后正文里一个字
//     都找不到，那个词就标不出来，而卡片上明明写着它。
//   - 一张指着这一段里没有的词的卡片，讲解也没法验：她拿着卡片回正文里找，
//     找不到，只能得出「这个功能坏了」的结论。
//
// prompt 里当然也写了这一条，但一句 prompt 里的「必须」如果代码里验不了，
// 它就只是一句期望（memory: prompt-output-must-be-verifiable-2026-09-03）。

const wordPara = "Rescue teams were scrambling to reach the valley before the river crested, " +
	"and the carbon footprint of the airlift was the last thing on anyone's mind."

func TestParseWordCardsKeepsOnlyWordsThatAreReallyInTheParagraph(t *testing.T) {
	raw := `{"words":[
	  {"term":"scrambling","pos":"动词","meaning":"手忙脚乱地赶","note":"","example":"x","exampleZh":"x"},
	  {"term":"crested","pos":"动词","meaning":"涨到最高点","note":"","example":"y","exampleZh":"y"},
	  {"term":"unprecedented","pos":"形容词","meaning":"前所未有的","note":"","example":"z","exampleZh":"z"}
	]}`
	got, ok := parseWordCards(raw, wordPara)
	if !ok {
		t.Fatal("这一段里明明有两个词对得上，却整件工具算失败了")
	}
	if len(got) != 2 {
		t.Fatalf("留下了 %d 张卡片，want 2：%+v", len(got), got)
	}
	for _, w := range got {
		if !strings.Contains(wordPara, w.Term) {
			t.Errorf("留下了一个段落里没有的词：%q", w.Term)
		}
	}
}

// 🚨 模型把句首那个词写成小写、或者把整个词大写，都不该让它标不出来 ——
// 但**显示的必须是正文里的那一份写法**，因为荧光笔按它去找。
func TestParseWordCardsTakesTheParagraphsOwnSpelling(t *testing.T) {
	got, ok := parseWordCards(
		`{"words":[{"term":"RESCUE teams","pos":"名词","meaning":"救援队"}]}`, wordPara)
	if !ok || len(got) != 1 {
		t.Fatalf("大小写不同就丢掉了：%+v", got)
	}
	if got[0].Term != "Rescue teams" {
		t.Errorf("term = %q，want 正文里的写法「Rescue teams」", got[0].Term)
	}
}

// 词典形（scrambling → scramble）在正文里找不到。找不到就是没有，不猜。
func TestParseWordCardsDropsALemmaThatIsNotInTheText(t *testing.T) {
	got, ok := parseWordCards(
		`{"words":[{"term":"scramble","pos":"动词","meaning":"手忙脚乱"}]}`, wordPara)
	if ok {
		t.Errorf("还原成原形的词不在正文里，却被留下了：%+v", got)
	}
}

func TestParseWordCardsDropsEmptyAndDuplicateEntries(t *testing.T) {
	raw := `{"words":[
	  {"term":"","pos":"","meaning":"没有词"},
	  {"term":"crested","pos":"动词","meaning":""},
	  {"term":"scrambling","pos":"动词","meaning":"手忙脚乱地赶"},
	  {"term":"Scrambling","pos":"动词","meaning":"重复的一张"}
	]}`
	got, ok := parseWordCards(raw, wordPara)
	if !ok || len(got) != 1 {
		t.Fatalf("空的、没意思的、重复的都该丢掉，剩下：%+v", got)
	}
}

// 一张都不剩就是失败。绝不返回一组空卡片 —— 一个空的「关键单词」在屏幕上和
// 「这一段没有值得学的词」长得一样，而后者是一句我们没有说过的话。
func TestParseWordCardsFailsInsteadOfReturningNothing(t *testing.T) {
	for _, raw := range []string{
		"",
		"抱歉，我无法完成。",
		`{"words":[]}`,
		`{"words":[{"term":"unprecedented","meaning":"前所未有的"}]}`,
	} {
		if _, ok := parseWordCards(raw, wordPara); ok {
			t.Errorf("垃圾输入 %q 应该算失败", raw)
		}
	}
}

// 最多五张。多出来的十有八九是模型在凑数（挑最长的那几个词）。
func TestParseWordCardsCapsTheDeck(t *testing.T) {
	para := "alpha bravo charlie delta echo foxtrot golf"
	raw := `{"words":[
	  {"term":"alpha","meaning":"一"},{"term":"bravo","meaning":"二"},
	  {"term":"charlie","meaning":"三"},{"term":"delta","meaning":"四"},
	  {"term":"echo","meaning":"五"},{"term":"foxtrot","meaning":"六"},
	  {"term":"golf","meaning":"七"}
	]}`
	got, ok := parseWordCards(raw, para)
	if !ok {
		t.Fatal("parse failed")
	}
	if len(got) != readingWordsMax {
		t.Errorf("留下了 %d 张，want %d", len(got), readingWordsMax)
	}
}

// body 那一列要在没有解析器的地方也读得懂 —— 日后的报告、教师端、一次 psql。
func TestWordCardsAsProseCarriesEveryField(t *testing.T) {
	prose := wordCardsAsProse([]readingWord{{
		Term: "crested", Pos: "动词", Meaning: "涨到最高点",
		Note: "crest 是浪尖", Example: "The river crested at dawn.", ExampleZh: "河水在黎明涨到最高点。",
	}})
	for _, want := range []string{"crested", "动词", "涨到最高点", "crest 是浪尖", "The river crested at dawn.", "河水在黎明涨到最高点。"} {
		if !strings.Contains(prose, want) {
			t.Errorf("纯文字形态里少了 %q：\n%s", want, prose)
		}
	}
}

// 语法那件工具讲的是一句话。prompt 里要写死「只讲这一句」，否则模型会顺手把
// 整段讲一遍 —— 而她点的是那一句。
func TestBuildReadingBlockPromptScopesToOneSentence(t *testing.T) {
	blocks := []Block{{ID: "b1", Text: wordPara}}
	sentence := "Rescue teams were scrambling to reach the valley before the river crested,"

	withSentence := buildReadingBlockPrompt("洪水", blocks, 0, sentence)
	if !strings.Contains(withSentence, "【要讲解的这一句】") {
		t.Errorf("没有把这一句单独拎出来：\n%s", withSentence)
	}
	if !strings.Contains(withSentence, sentence) {
		t.Error("她点的那一句没有进 prompt")
	}
	if !strings.Contains(withSentence, "不要讲解整段") {
		t.Error("没有拦住它去讲整段")
	}

	// 不给句子时形状不变 —— 其余七件工具走的还是老路。
	whole := buildReadingBlockPrompt("洪水", blocks, 0, "")
	if !strings.Contains(whole, "【要讲解的这一段】") {
		t.Errorf("整段那一路的 prompt 变了：\n%s", whole)
	}
	if strings.Contains(whole, "【要讲解的这一句】") {
		t.Error("没点句子却拎出了一句")
	}
}

// 按句子讲的工具是**闭表**，而且判据是一句话：**换一句话，它讲的东西会不会变。**
//
// 会变的才按句子讲；不变的带上 subject 只会让同一段缓存出好几份一模一样的
// 讲解。「关键单词」按句子挑词根本讲不通，所以它按整段。
//
// 🚨 2026-09-23 从「只有 grammar」扩成三件。新加的两件是文言文专用的
// （字词释义 / 句法），而它们和 grammar 是同一个形状：她卡在哪一句，就讲哪
// 一句 —— 换一句，要讲的通假字和句式整个不一样。
// 判据从「只能是 grammar」改成这张闭表加上面那条理由，是因为原来那一版钉的
// 是**名单**，而名单本身说不出为什么。
func TestOnlyPerSentenceToolsTeachOneSentence(t *testing.T) {
	// 🚨 2026-09-24 又加了一件：白话翻译。
	//
	// 它过得了那条判据 —— **换一句话，译文当然整个不一样**，这是这张表里
	// 最显然的一件。它之所以到今天才有，是因为中文那边原来只有第二层
	// （字词释义）和第三层（句法）两颗按钮，而那份古文讲义里第一层
	// （白话翻译）才是**默认层**：
	//
	//	分层译讲，不堆砌：按「白话翻译 → 关键字词 → 句法 → 背景寓意」
	//	四层递进；用户要哪层给哪层，不强行全给。
	//
	// 产品负责人 2026-09-24：「students may select some texts and need the
	// explanation/translation」。
	perSentence := map[string]bool{
		"grammar":             true, // 英文：这一句的句法、词法、时态
		"classical_words":     true, // 文言文：这一句里的通假、古今异义、专名、典故
		"classical_syntax":    true, // 文言文：这一句的判断句 / 宾语前置 / 被动 / 省略
		"classical_translate": true, // 文言文与诗词：这几个字的白话（第一层）
	}
	for _, tool := range append(append([]readingBlockTool{}, readingBlockTools...), readingWritingTools...) {
		if (tool.Subject == "sentence") != perSentence[tool.ID] {
			t.Errorf("%s 的 subject = %q —— 按句子讲的工具见这条测试里那张闭表，"+
				"加一件要同时说清楚「换一句话它讲的东西会不会变」", tool.ID, tool.Subject)
		}
	}
}

// 关键单词的 shape 决定它产出卡片而不是散文。写错了不会报错，只会让她拿回
// 一段 JSON 当讲解读。
func TestVocabularyToolProducesWordCards(t *testing.T) {
	tool, ok := findReadingBlockTool("vocabulary")
	if !ok {
		t.Fatal("关键单词这件工具不见了")
	}
	if tool.Shape != "words" {
		t.Errorf("shape = %q，want words", tool.Shape)
	}
	if !strings.Contains(readingShapedSuffix["words"], `"pos"`) {
		t.Error("words 那段格式说明里没有词性")
	}
	for _, field := range []string{`"term"`, `"meaning"`, `"note"`, `"example"`} {
		if !strings.Contains(readingShapedSuffix["words"], field) {
			t.Errorf("words 那段格式说明里少了 %s", field)
		}
	}
}
