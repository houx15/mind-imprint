package enforcement

import "testing"

func TestLexicalSimilarity_Cosine(t *testing.T) {
	sim := LexicalSimilarity{}

	if got := sim.Cosine("", "任何文本"); got != 0 {
		t.Fatalf("empty vs non-empty: want 0, got %v", got)
	}
	if got := sim.Cosine("单", "字"); got != 0 {
		t.Fatalf("single-rune inputs (no bigrams): want 0, got %v", got)
	}
	if got := sim.Cosine("完全相同的句子", "完全相同的句子"); got != 1 {
		t.Fatalf("identical strings: want 1, got %v", got)
	}

	overlapping := sim.Cosine("中国的治理决心正在增强", "中国的治理决心尚未解决存量问题")
	unrelated := sim.Cosine("中国的治理决心正在增强", "今天天气很好我想去公园散步")
	if overlapping <= unrelated {
		t.Fatalf("want shared-substring pair to score higher than an unrelated pair: overlapping=%v unrelated=%v", overlapping, unrelated)
	}
	if unrelated > 0.15 {
		t.Fatalf("want unrelated pair to score low, got %v", unrelated)
	}

	// Whitespace/case differences must not move the score (both compact away).
	if got, want := sim.Cosine("Hello World", "helloworld"), sim.Cosine("helloworld", "helloworld"); got != want {
		t.Fatalf("whitespace/case should not affect score: got %v, want %v", got, want)
	}
}
