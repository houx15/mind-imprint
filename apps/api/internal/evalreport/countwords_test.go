package evalreport

import "testing"

func TestCountWords(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want int
	}{
		{"empty", "", 0},
		{"whitespace only", "   \n\t ", 0},
		{"two english words", "hello world", 2},
		{"leading/trailing/multi space english", "  the  quick brown ", 3},
		{"pure chinese six chars", "中国最可持续吗", 7}, // 中国最可持续吗 = 7 Han chars
		{"chinese five chars", "我爱北京城", 5},
		// mixed: 2 Han ("中国") + 2 tokens ("is","green")
		{"mixed zh en", "中国 is green", 4},
		// glued CJK+Latin: 2 Han ("中国") + 1 token ("GDP")
		{"glued cjk latin", "中国GDP", 3},
		// CJK punctuation must NOT count as words; only the 5 Han chars do.
		{"cjk punctuation excluded", "中国，可持续。", 5}, // 中国可持续 = 5 Han, full-width punctuation excluded
		{"digits are a token", "研究 2026 年", 4},       // 研究(2) + 2026(1) + 年(1) = 4
		{"english punctuation glued", "hello, world!", 2}, // "hello," + "world!" = 2
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := CountWords(tc.in); got != tc.want {
				t.Errorf("CountWords(%q) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}
