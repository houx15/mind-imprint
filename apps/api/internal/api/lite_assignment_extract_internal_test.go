package api

import "testing"

func TestNormalizeExtraction(t *testing.T) {
	cases := []struct {
		raw    string
		prompt string
		words  int // 0 = nil
		lang   string
		ok     bool
	}{
		{`{"prompt":"写一篇关于雨的记叙文","targetWords":800,"lang":"zh"}`, "写一篇关于雨的记叙文", 800, "zh", true},
		{"```json\n{\"prompt\":\"Describe a storm\",\"targetWords\":300,\"lang\":\"en\"}\n```", "Describe a storm", 300, "en", true},
		{`{"prompt":"写雨","targetWords":20,"lang":"zh"}`, "写雨", 50, "zh", true},
		{`{"prompt":"写雨","targetWords":0,"lang":"zh"}`, "写雨", 0, "zh", true},
		{`{"prompt":"写雨","targetWords":-5,"lang":"zh"}`, "写雨", 0, "zh", true},
		{`{"prompt":"写雨","targetWords":999999,"lang":"zh"}`, "写雨", 10000, "zh", true},
		{`{"prompt":"写雨","targetWords":null,"lang":"fr"}`, "写雨", 0, "zh", true},
		{`{"prompt":"Describe a storm","lang":"fr"}`, "Describe a storm", 0, "en", true},
		{`{"prompt":"","targetWords":800,"lang":"zh"}`, "", 0, "", false},
		{`not json`, "", 0, "", false},
	}
	for _, c := range cases {
		p, w, l, ok := normalizeExtraction(c.raw)
		gotWords := 0
		if w != nil {
			gotWords = *w
		}
		if ok != c.ok || (ok && (p != c.prompt || gotWords != c.words || l != c.lang)) {
			t.Errorf("%q → (%q,%d,%q,%v)", c.raw, p, gotWords, l, ok)
		}
	}
}
