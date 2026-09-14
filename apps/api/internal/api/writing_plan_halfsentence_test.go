package api

import (
	"strings"
	"testing"
)

// 救援不能把**半句话**交给她。
//
// 🚨 这一条记的是我自己加的救援造成的回归，而且它比它取代的那个错误更糟 ——
// 因为它不出声。2026-09-12 第十五轮线上走查，英文那个学生连着六步说：
//
//	「印记的话没说完就断了，停在『后面』」
//	「它说李浩然那个例子后面就没字了，不知道它到底想让我先标句子还是接着聊」
//	「印记的话又没说完就断了，看着难受」
//
// 查库：那两条回复一条 68 字停在「后面」，一条 43 字停在「一个是李浩然」，
// 而日志里**一条 unparseable 都没有**。原因是模型在 JSON 字符串里写了一个
// 没转义的引号：
//
//	{"reply":"…一个是李浩然"同学那件事"，还有…","add":[…]}
//
// 整份 Unmarshal 当然失败，于是走到救援；救援把 reply 解到**第一个没转义的
// 引号**为止，得到一个语法合法、语义半截的字符串，然后当成成功交了出去。
// 一次错误弹窗变成了一句永远说不完的话，而日志上什么都没有。
//
// 所以救出来的那句话要过一道「它像不像说完了」：句末得有个终止标点。
// 判错的方向是对的 —— 宁可报错让她重来（她的话还在输入框里），
// 也不要把半句话当成印记说的话摆给她看。
func TestSalvageWritingPlanReply_RefusesAHalfSentence(t *testing.T) {
	// 模型在 reply 里写了没转义的引号：解码会停在那儿。
	broken := `{"reply":"这三条都很扎实：一个是收餐台那二十分钟的亲眼观察，一个是李浩然"同学那件事"，还有你自己算的账。","add":[]}`

	got, ok := parseWritingPlanReply(broken)
	if ok {
		t.Fatalf("半句话被当成印记说的话交出去了：%q", got.Reply)
	}
}

// 反过来：真的到齐了的那一份，救援照旧要救回来 —— 这是它存在的理由，
// 不能为了挡半句话把它整个废掉。
func TestSalvageWritingPlanReply_StillRescuesAWholeSentence(t *testing.T) {
	cases := []string{
		// 结构那一半断了，reply 本身是完整的一句话。
		`{"reply":"你说的是上周五那六个桶。把那个数写进去，比「很多」有力得多。","add":[{"text":"食堂`,
		// 该收 ] 的地方收了 }。
		`{"reply":"先把中心论点定下来。","add":[{"text":"校车该不该装安全带","role":"claim"}}]}`,
		// 问号收尾也算说完了。
		`{"reply":"那天你见到的是什么？","add":[{"text":"半截`,
	}
	for _, c := range cases {
		got, ok := parseWritingPlanReply(c)
		if !ok {
			t.Errorf("这一份的 reply 是完整的，应该救回来：%s", c)
			continue
		}
		if strings.TrimSpace(got.Reply) == "" {
			t.Errorf("救回来了却是空的：%s", c)
		}
	}
}

// 完好的那一份不受影响。
func TestSalvageWritingPlanReply_HealthyRepliesUnaffected(t *testing.T) {
	// 结尾是中文引号、后面没有标点 —— 这是真实回复里常见的收尾，不能误杀。
	got, ok := parseWritingPlanReply(`{"reply":"你想让读者记住的是那一句「浪费不是个别现象」","add":[]}`)
	if !ok {
		t.Fatal("一句以引号收尾的完好回复被当成半句话丢掉了")
	}
	if !strings.Contains(got.Reply, "浪费不是个别现象") {
		t.Fatalf("got %q", got.Reply)
	}
}
