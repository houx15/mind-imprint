package api

import (
	"strings"
	"testing"

	"mindimprint/api/internal/store/sqlc"
)

// 同事 2026-09-17 逐字：「我总觉得不是所有的文章都应该按照主张、证据、限制
// 这样的内容来拆分，而且主张、证据、限制很多时候并不知道哪些该在哪里。」
//
// 同一天他交了一份阅读模块 PRD：议论文、说明文、记叙文、新闻报道各有各的
// 读法、各有各的板。产品负责人的条件是议论文的体验一点都不许变 ——
// 下面有几条测试专门锁住那一半。

func TestGenreClosedTable(t *testing.T) {
	for _, ok := range []string{"argument", "report", "narrative", "explain"} {
		if got := validateGenre(ok); got != ok {
			t.Errorf("validateGenre(%q) = %q", ok, got)
		}
	}
	if got := validateGenre("  ARGUMENT "); got != genreArgument {
		t.Errorf("大小写和空格应该收得住，得到 %q", got)
	}
	// 模型编的第五个体裁进不来 —— 和格子名、学科表是同一条纪律。
	for _, bad := range []string{"editorial", "议论文", "", "opinion piece"} {
		if got := validateGenre(bad); got != "" {
			t.Errorf("validateGenre(%q) = %q，编出来的体裁不该留下", bad, got)
		}
	}
}

// 每种语言、每种体裁都至少有一套读法 —— pickRoutineForGenre 才有地方可换。
func TestEveryGenreHasARoutineInEachLanguage(t *testing.T) {
	for _, lang := range []string{"zh", "en"} {
		for _, g := range []string{genreArgument, genreReport, genreNarrative, genreExplain} {
			found := false
			for _, r := range readingRoutinesFor(lang) {
				if r.serves(g) {
					found = true
				}
			}
			if !found {
				t.Errorf("%s 没有服务 %s 的读法", lang, g)
			}
		}
	}
}

// 🚨 议论文的三套读法，步骤只能按**产品负责人点过头的那一次**动。
//
// 这条测试原来叫「一个都不许动」，钉的是 2026-09-17 那句
// 「don't bother the current experience of argument papers」。它替产品负责人
// 看住过一次：2026-09-24 我照 R3 加了一步「拼出全文结构」，被它拦下来撤回了。
//
// 2026-09-25 产品负责人明确点头要 R3 和 R13（R5 那道闸不要），所以三套里
// 两套各多了一步 reflect·拼出全文结构，插在通读和精读之间 —— R3 那句
// 「…学生分段通读并概括 → **拼出全文结构** → 按文章类型精读…」的第三段。
//
// 🚨 en-close-read **没有动**：产品负责人说的是议论文那条路，而这一套是英文
// 精读，服务的体裁里虽然有议论文，它却不是照 R3 排的。少动一套少一份风险。
//
// 判据仍然逐字钉死整串 kind：这一族的步骤序会被别的改动顺手碰到 ——
// 插一步就把 plan 那边**按下标配对**的 detail 全部错位（见
// reading_plan_test.go 那份桩上面的注释），所以它要红得很吵。
func TestArgumentRoutinesAreUnchanged(t *testing.T) {
	want := map[string]string{
		"zh-scan-focus-lens": "predict read reflect focus_block label critique reflect connect hunt",
		"en-close-read":      "predict read focus_block label critique connect recall hunt",
		"en-argument":        "predict read reflect focus_block label critique connect hunt",
	}
	for key, kinds := range want {
		r, ok := findReadingRoutine(key)
		if !ok {
			t.Fatalf("%s 不见了", key)
		}
		got := make([]string, 0, len(r.Steps))
		for _, s := range r.Steps {
			got = append(got, string(s.Kind))
		}
		if strings.Join(got, " ") != kinds {
			t.Errorf("%s 的步骤变了：%v", key, got)
		}
		if !r.serves(genreArgument) {
			t.Errorf("%s 不再服务议论文", key)
		}
	}
}

func TestPickRoutineForGenre(t *testing.T) {
	zhArg, _ := findReadingRoutine("zh-scan-focus-lens")
	enClose, _ := findReadingRoutine("en-close-read")
	cases := []struct {
		chosen readingRoutine
		genre  string
		want   string
	}{
		// 议论文：模型挑哪套就是哪套（英文有两套可挑）。
		{zhArg, genreArgument, "zh-scan-focus-lens"},
		{enClose, genreArgument, "en-close-read"},
		// 认不出体裁：不动，那就是今天的样子。
		{zhArg, "", "zh-scan-focus-lens"},
		{enClose, "opinion", "en-close-read"},
		// 打架的时候换成同语言里服务这个体裁的那一套。
		{zhArg, genreReport, "zh-report"},
		{zhArg, genreExplain, "zh-explain"},
		{zhArg, genreNarrative, "zh-narrative"},
		{enClose, genreReport, "en-report"},
		{enClose, genreExplain, "en-explain"},
		{enClose, genreNarrative, "en-narrative"},
	}
	for _, c := range cases {
		if got := pickRoutineForGenre(c.chosen, c.genre); got.Key != c.want {
			t.Errorf("pick(%s, %q) = %s, want %s", c.chosen.Key, c.genre, got.Key, c.want)
		}
	}
}

// 排序步只出现在报道和记叙的读法里；说明文和议论文里没有。
func TestSequenceStepOnlyWhereEventsHaveAnOrder(t *testing.T) {
	for _, r := range readingRoutines {
		has := false
		for _, s := range r.Steps {
			if s.Kind == taskSequence {
				has = true
			}
		}
		wantSeq := r.serves(genreReport) || r.serves(genreNarrative)
		if has != wantSeq {
			t.Errorf("%s：排序步 = %v，想要 %v", r.Key, has, wantSeq)
		}
	}
}

func TestBoardBinsFollowGenre(t *testing.T) {
	board := &coachCard{Type: coachCardLabelRoles, Prompt: coachLabelBoardPrompt,
		Labels: coachArgueBinsBasic, Options: []coachCardOption{{BlockID: "b1", Quote: "x"}}}

	// 议论文、认不出来的体裁：原样返回，一个字节都不变。
	for _, g := range []string{genreArgument, ""} {
		if got := fitBoardToGenre(board, g); got != board {
			t.Errorf("genre %q 上板被换过了：%+v", g, got)
		}
	}
	want := map[string][]string{
		genreReport:    {"事实", "引述", "解释"},
		genreExplain:   {"说明对象", "原理与过程", "例子与数据"},
		genreNarrative: {"动作描写", "语言描写", "心理描写", "环境描写"},
	}
	for g, bins := range want {
		got := fitBoardToGenre(board, g)
		if strings.Join(got.Labels, "/") != strings.Join(bins, "/") {
			t.Errorf("%s 的格子 = %v", g, got.Labels)
		}
		// 议论文那句标准题目在别的体裁上说的是「论证成分」—— 必须换掉。
		if got.Prompt == coachLabelBoardPrompt {
			t.Errorf("%s 上还是议论文的题目", g)
		}
	}
	// 题目点了议论文的格子名，换成这块板的题目；没点就留着模型写的。
	named := &coachCard{Type: coachCardLabelRoles, Prompt: "哪几句是证据，哪几句是记者的推断？", Labels: coachArgueBinsBasic}
	if got := fitBoardToGenre(named, genreReport); got.Prompt != coachGenreBoards[genreReport].Prompt {
		t.Errorf("题目里点了「证据」，却没换：%q", got.Prompt)
	}
	own := &coachCard{Type: coachCardLabelRoles, Prompt: "这些句子分别是谁的说法？", Labels: coachArgueBinsBasic}
	if got := fitBoardToGenre(own, genreReport); got.Prompt != own.Prompt {
		t.Errorf("一道没点错格子名的题目被换掉了：%q", got.Prompt)
	}
	// 别的卡片类型不动。
	span := &coachCard{Type: coachCardChooseSpan, Prompt: "p"}
	if got := fitBoardToGenre(span, genreReport); got != span {
		t.Error("choose_span 被动过")
	}
}

// 她在报道上摆过的板，读回转写时格子名要认得出来 —— 否则「挪到它已经在的
// 那一格」那条判据在报道上就瞎了。
func TestLastPlacementReadsGenreBins(t *testing.T) {
	payload := coachCardAnswerPayload(&coachCardAnswer{
		Type: coachCardLabelRoles, Prompt: "p",
		Choice: "引述：\n“We will not leave,” said the mayor.\n事实：\nThe bridge closed on Monday.",
	})
	got := lastBoardPlacement([]sqlc.AtomMessage{{Role: "student", Payload: payload}})
	if got["The bridge closed on Monday."] != "事实" || len(got) != 2 {
		t.Errorf("读回来的摆放 = %v", got)
	}
}

// 🚨 议论文那条路一个字节都不变 —— 这是 2026-09-17 定的边界
// （「don't bother the current experience of argument papers」）。
// 认不出来的体裁同理，按议论文办。
//
// 放在这个内部测试文件（而不是 task brief 原写的 reading_genre_test.go）里，
// 是因为 buildGenreCoachSection / genreArgument 都是包内私有标识符 ——
// reading_genre_test.go 是 package api_test，编译不过；这个文件是 package api。
func TestOnlyUnknownGenresHaveNoCoachSection(t *testing.T) {
	// 认不出来的体裁仍然没有这一节 —— 拿一套读法去套一篇没判出体裁的文章，
	// 比不给更糟。这一半从 2026-09-17 起就没变过。
	for _, genre := range []string{"", "不认识的体裁"} {
		if got := buildGenreCoachSection(genre); got != "" {
			t.Errorf("体裁 %q 不该有带读说明，拿到 %d 字", genre, len([]rune(got)))
		}
	}
	// 🚨 议论文这一半 2026-09-25 反过来了：产品负责人点头要 R13
	// （示范额度最多一段），而那条规矩只在议论文上成立，通用那份说不出口。
	got := buildGenreCoachSection(genreArgument)
	if got == "" {
		t.Fatal("议论文没有带读说明 —— R13 没地方装")
	}
	if !strings.Contains(got, "示范") {
		t.Error("议论文那一段里没有示范额度那条规矩")
	}
}

// 另外三种体裁各自要拿到自己那一段，而且必须提到自己那块板的格子名。
func TestGenreCoachSectionNamesItsOwnBins(t *testing.T) {
	for genre, bin := range map[string]string{
		genreReport:    "引述",
		genreExplain:   "说明对象",
		genreNarrative: "心理描写",
	} {
		got := buildGenreCoachSection(genre)
		if got == "" {
			t.Errorf("体裁 %q 没有带读说明", genre)
			continue
		}
		if !strings.Contains(got, bin) {
			t.Errorf("体裁 %q 的带读说明里没有格子名 %q", genre, bin)
		}
	}
}

func TestGenreSectionOnlyOffArgument(t *testing.T) {
	// 🚨 2026-09-25：议论文从这张「不该有」的名单里挪走了（R13）。
	// 剩下的两种仍然在：空串和认不出来的体裁没有自己的读法。
	for _, g := range []string{"", "editorial"} {
		if s := buildGenreCoachSection(g); s != "" {
			t.Errorf("genre %q 上多了一节：%q", g, s)
		}
	}
	for _, g := range []string{genreReport, genreExplain, genreNarrative} {
		s := buildGenreCoachSection(g)
		if !strings.Contains(s, coachGenreBoards[g].Prompt) {
			t.Errorf("%s 那一节没说这块板的题目", g)
		}
		if strings.Contains(s, "order_events") != genreHasOrderBoard(g) {
			t.Errorf("%s 那一节对排序板的说法不对", g)
		}
	}
}

// 🚨 2026-09-25 这条也反过来了。
//
// 它原来钉的是「议论文的整段 prompt 必须和不带体裁时一字不差」，是
// 「议论文一个字都不动」在 prompt 那一侧的保证。产品负责人点头要 R13 之后，
// 议论文多了自己的一节，所以现在钉的是**另一件事**：
// 多出来的**只有**那一节 —— 把它原样减掉之后，仍然要和不带体裁时一字不差。
// 否则 R13 顺手改到了别的地方，而那是没人审过的。
func TestArgumentCoachPromptDiffersOnlyByItsOwnSection(t *testing.T) {
	blocks := outlineBlocks(4)
	tasks := []sqlc.ReadingTask{{Kind: string(taskLabel), Label: "拆开作者的论证", Status: "pending"}}
	arg := buildReadingCoachPrompt("t", blocks, readingOutline{OneLine: "问", Genre: genreArgument}, tasks, nil, nil, "我读完了", nil, "")
	none := buildReadingCoachPrompt("t", blocks, readingOutline{OneLine: "问"}, tasks, nil, nil, "我读完了", nil, "")
	// 多出来的只有议论文自己那一节：原样减掉之后必须和不带体裁时一字不差。
	if without := strings.Replace(arg, buildGenreCoachSection(genreArgument), "", 1); without != none {
		t.Error("议论文的 prompt 除了自己那一节，还有别的地方变了")
	}
	if arg == none {
		t.Error("议论文没有拿到自己那一节 —— R13 没装上")
	}
	rep := buildReadingCoachPrompt("t", blocks, readingOutline{OneLine: "问", Genre: genreReport}, tasks, nil, nil, "我读完了", nil, "")
	if !strings.Contains(rep, "【这篇的体裁") {
		t.Error("报道的 prompt 里没有体裁那一节")
	}
}

func TestOrderBoardShowsArticleOrder(t *testing.T) {
	blocks := []Block{
		{ID: "b1", Text: "The storm reached the coast on Friday night."},
		{ID: "b2", Text: "Officials had ordered an evacuation on Wednesday. Schools closed on Thursday."},
		{ID: "b3", Text: "By Sunday, power was restored to most homes."},
	}
	// 模型按它心里的发生顺序给 —— 那个顺序不能原样上屏。
	c, why := validateCoachCardWhy(&coachCard{Type: coachCardOrderEvents, Prompt: "这几件事是按什么先后发生的？",
		Options: []coachCardOption{
			{BlockID: "b2", Quote: "Officials had ordered an evacuation on Wednesday."},
			{BlockID: "b2", Quote: "Schools closed on Thursday."},
			{BlockID: "b1", Quote: "The storm reached the coast on Friday night."},
			{BlockID: "b3", Quote: "By Sunday, power was restored to most homes."},
		}}, blocks)
	if why != cardOK {
		t.Fatalf("排序板被拒：%s", why)
	}
	var got []string
	for _, o := range c.Options {
		got = append(got, o.BlockID)
	}
	if strings.Join(got, ",") != "b1,b2,b2,b3" {
		t.Errorf("没按原文顺序摆：%v", got)
	}
	if c.Options[1].Quote != "Officials had ordered an evacuation on Wednesday." || c.Options[0].Where != "第1段" {
		t.Errorf("同一段里的先后或段号不对：%+v", c.Options)
	}
	// 两件不成排序。
	if _, why := validateCoachCardWhy(&coachCard{Type: coachCardOrderEvents, Prompt: "排一排",
		Options: c.Options[:2]}, blocks); why != cardRejectFewOptions {
		t.Errorf("两件事的排序板应该被拒，得到 %q", why)
	}
}

func TestOrderBoardFallbackSpreadsAcrossParts(t *testing.T) {
	blocks := []Block{
		{ID: "b1", Text: "The storm reached the coast on Friday night."},
		{ID: "b2", Text: "Residents described water rising into their homes."},
		{ID: "b3", Text: "Officials had ordered an evacuation on Wednesday."},
		{ID: "b4", Text: "Many people refused to leave their homes."},
		{ID: "b5", Text: "By Sunday, power was restored to most homes."},
	}
	parts := []readingPart{{From: "b1", To: "b2"}, {From: "b3", To: "b4"}, {From: "b5", To: "b5"}}
	c := validateCoachCard(buildOrderBoard(blocks, parts), blocks)
	if c == nil || c.Type != coachCardOrderEvents || len(c.Options) != 3 {
		t.Fatalf("兜底排序板 = %+v", c)
	}
	if c.Options[0].BlockID != "b1" || c.Options[1].BlockID != "b3" || c.Options[2].BlockID != "b5" {
		t.Errorf("没有每个部分各取一句：%+v", c.Options)
	}
	// 没有切法：全篇等距取。
	if c := validateCoachCard(buildOrderBoard(blocks, nil), blocks); c == nil || len(c.Options) < 3 {
		t.Errorf("没有切法时兜不出板：%+v", c)
	}
	// 太短的文章不硬凑。
	if c := buildOrderBoard(blocks[:2], nil); c != nil {
		t.Errorf("两段的文章兜出了一块板：%+v", c)
	}
}

// 导读的校验把体裁一起收进闭表。
func TestOutlineKeepsTheGenre(t *testing.T) {
	blocks := outlineBlocks(6)
	load := fullLoad(6, loadSupport)
	load["b2"] = loadCore
	got, ok := validateOutline(readingOutline{
		OneLine: "这篇在问什么", Genre: "report", Load: load,
	}, blocks)
	if !ok {
		t.Fatal("这份导读该留下")
	}
	if got.Genre != genreReport {
		t.Errorf("genre = %q, want report", got.Genre)
	}
}
