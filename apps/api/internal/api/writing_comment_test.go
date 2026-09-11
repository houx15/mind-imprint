package api

import (
	"testing"

	"mindimprint/api/internal/store/sqlc"
	"mindimprint/api/internal/vocab"
)

// 这些测的都是「读代码看不出对错」的那一类：一条意见在什么情况下会被丢掉。
// 每一条对应 writing_comment.go 里一条写成代码的规矩——prompt 里的「必须」
// 如果代码里验不了，它就只是一句期望。

const commentSource = "我家楼下那条路就是这样，两排树掘得密密麻麻，长了几年也还是瘦瘦的一根杆。"

// issue 造一条合法的「要改的」意见，再按需要改坏其中一个字段。
func issue(sym, quote string) CommentPoint {
	return CommentPoint{Kind: "issue", Symptom: sym, Text: "说清怎么了。", Action: "把这句里的数字去掉，换成你看到的那一棵。", Quote: quote}
}

// The quote IS the trace. A point whose quote is not literally in her text would
// send her to a sentence she never wrote — "specific, confident and wrong", the
// one failure mode this room refuses. Drop it; never guess.
func TestValidateCommentPoints_DropsQuotesSheNeverWrote(t *testing.T) {
	in := []CommentPoint{
		issue("abstract_and_dry", "两排树掘得密密麻麻"),
		issue("abstract_and_dry", "根据 2019 年的一项研究"),
		issue("abstract_and_dry", ""),
	}
	got := validateCommentPoints(in, commentSource, "zh", 3)
	if len(got) != 1 {
		t.Fatalf("kept %d points %v, want only the one whose quote is really in her text", len(got), got)
	}
	if got[0].Quote != "两排树掘得密密麻麻" {
		t.Fatalf("kept the wrong point: %+v", got[0])
	}
}

// 🚨 这条是这次改动的核心：**只留最上面那一层**。
//
// 四份互不相干的材料都写了同一句话（见 writing_symptoms.go）。一篇主张还没立住
// 的文章，收到的第一条意见不该是某个词不准——而这一刀只有一次。
func TestValidateCommentPoints_KeepsOnlyTheTopLayer(t *testing.T) {
	in := []CommentPoint{
		issue("sentence_bloat", "两排树掘得密密麻麻"),      // 第 4 层
		issue("claim_not_stated", "长了几年也还是瘦瘦的一根杆。"), // 第 1 层
		issue("loose_whole", "我家楼下那条路就是这样"),        // 第 3 层
	}
	got := validateCommentPoints(in, commentSource, "zh", 3)
	if len(got) != 1 {
		t.Fatalf("kept %d points, want only the single layer-1 one: %+v", len(got), got)
	}
	if got[0].Symptom != "claim_not_stated" || got[0].Layer != writingLayerClaim {
		t.Fatalf("kept the wrong layer: %+v", got[0])
	}
}

// 层是从闭表查出来的，不采信模型报的值——模型已经挑了一个 symptom，
// 再让它报一遍层只是多一个会错的字段。
func TestValidateCommentPoints_DerivesLayerAndIgnoresTheModelsClaim(t *testing.T) {
	p := issue("sentence_bloat", "两排树掘得密密麻麻")
	p.Layer = writingLayerClaim // 模型撒谎说这是第 1 层
	got := validateCommentPoints([]CommentPoint{p}, commentSource, "zh", 3)
	if len(got) != 1 || got[0].Layer != writingLayerSentence {
		t.Fatalf("layer must come from the table, not the model: %+v", got)
	}
}

// 编出来的 symptom 挂不上任何技法，她拿它没有下一步可做 —— 整条丢掉。
func TestValidateCommentPoints_DropsInventedSymptoms(t *testing.T) {
	in := []CommentPoint{issue("vibes_are_off", "两排树掘得密密麻麻")}
	if got := validateCommentPoints(in, commentSource, "zh", 3); len(got) != 0 {
		t.Fatalf("kept an invented symptom id: %+v", got)
	}
}

// "Diagnostic Without Return" —— 一条没有下一步的意见把她的下一步收走了。
func TestValidateCommentPoints_DropsPointsWithNoNextStep(t *testing.T) {
	p := issue("abstract_and_dry", "两排树掘得密密麻麻")
	p.Action = "   "
	if got := validateCommentPoints([]CommentPoint{p}, commentSource, "zh", 3); len(got) != 0 {
		t.Fatalf("kept a point with no action: %+v", got)
	}
}

// 对着文字说，别对着人说。
func TestValidateCommentPoints_DropsVerdictsAboutHer(t *testing.T) {
	p := issue("abstract_and_dry", "两排树掘得密密麻麻")
	p.Text = "你太懒了，这里根本没写。"
	if got := validateCommentPoints([]CommentPoint{p}, commentSource, "zh", 3); len(got) != 0 {
		t.Fatalf("kept a verdict about her rather than her text: %+v", got)
	}

	// 而一句对着她说的**祈使**是好的，不能误伤。
	ok := issue("abstract_and_dry", "两排树掘得密密麻麻")
	ok.Action = "把你这句里的「密密麻麻」换成你真的数过的那个数。"
	if got := validateCommentPoints([]CommentPoint{ok}, commentSource, "zh", 3); len(got) != 1 {
		t.Fatalf("an imperative addressed to her must survive: %+v", got)
	}
}

// 一条肯定排在最前面，而且只留一条 —— 「很有灵气」说多了就不值钱了。
func TestValidateCommentPoints_OneGoodFirst(t *testing.T) {
	in := []CommentPoint{
		issue("claim_not_stated", "长了几年也还是瘦瘦的一根杆。"),
		{Kind: "good", Method: "point_pee", Text: "这个例子看得见。", Quote: "两排树掘得密密麻麻"},
		{Kind: "good", Method: "point_pee", Text: "第二条肯定。", Quote: "我家楼下那条路就是这样"},
	}
	got := validateCommentPoints(in, commentSource, "zh", 3)
	if len(got) != 2 {
		t.Fatalf("want exactly one good + one issue, got %d: %+v", len(got), got)
	}
	if got[0].Kind != "good" {
		t.Fatalf("the good point must come first: %+v", got)
	}
	if got[1].Kind != "issue" {
		t.Fatalf("the issue must follow the good one: %+v", got)
	}
}

// good 上认不出来的方法 id 只清掉那个字段，**不丢掉整条** —— 它是锦上添花，
// 不是这条意见成立的条件。
func TestValidateCommentPoints_UnknownMethodOnlyClearsTheField(t *testing.T) {
	in := []CommentPoint{{Kind: "good", Method: "not_a_method", Text: "这句看得见。", Quote: "两排树掘得密密麻麻"}}
	got := validateCommentPoints(in, commentSource, "zh", 3)
	if len(got) != 1 {
		t.Fatalf("an unknown method must not drop the whole point: %+v", got)
	}
	if got[0].Method != "" {
		t.Fatalf("the unknown method id should have been cleared: %+v", got[0])
	}
}

// 一段只给一条要改的。列五条，她很可能只记住「我写得很烂」。
func TestValidateCommentPoints_BlockCommentGivesOneIssue(t *testing.T) {
	in := []CommentPoint{
		issue("abstract_and_dry", "两排树掘得密密麻麻"),
		issue("abstract_and_dry", "我家楼下那条路就是这样"),
		issue("abstract_and_dry", "长了几年也还是瘦瘦的一根杆。"),
	}
	got := validateCommentPoints(in, commentSource, "zh", writingBlockCommentMaxIssues)
	if len(got) != 1 {
		t.Fatalf("a block comment must carry exactly one issue, got %d: %+v", len(got), got)
	}
}

// 中英两张表是分开的：中文那张里的 id 在英文稿子上不成立，反过来也一样。
// 合成一张的话，一篇中文作文会收到「冠词和可数」这种意见。
func TestValidateCommentPoints_SymptomTablesAreLanguageScoped(t *testing.T) {
	zhOnly := issue("flat_chronicle", "两排树掘得密密麻麻")
	if got := validateCommentPoints([]CommentPoint{zhOnly}, commentSource, "en", 3); len(got) != 0 {
		t.Fatalf("a Chinese-only symptom must not validate on an English piece: %+v", got)
	}

	enOnly := issue("article_control", "两排树掘得密密麻麻")
	if got := validateCommentPoints([]CommentPoint{enOnly}, commentSource, "zh", 3); len(got) != 0 {
		t.Fatalf("an English-only symptom must not validate on a Chinese piece: %+v", got)
	}
	if got := validateCommentPoints([]CommentPoint{enOnly}, commentSource, "en", 3); len(got) != 1 {
		t.Fatalf("an English symptom must validate on an English piece: %+v", got)
	}
}

// 症状表本身的不变量：id 不重名，层都在范围里，每一条都有可观察的信号。
// 🚨 最后一条是这张表存在的理由 —— 信号必须是她自己能验的现象，不是一个形容词。
func TestWritingSymptomTablesAreWellFormed(t *testing.T) {
	for _, lang := range []string{"zh", "en"} {
		seen := map[string]bool{}
		for _, s := range writingSymptomTable(lang) {
			if seen[s.ID] {
				t.Fatalf("%s: duplicate symptom id %q", lang, s.ID)
			}
			seen[s.ID] = true
			if s.Layer < writingLayerClaim || s.Layer > writingLayerSentence {
				t.Fatalf("%s: symptom %q has layer %d, outside 1..4", lang, s.ID, s.Layer)
			}
			if s.Name == "" || s.Signal == "" {
				t.Fatalf("%s: symptom %q needs both a name and an observable signal", lang, s.ID)
			}
		}
		// 每一层都得有东西，否则「只留最上面那一层」在那一层上是空转。
		byLayer := map[int]int{}
		for _, s := range writingSymptomTable(lang) {
			byLayer[s.Layer]++
		}
		for l := writingLayerClaim; l <= writingLayerSentence; l++ {
			if byLayer[l] == 0 {
				t.Fatalf("%s: layer %d has no symptoms", lang, l)
			}
		}
	}
}

// 🚨 系统提示词说「method 取自【可用的方法】」，那份表就必须真的在用户那一轮里。
//
// 这条测试是被 LIVE_LLM 实测逼出来的：prompt 一直这么写，而 builder 从来没把
// 表放进去，于是真模型回了 `concrete_data` / `specific_detail` 两个不存在的 id，
// 校验器把字段清空 ——「说出她用对了哪个方法」这件事一次都没发生过。
// 单元测试当时全绿（我喂的 JSON 里写的是真 id），屏幕上也看不出来。
//
// 凡是提示词里点名要模型从某张表里挑的东西，那张表在不在 prompt 里，
// 都是可以这样机械地验一次的。
func TestWritingCommentPrompt_CarriesTheMethodLibrary(t *testing.T) {
	for _, lang := range []string{"zh", "en"} {
		prompt := buildWritingCommentPrompt(sqlc.Writing{Title: "食堂浪费", Lang: lang}, "她写的这一段", "随便一句。")
		if !contains(prompt, "【可用的方法】") {
			t.Fatalf("%s: prompt 里没有【可用的方法】这一节", lang)
		}
		methods := vocab.ForLang(lang)
		if len(methods) == 0 {
			t.Fatalf("%s: 方法库是空的", lang)
		}
		for _, m := range methods {
			if !contains(prompt, m.ID) {
				t.Fatalf("%s: prompt 里缺方法 id %q —— 模型挑不到它，只会自己造一个", lang, m.ID)
			}
		}
	}
}

// prompt 里那份目录必须真的把 id 写出来 —— 模型要回填的就是它。
func TestWritingSymptomCatalogListsEveryID(t *testing.T) {
	for _, lang := range []string{"zh", "en"} {
		cat := writingSymptomCatalog(lang)
		for _, s := range writingSymptomTable(lang) {
			if !contains(cat, s.ID) {
				t.Fatalf("%s: catalog is missing %q", lang, s.ID)
			}
		}
	}
}

func contains(hay, needle string) bool {
	return len(hay) >= len(needle) && (func() bool {
		for i := 0; i+len(needle) <= len(hay); i++ {
			if hay[i:i+len(needle)] == needle {
				return true
			}
		}
		return false
	})()
}
