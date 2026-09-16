package pbl

import "testing"

func TestOriginalQuoteRestoresSourcePunctuationAndWhitespace(t *testing.T) {
	source := "提示：\n同伴问我。\n我发现“完全禁止”没有考虑这个情况。这是虚构演练。\n其他段落"
	candidate := `同伴问我。 我发现"完全禁止"没有考虑这个情况。这是虚构演练。`
	got, ok := originalQuote(source, candidate)
	if !ok || got != "同伴问我。\n我发现“完全禁止”没有考虑这个情况。这是虚构演练。" {
		t.Fatalf("%q %v", got, ok)
	}
	draft, dropped := GroundSiteDraft(SiteDraft{Sections: []SiteSection{{Key: "a", Body: candidate}}}, source)
	if len(dropped) != 0 || len(draft.Sections) != 1 || draft.Sections[0].Body != got {
		t.Fatalf("%+v %v", draft, dropped)
	}
	withoutQuotes := "同伴问我。 我发现完全禁止没有考虑这个情况。这是虚构演练。"
	if restored, ok := originalQuote(source, withoutQuotes); !ok || restored != got {
		t.Fatalf("quote omission not restored: %q", restored)
	}
}
func TestOriginalQuoteRejectsChangedWordsAndAmbiguousSpans(t *testing.T) {
	for _, tc := range []struct{ source, candidate string }{
		{`我说“这不是真实经历”。`, `我说"这是真实经历"。`},
		{`我说“原话”。另一个段落。`, `我说"原话"。...另一个段落。`},
		{`我说“原话”。我说‘原话’。`, `我说"原话"。`},
		{`我说“原话”。`, ""},
	} {
		if got, ok := originalQuote(tc.source, tc.candidate); ok {
			t.Fatalf("accepted %q", got)
		}
	}
}
