package api

import (
	"strings"
	"testing"
)

// 陪练能不能看见她写的字。
//
// 这一条是从线上量出来的，不是想出来的：2026-09-11 的走查里同一个学生在
// 第 35、38、43 步报了同一件事 ——「印记说缺例子，但我框里明明已经写了
// 张伟那句」。她是对的，例子在第 300 个字之后，而当时的上限就是 300。

func TestWritingProjectionSnippet_ShortTextUntouched(t *testing.T) {
	short := "上周五我数了一下，食堂门口有六个泔水桶是满的。"
	if got := writingProjectionSnippet(short); got != short {
		t.Fatalf("没超上限的段落不该动它：\ngot  %q\nwant %q", got, short)
	}
}

// 她写到一千字左右的一段，必须**整段**进上下文 —— 这正是原来那个 300 切掉的。
func TestWritingProjectionSnippet_AThousandCharacterParagraphSurvivesWhole(t *testing.T) {
	para := strings.Repeat("食", 900) + "张伟说他每天都倒掉半份饭。"
	got := writingProjectionSnippet(para)
	if !strings.Contains(got, "张伟") {
		t.Fatal("段落末尾那个例子被切掉了 —— 陪练会据此说她缺例子，而她写了")
	}
	if got != para {
		t.Error("没到上限就不该加任何说明")
	}
}

// 真的超了上限，就要**说出来**。默不作声地切一刀，读的人会把「被切掉」
// 当成「她没写」，然后去要一件她已经写过的东西。
func TestWritingProjectionSnippet_SaysSoWhenItReallyTruncates(t *testing.T) {
	long := strings.Repeat("字", writingProjectionSnippetRunes+400)
	got := writingProjectionSnippet(long)

	if len([]rune(got)) <= writingProjectionSnippetRunes {
		t.Fatal("截断之后什么都没说")
	}
	for _, want := range []string{"一共", "不能据此判断学生未写相关内容"} {
		if !strings.Contains(got, want) {
			t.Errorf("截断说明里少了 %q：\n%s", want, got[len(got)-300:])
		}
	}
	// 正文那部分正好切在上限上，不多不少。
	if !strings.HasPrefix(got, strings.Repeat("字", writingProjectionSnippetRunes)) {
		t.Error("截断的位置不对")
	}
}
