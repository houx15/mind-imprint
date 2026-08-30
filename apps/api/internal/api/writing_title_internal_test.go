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

func TestValidateTitleIdeas(t *testing.T) {
	current := "我想写中国是不是真的让地球更可持续了"

	got := validateTitleIdeas([]string{
		"《转弯中的国家》",              // 书名号 stripped — packaging, not the name
		"  转弯中的国家  ",            // same name after trimming: a duplicate
		"“看方向盘，不是看车道”",         // curly quotes stripped
		"",                      // empty
		"   ",                   // whitespace only
		current,                 // the placeholder she is replacing
		strings.Repeat("很长的标题", 20), // way over the cap
		"总量第一，人均第五十",
		"中国真的让地球变绿了吗",
		"第五个候选应该被砍掉",
	}, current)

	want := []string{"转弯中的国家", "看方向盘，不是看车道", "总量第一，人均第五十", "中国真的让地球变绿了吗"}
	if len(got) != len(want) {
		t.Fatalf("validateTitleIdeas returned %d ideas (%q), want %d", len(got), got, len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("idea %d = %q, want %q (all: %q)", i, got[i], want[i], got)
		}
	}
}

func TestValidateTitleIdeasDropsOverlongRatherThanTruncating(t *testing.T) {
	// Cutting a too-long title mid-clause would produce something worse than
	// the placeholder it replaces, so it is dropped outright.
	long := string([]rune(strings.Repeat("字", suggestedTitleRuneCap+1)))
	got := validateTitleIdeas([]string{long, "好名字"}, "旧标题")
	if len(got) != 1 || got[0] != "好名字" {
		t.Fatalf("validateTitleIdeas = %q, want only the short one", got)
	}
}

func TestParseWritingTitleIdeas(t *testing.T) {
	// Fenced JSON is the shape every model in this package actually returns;
	// extractWritingJSONObject is shared for exactly this reason.
	got, ok := parseWritingTitleIdeas("```json\n{\"ideas\":[\"转弯中的国家\",\"总量与人均\"]}\n```")
	if !ok {
		t.Fatal("parseWritingTitleIdeas: not ok on fenced JSON")
	}
	if len(got.Ideas) != 2 || got.Ideas[0] != "转弯中的国家" {
		t.Fatalf("ideas = %q", got.Ideas)
	}

	if _, ok := parseWritingTitleIdeas("抱歉，我不知道该叫什么。"); ok {
		t.Fatal("parseWritingTitleIdeas: prose must not parse as ok")
	}
}
