package api

import (
	"strings"
	"testing"
)

// 产品负责人 2026-09-23：
//
//	「since we will face reading poems/文言文 in chinese reading, and writing
//	  letters in english writing. these are very important scene in junior
//	  study.」
//
// 🚨 在这之前这两种都会落到原来那四种里的某一个：一首绝句被当成「记叙」去排
// 事件顺序，一篇《陋室铭》被当成「说明」去找说明对象。两种都不是它们。

func TestPoemAndClassicalAreRealGenres(t *testing.T) {
	for _, g := range []string{genrePoem, genreClassical} {
		if validateGenre(g) != g {
			t.Errorf("%s 不在阅读的体裁闭表里 —— 模型判出来也会被丢掉", g)
		}
		// 每一种都要有服务它的读法，否则 pickRoutineForGenre 会「保留模型挑的
		// 那一套」—— 一首诗因此可能拿到按记叙文排的清单，而这件事不报错。
		var served []string
		for _, r := range readingRoutines {
			if r.serves(g) {
				served = append(served, r.Key)
			}
		}
		if len(served) == 0 {
			t.Errorf("没有一套读法服务 %s", g)
		}
	}
}

// 🚨 一首诗上没有「几件事的先后」可排。摆一块排序板给她，等于要她把
// 「床前明月光 / 疑是地上霜」排出先后 —— 那不是这首诗里的东西。
func TestPoemAndClassicalGetNoOrderBoard(t *testing.T) {
	for _, g := range []string{genrePoem, genreClassical} {
		if genreHasOrderBoard(g) {
			t.Errorf("%s 上摆了排序板", g)
		}
	}
	// 报道和记叙照旧有 —— 加两种体裁不许把原来那两种弄坏。
	for _, g := range []string{genreReport, genreNarrative} {
		if !genreHasOrderBoard(g) {
			t.Errorf("%s 的排序板没了", g)
		}
	}
}

// 两种各有自己的一块标注板，格子是语文课上真有的词。
func TestPoemAndClassicalHaveTheirOwnBoards(t *testing.T) {
	poem, ok := genreBoardFor(genrePoem)
	if !ok {
		t.Fatal("诗词没有自己的标注板")
	}
	if strings.Join(poem.Bins, "/") != "写景/叙事/抒情/说理" {
		t.Errorf("诗词的格子是 %v", poem.Bins)
	}
	cls, ok := genreBoardFor(genreClassical)
	if !ok {
		t.Fatal("文言文没有自己的标注板")
	}
	if strings.Join(cls.Bins, "/") != "叙事/描写/议论/对话" {
		t.Errorf("文言文的格子是 %v", cls.Bins)
	}
	// 🚨 两块板不许是同一套：它们要分的东西不一样（诗词分表达方式里的
	// 景与情，文言文要认出「议」从哪一句开始）。
	if strings.Join(poem.Bins, "/") == strings.Join(cls.Bins, "/") {
		t.Error("诗词和文言文用了同一套格子")
	}
}

// 两种都要有自己的带读说明 —— 取不到会静默变成「印记这一次话比平时少」。
func TestPoemAndClassicalHaveACoachSection(t *testing.T) {
	for _, g := range []string{genrePoem, genreClassical} {
		got := buildGenreCoachSection(g)
		if strings.TrimSpace(got) == "" {
			t.Fatalf("%s 没有带读说明", g)
		}
		if strings.Contains(got, "@@") {
			t.Errorf("%s 的带读说明里有没换掉的占位符", g)
		}
	}
	// 诗词那一段里要有讲义最值钱的那一条：说出手法的名字之后要说它做成了什么。
	poem := buildGenreCoachSection(genrePoem)
	if !strings.Contains(poem, "做成了什么") {
		t.Error("诗词那一段没有「说出手法之后要说它做成了什么」——" +
			"中学生的诗歌鉴赏最常见的失败就是背出手法名字然后停住")
	}
	// 文言文那一段要有「一次只讲一层」：讲义里那条「分层译讲，不堆砌」。
	cls := buildGenreCoachSection(genreClassical)
	if !strings.Contains(cls, "一次只讲一层") {
		t.Error("文言文那一段没有「一次只讲一层」——" +
			"一句话里把字词、句式、寓意全讲完，她一样都记不住")
	}
}

// 🚨 文言文和诗词的段落工具**只在它们自己的体裁上出现**。
//
// 一篇现代散文的工具条上摆着「字词释义（通假字、古今异义）」，是一个按下去
// 讲不出东西的按钮，而没有意义的按钮会让她不信任整条工具条。
func TestGenreToolsOnlyShowUpOnTheirOwnGenre(t *testing.T) {
	has := func(tools []readingBlockTool, id string) bool {
		for _, t := range tools {
			if t.ID == id {
				return true
			}
		}
		return false
	}

	cls := readingBlockToolsFor("zh", genreClassical)
	if !has(cls, "classical_words") || !has(cls, "classical_syntax") {
		t.Error("文言文上没有字词释义 / 句法")
	}
	if has(cls, "poem_images") {
		t.Error("文言文上摆了诗词的意象工具")
	}

	poem := readingBlockToolsFor("zh", genrePoem)
	if !has(poem, "poem_images") {
		t.Error("诗词上没有意象工具")
	}
	if has(poem, "classical_words") {
		t.Error("诗词上摆了文言文的字词释义")
	}

	// 🚨 别的体裁、以及**体裁认不出来的时候**，三件都不给。
	// 方向和别处的「空 = 不限」相反，故意的：少一件工具的代价，
	// 比多一件按下去讲不出东西的小得多。
	for _, g := range []string{genreArgument, genreNarrative, genreReport, genreExplain, ""} {
		tools := readingBlockToolsFor("zh", g)
		for _, id := range []string{"classical_words", "classical_syntax", "poem_images"} {
			if has(tools, id) {
				t.Errorf("体裁 %q 的工具条上摆了 %s", g, id)
			}
		}
		// 原来那三件中文工具一件都不许丢。
		for _, id := range []string{"rhetoric", "examples", "structure"} {
			if !has(tools, id) {
				t.Errorf("体裁 %q 上少了原有的工具 %s", g, id)
			}
		}
	}

	// 报告那一侧要认得出每一件 —— 她用过文言文的字词释义，报告里不该漏掉那一行。
	if !has(readingBlockToolsAll(), "classical_words") {
		t.Error("报告那一侧查不到 classical_words，她用过的那一行会消失")
	}
}

// 英文那一边一件都没多 —— 加两种中文体裁不许动英文那一套。
func TestEnglishToolsUnchanged(t *testing.T) {
	before := []string{"translate", "vocabulary", "lookup", "grammar", "craft"}
	got := readingBlockToolsFor("en", "")
	for _, id := range before {
		var found bool
		for _, tool := range got {
			if tool.ID == id {
				found = true
			}
		}
		if !found {
			t.Errorf("英文那边少了 %s", id)
		}
	}
	for _, tool := range got {
		if strings.HasPrefix(tool.ID, "classical_") || tool.ID == "poem_images" {
			t.Errorf("英文那边混进了 %s", tool.ID)
		}
	}
}
