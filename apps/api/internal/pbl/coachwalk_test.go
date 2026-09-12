package pbl

import "testing"

// 这些判据决定报告里会写下几条「产品缺陷」。判据本身错了，走查就会把一个走通了
// 的陪练记成走不通的——2026-09-04 和 09-12 各栽过一次，所以它们必须先有测试。

func TestCountQuestionsCountsBothScripts(t *testing.T) {
	for _, c := range []struct {
		in   string
		want int
	}{
		{"你打算怎么量？", 1},
		{"你打算怎么量?", 1},
		{"称什么？在哪称？", 2},
		{"先说说你的想法。", 0},
		{"", 0},
	} {
		if got := countQuestions(c.in); got != c.want {
			t.Errorf("countQuestions(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestEchoesLastStudentNeedsFourRunesOfHerOwnWords(t *testing.T) {
	recent := []Turn{
		{Role: "ai", Content: "你打算怎么量？"},
		{Role: "student", Content: "我这周已经称了三天了，平均每天大概 47 公斤剩饭。"},
	}
	if !echoesLastStudent("你说你称了三天了，那第四天呢？", recent) {
		t.Error("接住了她的原话，却判成接不住")
	}
	if echoesLastStudent("很好，继续保持，你做得很棒！", recent) {
		t.Error("一句空洞鼓励，却判成接住了")
	}
	// 🚨 只找最后一句学生的话。印记接住的若是三轮前的内容，这一轮仍算接不住——
	// 否则一个在原地绕的陪练会因为反复引用同一句旧话而每轮都「合格」。
	older := []Turn{
		{Role: "student", Content: "称重量吧，数份数太不准了。"},
		{Role: "ai", Content: "那你打算什么时候称？"},
		{Role: "student", Content: "午饭结束后统一称。"},
	}
	if echoesLastStudent("你刚才说数份数太不准了", older) {
		t.Error("引用的是更早那句，不该算接住了她上一句")
	}
}

func TestEchoesLastStudentDoesNotPunishVeryShortReplies(t *testing.T) {
	// 她只说了「不知道」这种极短的话时，任何回复都可能偶然命中或偶然不命中。
	// 这种轮次不判——宁可漏掉一次，也不要凭巧合记一条犯规。
	recent := []Turn{{Role: "student", Content: "嗯"}}
	if !echoesLastStudent("那我们换个角度看。", recent) {
		t.Error("她的话短于四个字时不应该判犯规")
	}
}

// 🚨 走查台自己的回归测试。判据的第一版只折空白，于是印记把她的原话加了一对
// 引号再说回来时，被判成「接不住她」——一条读起来和真缺陷一模一样的假缺陷。
func TestEchoesLastStudentIgnoresPunctuationTheCoachAdds(t *testing.T) {
	recent := []Turn{{Role: "student", Content: "我咋知道他们是因为打多了还是不好吃才剩的？"}}
	if !echoesLastStudent("先别急着分「打多了」还是「不好吃」——那个在收餐台看不出来。", recent) {
		t.Error("印记引用了她的原话，只是加了引号，不该判成接不住")
	}
	if !echoesLastStudent("你说的 47 公斤，是怎么称出来的？",
		[]Turn{{Role: "student", Content: "平均每天大概47公斤剩饭"}}) {
		t.Error("数字中间被插了空格，不该判成接不住")
	}
}

func TestEchoesLastStudentIgnoresWhitespaceReflow(t *testing.T) {
	recent := []Turn{{Role: "student", Content: "平均每天大概 47 公斤剩饭"}}
	if !echoesLastStudent("你说的「47 公斤」是怎么称出来的？", recent) {
		t.Error("模型重新排了空格，不该算作转述")
	}
}

func TestRepeatOfCatchesTheSameQuestionRewordedAndSpareseDifferentOnes(t *testing.T) {
	prev := []WalkTurn{
		{N: 1, Reply: "你打算在什么时间点称剩饭？是午饭刚结束，还是等大家都走了？"},
		{N: 2, Reply: "那三天里最多的一天和最少的一天，差了多少？"},
	}
	// 换了说法的同一个问题，对学生来说是同一个问题又被问了一遍。
	if got := repeatOf("你准备什么时间点去称剩饭？午饭刚结束，还是等大家都走了？", prev); got != 1 {
		t.Errorf("换了说法的重复没被抓到：got %d, want 1", got)
	}
	// 一个真正的新问题不能被误判成重复，否则报告会凭空多出一堆犯规。
	if got := repeatOf("食堂师傅怎么看这件事？", prev); got != 0 {
		t.Errorf("新问题被误判成第 %d 轮的重复", got)
	}
	if got := repeatOf("", prev); got != 0 {
		t.Errorf("空回复不该算重复：got %d", got)
	}
}

func TestRepeatOfSkipsTurnsThatFailedToParse(t *testing.T) {
	// 解析失败那一轮没有 reply，拿空字符串去比会把任何一句都判成重复。
	prev := []WalkTurn{{N: 1, ParseErr: "pbl: unexpected end of JSON input"}}
	if got := repeatOf("你打算怎么量？", prev); got != 0 {
		t.Errorf("不该和解析失败的那一轮比：got %d", got)
	}
}

func TestViolationsReportsParseFailureAloneAndStopsThere(t *testing.T) {
	// 一轮解析失败时，别再给同一轮叠上「接不住她」「没问问题」——那一轮根本没有
	// reply，叠出来的三条会让一次故障在总表里看起来像三处缺陷。
	w := &WalkLog{Turns: []WalkTurn{{N: 1, ParseErr: "pbl: no reply"}}}
	vs := w.Violations()
	if len(vs) != 1 || vs[0].Kind != "parse" {
		t.Fatalf("解析失败那一轮应当只记一条 parse：%+v", vs)
	}
}

func TestViolationsCountsTheCheckableTieluBreaches(t *testing.T) {
	w := &WalkLog{Turns: []WalkTurn{
		{N: 1, Reply: "你打算怎么量？", QuestionsInReply: 1, EchoesHer: true},
		{N: 2, Reply: "称什么？在哪称？", QuestionsInReply: 2, EchoesHer: true},
		{N: 3, Reply: "很好，继续努力。", QuestionsInReply: 0, EchoesHer: false},
		{N: 4, Reply: "你打算怎么量？", QuestionsInReply: 1, EchoesHer: true, RepeatOf: 1},
	}}
	vs := w.Violations()
	kinds := map[string]int{}
	for _, v := range vs {
		kinds[v.Kind]++
	}
	if kinds["multi-question"] != 1 || kinds["ungrounded"] != 1 || kinds["repeat"] != 1 {
		t.Fatalf("数出来的犯规不对：%+v", kinds)
	}
	// 第一轮完全合规，不该出现在列表里。
	for _, v := range vs {
		if v.Turn == 1 {
			t.Fatalf("合规的一轮被记了犯规：%+v", v)
		}
	}
}
