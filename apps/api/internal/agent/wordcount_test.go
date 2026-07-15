package agent

import "testing"

func TestCountWords(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want int
	}{
		{"empty", "", 0},
		{"whitespace only", "   \n\t ", 0},
		{"latin words", "the quick brown fox", 4},
		{"cjk chars each count", "中国是否让地球更可持续", 11},
		{"mixed cjk and latin", "中国的 GDP 增长", 6}, // 中 国 的 (3) + GDP (1) + 增 长 (2) = 6
		{"latin with punctuation", "hello, world!", 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := CountWords(c.in); got != c.want {
				t.Errorf("CountWords(%q) = %d, want %d", c.in, got, c.want)
			}
		})
	}
}
