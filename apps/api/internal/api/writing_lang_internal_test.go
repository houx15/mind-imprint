package api

// writing_lang_internal_test.go — the unit on the target length, which is the
// whole reason writing_lang.go exists.
//
// This is a "read the code and you cannot see whether it is right" function
// in the sense AGENTS.md means: the number is stored language-agnostically,
// the word next to it is not, and getting them out of step produced advice a
// human immediately recognised as absurd (印记 telling a 500-WORD essay it
// had room for only one example, because it read 500 as Chinese characters).
// Nothing about the type signature prevents that, so the pairing is pinned
// here.

import (
	"strings"
	"testing"

	"mindimprint/api/internal/store/sqlc"
)

func words(n int32) *int32 { return &n }

func TestWritingLengthLineUsesTheUnitTheStudentWasShown(t *testing.T) {
	// The setup dialog shows "words" for an English piece and 「字」 for a
	// Chinese one (WritingSetupModal.tsx), and stores both in the same
	// target_words column. The prompt has to say back whichever she saw.
	for _, tc := range []struct {
		name    string
		lang    string
		want    string
		notWant string
	}{
		{name: "english counts words", lang: "en", want: "500 词", notWant: "500 字"},
		{name: "chinese counts characters", lang: "zh", want: "500 字", notWant: "500 词"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := writingLengthLine(sqlc.Writing{Lang: tc.lang, TargetWords: words(500)}, "目标篇幅")
			if !strings.Contains(got, tc.want) {
				t.Fatalf("lang=%q: want %q in %q", tc.lang, tc.want, got)
			}
			// 🚨 The regression itself: an English piece described as 「500 字」
			// is a two-paragraph piece, and 印记 advised it accordingly.
			if strings.Contains(got, tc.notWant) {
				t.Fatalf("lang=%q: must not contain %q, got %q", tc.lang, tc.notWant, got)
			}
		})
	}
}

func TestWritingLengthLineSpellsOutTheUnitForEnglish(t *testing.T) {
	// 「约 500 词」 sitting inside an otherwise Chinese prompt is still
	// readable as 字 by habit, and the cost of that misreading is the bug
	// this file was created for. The English branch names the unit a second
	// time, unambiguously, and says out loud that 500 of them is a whole
	// essay.
	got := writingLengthLine(sqlc.Writing{Lang: "en", TargetWords: words(500)}, "目标篇幅")
	if !strings.Contains(got, "English words") {
		t.Fatalf("english length line must name the unit in English: %q", got)
	}
	if !strings.Contains(got, "不是汉字") {
		t.Fatalf("english length line must rule out 汉字 explicitly: %q", got)
	}
}

func TestWritingLengthLineIsEmptyWhenSheNeverSaid(t *testing.T) {
	// 目标字数 may be left blank forever — it is not a gate (writing_stage.go).
	// A prompt that invents a length she never chose would be worse than one
	// that omits it: she would get advice sized for a piece nobody asked for.
	for _, lang := range []string{"zh", "en"} {
		if got := writingLengthLine(sqlc.Writing{Lang: lang}, "目标篇幅"); got != "" {
			t.Fatalf("lang=%q: want empty line for nil TargetWords, got %q", lang, got)
		}
	}
}

func TestWritingLangLineSeparatesCoachingFromContent(t *testing.T) {
	// The distinction is the fix for 「英文的写作，中文的mindmap」: the mind
	// map is the essay's own words and must be English, while 印记 keeps
	// TEACHING in Chinese. A line that only stated the language (which three
	// builders already did) changed no output at all, so both halves have to
	// be present.
	got := writingLangLine(sqlc.Writing{Lang: "en"})
	for _, want := range []string{"英文", "中文", "提纲", "思维导图", "标题"} {
		if !strings.Contains(got, want) {
			t.Fatalf("english lang line must mention %q, got %q", want, got)
		}
	}

	zh := writingLangLine(sqlc.Writing{Lang: "zh"})
	if strings.Contains(zh, "英文") {
		t.Fatalf("chinese lang line must not talk about English: %q", zh)
	}
}
