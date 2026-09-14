package api

// reading_coach_rawnewline_test.go —— 字符串值里没转义的换行，不能让她这一轮收不到回复。
//
// 形状来自 2026-09-14 的 coachwalk 走查（deepseek/deepseek-flash，阅读室）：模型在
// reply 里分了两段，换行直接写进了 JSON 字符串。正文缩短并换了内容，换行的位置
// 和字段顺序照原样。

import (
	"strings"
	"testing"
)

func TestCoachReplySurvivesRawNewlinesInsideTheReplyString(t *testing.T) {
	raw := "{\"reply\":\"第 6 段最后一句值得细读。\n\n先看它说的是能力还是结果。\"," +
		"\"advance\":\"\",\"focusBlock\":\"b6\",\"tool\":\"\",\"lens\":\"\",\"card\":null}"
	blocks := SplitBlocks(benchReadingArticle)
	got, ok := parseReadingCoachReply(raw, blocks, "zh", func(string) bool { return true })
	if !ok {
		t.Fatal("a complete reply was thrown away because of a line break — production answers this turn with a 502")
	}
	// 模型想要的那个分段要留成真的换行，而不是被抹掉或变成字面的 \n。
	if got.Reply != "第 6 段最后一句值得细读。\n\n先看它说的是能力还是结果。" {
		t.Errorf("reply = %q", got.Reply)
	}
	if got.FocusBlock != "b6" {
		t.Errorf("fields after the reply were lost: focusBlock = %q", got.FocusBlock)
	}
}

// 断在半路 + 字符串里有换行：两种毛病叠在一起时，救援那条路也得接得住。
func TestCoachReplySalvageSurvivesRawNewlinesToo(t *testing.T) {
	raw := "{\"reply\":\"先读第三段。\n再告诉我你的判断。\",\"advance\":\"\",\"card\":{\"type\":\"short_text\",\"prompt\":\"作者"
	got, ok := parseReadingCoachReply(raw, SplitBlocks(benchReadingArticle), "zh", func(string) bool { return true })
	if !ok {
		t.Fatal("truncated reply with a raw newline was thrown away")
	}
	if got.Reply != "先读第三段。\n再告诉我你的判断。" {
		t.Errorf("reply = %q", got.Reply)
	}
	if got.Card != nil {
		t.Errorf("kept a card that never finished arriving: %+v", got.Card)
	}
}

func TestEscapeRawControlInStringsOnlyTouchesStringValues(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"raw newline inside a value", "{\"a\":\"x\ny\"}", `{"a":"x\ny"}`},
		// 缩进过的 JSON：键与键之间的换行是合法空白，一个字节都不能改。
		{"newlines between keys stay", "{\n  \"a\": \"x\",\n  \"b\": 1\n}", "{\n  \"a\": \"x\",\n  \"b\": 1\n}"},
		{"already escaped stays", `{"a":"x\ny"}`, `{"a":"x\ny"}`},
		// 转义过的引号不能让它以为字符串结束了，后面的换行仍在字符串里。
		{"escaped quote then raw newline", "{\"a\":\"q\\\"\nz\"}", `{"a":"q\"\nz"}`},
		{"tab and carriage return", "{\"a\":\"x\ty\rz\"}", `{"a":"x\ty\rz"}`},
		{"other control char", "{\"a\":\"x\x01y\"}", "{\"a\":\"x\\u0001y\"}"},
		{"no strings at all", "  \n", "  \n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := escapeRawControlInStrings(tc.in); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// 纯文字的回复（一个左大括号都没有）走的是原样收下那条路，
// 那里的换行本来就是她该看到的换行，不能被改写成字面的 \n。
func TestProseReplyKeepsItsRealLineBreaks(t *testing.T) {
	raw := "你刚才说\"投入很大\"。\n先看第六段最后一句。"
	got, ok := parseReadingCoachReply(raw, SplitBlocks(benchReadingArticle), "zh", func(string) bool { return true })
	if !ok {
		t.Fatal("prose reply thrown away")
	}
	if !strings.Contains(got.Reply, "\n") || strings.Contains(got.Reply, `\n`) {
		t.Errorf("prose line break was rewritten: %q", got.Reply)
	}
}
