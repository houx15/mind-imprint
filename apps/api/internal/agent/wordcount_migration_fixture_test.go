package agent

import "testing"

// The 0153 migration backfills writing_version.word_count with a SQL regular
// expression. These are the strings store/migrate_0153_test.go feeds it; if
// CountWords changes, both numbers must be revisited together.
func TestCountWordsMatchesMigration0153Fixtures(t *testing.T) {
	cases := map[string]int{
		"我读了 NASA 的报告，数据是 2024 年的。": 15,
		"It rained all day.\n\n雨停了。":       8,
	}
	for s, want := range cases {
		if got := CountWords(s); got != want {
			t.Errorf("CountWords(%q) = %d, want %d", s, got, want)
		}
	}
}
