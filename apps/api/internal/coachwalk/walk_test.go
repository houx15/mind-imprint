package coachwalk

import (
	"testing"
)

// 合同失败与文本观察必须分开测试。文本信号可以帮助人找到样本，但不能再把一个
// 已经走通的陪练直接判成产品失败。

func TestQuestionMarkCountIsMechanicalAndNonSemantic(t *testing.T) {
	for _, c := range []struct {
		in   string
		want int
	}{
		{"你打算怎么量？", 1},
		{"你打算怎么量?", 1},
		{"称什么？在哪称？", 2},
		{"示范：谁统计的？什么口径？这些问题可能由 AI 自己回答。", 2},
		{"先说说你的想法。", 0},
		{"", 0},
	} {
		if got := QuestionMarkCount(c.in); got != c.want {
			t.Errorf("QuestionMarkCount(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestEchoesLastNeedsFourRunesOfHerOwnWords(t *testing.T) {
	hers := "我这周已经称了三天了，平均每天大概 47 公斤剩饭。"
	if !EchoesLast("你说你称了三天了，那第四天呢？", hers) {
		t.Error("接住了她的原话，却判成接不住")
	}
	if EchoesLast("很好，继续保持，你做得很棒！", hers) {
		t.Error("一句空洞鼓励，却判成接住了")
	}
}

func TestEchoesLastDoesNotPunishVeryShortReplies(t *testing.T) {
	// 她只说了「嗯」这种极短的话时，任何回复都可能偶然命中或偶然不命中。
	// 这种轮次不判——宁可漏掉一次，也不要凭巧合记一条犯规。
	if !EchoesLast("那我们换个角度看。", "嗯") {
		t.Error("她的话短于四个字时不应该判犯规")
	}
}

// 🚨 走查台自己的回归测试。判据的第一版只折空白，于是印记把她的原话加了一对
// 引号再说回来时，被判成「接不住她」——一条读起来和真缺陷一模一样的假缺陷。
func TestEchoesLastIgnoresPunctuationTheCoachAdds(t *testing.T) {
	hers := "我咋知道他们是因为打多了还是不好吃才剩的？"
	if !EchoesLast("先别急着分「打多了」还是「不好吃」——那个在收餐台看不出来。", hers) {
		t.Error("印记引用了她的原话，只是加了引号，不该判成接不住")
	}
	if !EchoesLast("你说的 47 公斤，是怎么称出来的？", "平均每天大概47公斤剩饭") {
		t.Error("数字中间被插了空格，不该判成接不住")
	}
}

func TestRepeatOfCatchesRewordedRepeatAndSparesNewQuestions(t *testing.T) {
	prev := []Turn{
		{N: 1, Reply: "你打算在什么时间点称剩饭？是午饭刚结束，还是等大家都走了？"},
		{N: 2, Reply: "那三天里最多的一天和最少的一天，差了多少？"},
	}
	// 换了说法的同一个问题，对学生来说是同一个问题又被问了一遍。实测落在 0.58。
	if got := repeatOf("你准备什么时间点去称剩饭？午饭刚结束，还是等大家都走了？", prev); got != 1 {
		t.Errorf("换了说法的重复没被抓到：got %d, want 1", got)
	}
	// 一个真正的新问题实测落在 0.00，不能被误判成重复，否则报告会凭空多出一堆犯规。
	if got := repeatOf("食堂师傅怎么看这件事？", prev); got != 0 {
		t.Errorf("新问题被误判成第 %d 轮的重复", got)
	}
	if got := repeatOf("", prev); got != 0 {
		t.Errorf("空回复不该算重复：got %d", got)
	}
}

func TestRepeatOfSkipsTurnsThatFailedToParse(t *testing.T) {
	// 解析失败那一轮没有 reply，拿空字符串去比会把任何一句都判成重复。
	prev := []Turn{{N: 1, ParseErr: "pbl: unexpected end of JSON input"}}
	if got := repeatOf("你打算怎么量？", prev); got != 0 {
		t.Errorf("不该和解析失败的那一轮比：got %d", got)
	}
}

func TestViolationsReportsParseFailureAloneAndStopsThere(t *testing.T) {
	// 一轮解析失败时，别再给同一轮叠上「接不住她」——那一轮根本没有 reply，
	// 叠出来的两条会让一次故障在总表里看起来像两处缺陷。
	l := &Log{Turns: []Turn{{N: 1, ParseErr: "pbl: no reply"}}}
	vs := l.Violations()
	if len(vs) != 1 || vs[0].Kind != "parse" {
		t.Fatalf("解析失败那一轮应当只记一条 parse：%+v", vs)
	}
}

func TestContractViolationsAndLanguageObservationsStaySeparate(t *testing.T) {
	l := &Log{Turns: []Turn{
		{N: 1, Reply: "你打算怎么量？", QuestionMarks: 1, EchoesHer: true},
		{N: 2, Reply: "称什么？在哪称？", QuestionMarks: 2, EchoesHer: true},
		{N: 3, Reply: "很好，继续努力。", QuestionMarks: 0, EchoesHer: false},
		{N: 4, Reply: "你打算怎么量？", QuestionMarks: 1, EchoesHer: true, RepeatOf: 1},
		{N: 5, Reply: "我帮你写一句。", QuestionMarks: 0, EchoesHer: true,
			Extra: []Violation{{Turn: 5, Kind: "banned-phrasing", Note: "替她写了"}}},
	}}
	hard := map[string]int{}
	for _, v := range l.Violations() {
		hard[v.Kind]++
		if v.Turn == 1 {
			t.Fatalf("合规的一轮被记了犯规：%+v", v)
		}
	}
	if hard["banned-phrasing"] != 1 || len(hard) != 1 {
		t.Fatalf("only the production contract failure should block: %+v", hard)
	}
	observed := map[string]int{}
	for _, observation := range l.Observations() {
		observed[observation.Kind]++
	}
	for _, want := range []string{"question-marks", "ungrounded", "repeat"} {
		if observed[want] != 1 {
			t.Errorf("%s 应当观察到 1 条，实际 %d（全部：%+v）", want, observed[want], observed)
		}
	}
	if l.Count("question-marks") != 0 || l.Count("banned-phrasing") != 1 || l.CountObservation("question-marks") != 1 {
		t.Errorf("blocking and observation counts are mixed")
	}
}

func TestLanguageSignalDoesNotBecomeAContractFailure(t *testing.T) {
	l := &Log{Turns: []Turn{{N: 1,
		Reply:         "信源透镜会检查：谁统计的？口径是什么？这些信息原文都没有。",
		QuestionMarks: 2, EchoesHer: true,
	}}}
	if got := l.Violations(); len(got) != 0 {
		t.Fatalf("a semantic question-count signal blocked the contract: %+v", got)
	}
	if got := l.CountObservation("question-marks"); got != 1 {
		t.Fatalf("diagnostic signal disappeared: got %d", got)
	}
}
