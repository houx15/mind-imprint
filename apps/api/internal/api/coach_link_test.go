package api

import "testing"

// coach_link_test.go — pure unit tests for the URL extractor behind the
// phase-agnostic link-in-coach bridge. No DB: extractFirstURL is a string
// function. (detectLinkOffer's dedup-against-library path is covered by the
// integration tests that exercise POST /coach with a live store.)

func TestExtractFirstURL(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"none", "我想研究中国的可持续发展", ""},
		{"plain https", "https://nature.com/articles/x", "https://nature.com/articles/x"},
		{"plain http", "看 http://ex.org/a", "http://ex.org/a"},
		{"trailing CJK period", "看看这篇 https://nature.com/x。", "https://nature.com/x"},
		{"trailing ascii period", "source: https://ex.org/a.", "https://ex.org/a"},
		{"wrapped in parens", "（来自 https://ex.org/paper）后面还有话", "https://ex.org/paper"},
		{"first of many", "https://a.org/1 和 https://b.org/2", "https://a.org/1"},
		{"mid sentence with query", "这个 https://ex.org/s?q=1&p=2 有用吗", "https://ex.org/s?q=1&p=2"},
		{"trailing comma", "https://ex.org/a, 然后", "https://ex.org/a"},
		{"not a url just text", "www.example.com 没有协议头", ""},
		// A path that legitimately ends in a paren (Wikipedia disambiguation) must
		// keep it — the balanced closer belongs to the URL, not the sentence.
		{"balanced wiki paren kept", "见 https://en.wikipedia.org/wiki/Water_(disambiguation) 这里", "https://en.wikipedia.org/wiki/Water_(disambiguation)"},
		{"balanced wiki paren then CJK period", "见 https://en.wikipedia.org/wiki/Water_(disambiguation)。", "https://en.wikipedia.org/wiki/Water_(disambiguation)"},
		// A URL wrapped in ASCII parens in prose loses only the wrapper.
		{"ascii wrapper paren stripped", "(https://ex.org/paper)", "https://ex.org/paper"},
		{"unbalanced bracket stripped", "https://ex.org/a] 后面", "https://ex.org/a"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := extractFirstURL(c.in); got != c.want {
				t.Fatalf("extractFirstURL(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestNormalizeURL(t *testing.T) {
	if got := normalizeURL("  https://ex.org/a。 "); got != "https://ex.org/a" {
		t.Fatalf("normalizeURL trailing = %q", got)
	}
	if got := normalizeURL("https://ex.org/a"); got != "https://ex.org/a" {
		t.Fatalf("normalizeURL clean = %q", got)
	}
}
