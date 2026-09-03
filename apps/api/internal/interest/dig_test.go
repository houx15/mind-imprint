package interest

import (
	"strings"
	"testing"
)

// dig_test —— 「继续深挖」里读代码看不出对错的那几条。

func TestBuildDigPromptCentresOnHerOwnWords(t *testing.T) {
	// 🚨 她的原话是这次调用**唯一真正重要的输入**。少了它们，模型只能围着一个
	// 词泛泛地想 —— 而那正是原型那四个空动词的来源。
	ev := []string{"我读到面积那一段才反应过来，四平方公里其实很小。", "一个避难所不是一个计划。"}
	system, user := BuildDigPrompt("样本代表性", "你现在会先问这个例子能代表多少。", ev)

	if !strings.Contains(system, "think") || !strings.Contains(system, "make") {
		t.Error("system prompt 没有说清四种种子")
	}
	for _, e := range ev {
		if !strings.Contains(user, e) {
			t.Errorf("她的原话没有进 prompt：%q", e)
		}
	}
	if !strings.Contains(user, "样本代表性") {
		t.Error("关键词没有进 prompt")
	}
}

func TestBuildDigPromptSaysSoWhenThereAreNoWords(t *testing.T) {
	// 不该发生（evidence 是 NOT NULL），但如果发生了，说实话比编一句强。
	_, user := BuildDigPrompt("样本代表性", "", nil)
	if !strings.Contains(user, "没有留下原话") {
		t.Errorf("没有原话时应该说出来：\n%s", user)
	}
}

func TestParseDigReplyKeepsOnePerKind(t *testing.T) {
	raw := "```json\n" + `{"seeds":[
	  {"kind":"think","text":"四平方公里凭什么代表一整片海？","why":"你自己写过面积那一段。"},
	  {"kind":"think","text":"重复的一颗","why":"x"},
	  {"kind":"read","text":"珊瑚白化到底是死了还是把藻吐出去了","why":"y"},
	  {"kind":"write","text":"一个避难所不是一个计划","why":"你写过这句。"},
	  {"kind":"make","text":"用一支五十块的温度计记录一个月水温","why":"z"}
	]}` + "\n```"
	got, err := ParseDigReply(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(got) != DigSeedCount {
		t.Fatalf("解析出 %d 颗，want %d：%+v", len(got), DigSeedCount, got)
	}
	seen := map[DigKind]bool{}
	for _, s := range got {
		if seen[s.Kind] {
			t.Errorf("同一种出现了两次：%s", s.Kind)
		}
		seen[s.Kind] = true
	}
}

func TestParseDigReplyDropsUnknownKindAndEmptyText(t *testing.T) {
	// text 会被当作标题直接送进创建接口 —— 空的会建出一个无名的房间。
	raw := `{"seeds":[
	  {"kind":"vibes","text":"随便想想","why":""},
	  {"kind":"read","text":"   ","why":""},
	  {"kind":"write","text":"一个避难所不是一个计划","why":""}
	]}`
	got, err := ParseDigReply(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(got) != 1 || got[0].Kind != DigWrite {
		t.Fatalf("应该只留下 write 那颗：%+v", got)
	}
}

// 解析失败就没有种子。**绝不摆四个通用动词顶上** —— 那正是这套东西要取代的
// 东西（memory: ai-errors-must-surface-never-fake）。
func TestParseDigReplyErrorsInsteadOfFallingBackToGenericVerbs(t *testing.T) {
	for _, raw := range []string{"", "抱歉，我无法完成。", "{not json", `{"seeds":[]}`} {
		if _, err := ParseDigReply(raw); err == nil {
			t.Errorf("垃圾输入 %q 没有报错", raw)
		}
	}
}

func TestIsDigKind(t *testing.T) {
	for _, ok := range []string{"think", "read", "write", "make"} {
		if !IsDigKind(ok) {
			t.Errorf("%q 应该是一种种子", ok)
		}
	}
	for _, bad := range []string{"", "Think", "ask", "project"} {
		if IsDigKind(bad) {
			t.Errorf("%q 不该被当成种子类型", bad)
		}
	}
}
