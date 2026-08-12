package agent

import "testing"

func TestSanitizeNarrate(t *testing.T) {
	cases := []struct{ in, want string }{
		// the real leak: a snippet id copied into prose, bracketed, mid-sentence.
		{
			in:   "你现在想试着在左侧调出你的暂定论点片段[3ffbb6bb-8e30-482e-82e3-630d9783453d]，动手把框架填进去吗？",
			want: "你现在想试着在左侧调出你的暂定论点片段，动手把框架填进去吗？",
		},
		// a bare (un-bracketed) uuid is still a leak.
		{
			in:   "参考 076139ab-59c1-46ec-a59e-e8c91478979a 这条。",
			want: "参考这条。",
		},
		// id-free narrate is untouched.
		{
			in:   "先跟我说说你打算怎么开头？",
			want: "先跟我说说你打算怎么开头？",
		},
		// idempotent: sanitizing already-clean text is a no-op.
		{
			in:   "你左边那条暂定论点，动手改改看。",
			want: "你左边那条暂定论点，动手改改看。",
		},
	}
	for _, c := range cases {
		if got := SanitizeNarrate(c.in); got != c.want {
			t.Errorf("SanitizeNarrate(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
