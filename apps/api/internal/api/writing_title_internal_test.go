package api

import (
	"strings"
	"testing"

	"mindimprint/api/internal/store/sqlc"
)

// The three pure decisions in writing_title.go. All of them are the kind you
// cannot see going wrong by reading a screen: a wrong `titleIsStillTheIdea`
// either pesters a student who already named her piece, or silently switches
// the whole feature off and leaves the placeholder on every report.

func TestTitleIsStillTheIdea(t *testing.T) {
	longIdea := strings.Repeat("我想写中国的可持续发展", 40) // > 200 runes
	truncated := string([]rune(longIdea)[:titleRuneCap])

	cases := []struct {
		name  string
		title string
		idea  string
		want  bool
	}{
		{
			name:  "untouched title equals the idea verbatim",
			title: "我想写中国是不是真的让地球更可持续了",
			idea:  "我想写中国是不是真的让地球更可持续了",
			want:  true,
		},
		{
			// createWriting stores only the first 200 runes as the title, so a
			// long idea's title is the truncation — not the whole sentence.
			// Comparing against the untruncated original would report every
			// long-idea writing as "already renamed" and never ask.
			name:  "a long idea matches its own truncation",
			title: truncated,
			idea:  longIdea,
			want:  true,
		},
		{
			name:  "she renamed it",
			title: "转弯中的国家",
			idea:  "我想写中国是不是真的让地球更可持续了",
			want:  false,
		},
		{
			// The placeholder is a prefix of nothing and a suffix of nothing —
			// only exact equality counts, or renaming to something that merely
			// starts the same way would be missed.
			name:  "a title that only starts like the idea is hers",
			title: "我想写中国",
			idea:  "我想写中国是不是真的让地球更可持续了",
			want:  false,
		},
		{
			name:  "surrounding whitespace alone is not a rename",
			title: "  我想写中国  ",
			idea:  "我想写中国",
			want:  true,
		},
		{
			// No opening turn to compare against should never be read as
			// "unnamed" — we do not interrupt her over a row we cannot account
			// for.
			name:  "no idea recorded",
			title: "转弯中的国家",
			idea:  "",
			want:  false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := titleIsStillTheIdea(tc.title, tc.idea); got != tc.want {
				t.Fatalf("titleIsStillTheIdea(%q, %q) = %v, want %v", tc.title, tc.idea, got, tc.want)
			}
		})
	}
}

func TestFirstStudentMessage(t *testing.T) {
	// createWriting writes the idea as seq=1 role='student' in the same
	// transaction as the writing row, so it is always first — but 印记's
	// opening turn lands right after it, and picking THAT up would feed the
	// model its own words as "what she wanted to write".
	msgs := []sqlc.AtomMessage{
		{Role: "student", Content: "我想写中国是不是真的让地球更可持续了"},
		{Role: "ai", Content: "这个问题很好，我们先看看你手上有什么材料。"},
		{Role: "student", Content: "我找到了一篇 Nature 的论文。"},
	}
	if got := firstStudentMessage(msgs); got != "我想写中国是不是真的让地球更可持续了" {
		t.Fatalf("firstStudentMessage = %q", got)
	}

	// An AI-first transcript still finds her turn rather than returning 印记's.
	if got := firstStudentMessage([]sqlc.AtomMessage{
		{Role: "ai", Content: "先说说你想写什么？"},
		{Role: "student", Content: "我想写校服该不该取消"},
	}); got != "我想写校服该不该取消" {
		t.Fatalf("firstStudentMessage (ai first) = %q", got)
	}

	if got := firstStudentMessage(nil); got != "" {
		t.Fatalf("firstStudentMessage(nil) = %q, want empty", got)
	}
}

// 2026-09-18：「不能直接给取名字」—— 关键词必须逐字出现在她的正文里。
// 模型拼出来的一句（她正文里没有）就是一个标题，要丢掉。
func TestValidateTitleKeywords(t *testing.T) {
	draft := "同样的练习，因为目标和做法发生变化，也带给了我不同的感受。跳绳仍然让我出汗、喘气，但那些疲惫不再占据我全部的注意力。"
	current := "作者说苦乐全在主观的心"

	got := validateTitleKeywords([]string{
		"「跳绳」",        // 包装去掉，逐字在正文里
		"跳绳",          // 重复
		"苦乐在心不在事",     // 正文里没有 —— 这是一个标题
		"同样的跳绳，不同的滋味", // 拼出来的标题
		"疲惫",
		"注意力",
		"我",                     // 太短
		strings.Repeat("跳", 13), // 太长
		current,                 // 占位标题
		"目标和做法",
		"出汗",
		"不同的感受",
		"第七个应该被砍掉",
	}, draft, current)

	want := []string{"跳绳", "疲惫", "注意力", "目标和做法", "出汗", "不同的感受"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("keywords = %q, want %q", got, want)
	}
}

func TestParseWritingTitleKeywords(t *testing.T) {
	got, ok := parseWritingTitleKeywords("```json\n{\"keywords\":[\"跳绳\",\"疲惫\"]}\n```")
	if !ok || len(got.Keywords) != 2 || got.Keywords[0] != "跳绳" {
		t.Fatalf("parse = %+v, %v", got, ok)
	}
	if _, ok := parseWritingTitleKeywords("抱歉，我不知道。"); ok {
		t.Fatal("prose must not parse as ok")
	}
}
