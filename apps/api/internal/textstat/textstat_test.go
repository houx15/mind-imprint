package textstat

import (
	"reflect"
	"testing"

	"mindimprint/api/internal/agent"
)

func TestSplitSentences_ChineseTerminators(t *testing.T) {
	got := SplitSentences("这是第一句。这是第二句！这是第三句？")
	want := []string{"这是第一句。", "这是第二句！", "这是第三句？"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestSplitSentences_AbbreviationNotABoundary(t *testing.T) {
	got := SplitSentences("Mr. Smith went to Washington. He was tired.")
	want := []string{"Mr. Smith went to Washington.", "He was tired."}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestSplitSentences_DecimalNotABoundary(t *testing.T) {
	got := SplitSentences("The price is $3.14 today. It rose.")
	want := []string{"The price is $3.14 today.", "It rose."}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestSplitSentences_EgAbbreviationMidSentence(t *testing.T) {
	got := SplitSentences("She likes animals, e.g. dogs and cats. Cats are independent.")
	want := []string{"She likes animals, e.g. dogs and cats.", "Cats are independent."}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestSplitSentences_InitialsNotABoundary(t *testing.T) {
	got := SplitSentences("J. K. Rowling wrote this.")
	want := []string{"J. K. Rowling wrote this."}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestSplitSentences_MixedChineseEnglish(t *testing.T) {
	got := SplitSentences("这是一句话。This is English. 这是第二句中文？")
	want := []string{"这是一句话。", "This is English.", "这是第二句中文？"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// 🚨 An ellipsis is not a boundary — neither "..." nor 「……」.
//
// This test used to pin the opposite ("Wait..." / "let me think." as two
// sentences). 2026-09-23 review: 「我不知道……也许吧。」 是一句犹豫，不是两句，
// 而 「……」 是中学生作文里最常见的句中标点 —— 每出现一次就多数出一句，
// MeanSentenceLength 和 ConnectiveDensity 跟着被摊薄，而批改提示词把这两个
// 数字当事实报给模型。
func TestSplitSentences_EllipsisIsNotABoundary(t *testing.T) {
	got := SplitSentences("Wait... let me think.")
	want := []string{"Wait... let me think."}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}

	zh := SplitSentences("我不知道……也许吧。")
	wantZH := []string{"我不知道……也许吧。"}
	if !reflect.DeepEqual(zh, wantZH) {
		t.Fatalf("got %q, want %q", zh, wantZH)
	}

	// A real terminator still splits, and a trailing ellipsis folds into the
	// sentence it follows rather than opening a new one.
	folded := SplitSentences("真的吗？……我不信。")
	wantFolded := []string{"真的吗？……", "我不信。"}
	if !reflect.DeepEqual(folded, wantFolded) {
		t.Fatalf("got %q, want %q", folded, wantFolded)
	}
}

func TestSplitSentences_FoldedQuote(t *testing.T) {

	got2 := SplitSentences("她说：「我们要开始了。」然后就走了。")
	want2 := []string{"她说：「我们要开始了。」", "然后就走了。"}
	if !reflect.DeepEqual(got2, want2) {
		t.Fatalf("got %q, want %q", got2, want2)
	}
}

func TestSplitSentences_Empty(t *testing.T) {
	if got := SplitSentences("   "); got != nil {
		t.Fatalf("got %q, want nil", got)
	}
}

// TestWordTokensParityWithAgentCountWords locks wordTokens (unexported, used
// by TypeTokenRatio) to agent.CountWords, the counter textstat reuses — see
// wordTokens' doc comment. If this ever fails, either wordTokens drifted
// from agent.CountWords, or agent.CountWords itself changed; either way the
// two must be brought back in step before this file's comment is still true.
func TestWordTokensParityWithAgentCountWords(t *testing.T) {
	cases := []string{
		"",
		"hello world",
		"这是中文",
		"中国GDP增长很快",
		"Mixed 中文 and English text.",
		"a, b, c!",
		"多个  空格   之间",
	}
	for _, c := range cases {
		got := len(wordTokens(c))
		want := agent.CountWords(c)
		if got != want {
			t.Errorf("len(wordTokens(%q)) = %d, agent.CountWords = %d", c, got, want)
		}
	}
}

func TestTypeTokenRatio(t *testing.T) {
	if got, want := TypeTokenRatio("cats cats dogs"), 2.0/3.0; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := TypeTokenRatio("猫猫狗"), 2.0/3.0; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	// Attached punctuation folds into the same type as the bare word.
	if got, want := TypeTokenRatio("Cats. cats, CATS!"), 1.0/3.0; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got := TypeTokenRatio(""); got != 0 {
		t.Errorf("got %v, want 0", got)
	}
}

func TestMeanSentenceLength(t *testing.T) {
	text := "I like cats. Dogs are nice too."
	// agent.CountWords("I like cats.") = 3 ("I","like","cats.")
	// agent.CountWords("Dogs are nice too.") = 4
	got := MeanSentenceLength(text)
	want := 3.5
	if got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got := MeanSentenceLength(""); got != 0 {
		t.Errorf("got %v, want 0", got)
	}
}

func TestComplexSentenceRatio(t *testing.T) {
	zh := "虽然天气不好，但是我们还是出去了。今天天气很好。"
	if got, want := ComplexSentenceRatio(zh, "zh"), 0.5; got != want {
		t.Errorf("zh: got %v, want %v", got, want)
	}
	en := "Although it rained, we went out. The weather was nice."
	if got, want := ComplexSentenceRatio(en, "en"), 0.5; got != want {
		t.Errorf("en: got %v, want %v", got, want)
	}
	// Word-boundary matching: "if" must not fire inside "gift".
	noFalseHit := "The gift was nice. We stayed home."
	if got, want := ComplexSentenceRatio(noFalseHit, "en"), 0.0; got != want {
		t.Errorf("gift: got %v, want %v (word boundary false positive)", got, want)
	}
	hit := "If it rains, we stay in. Otherwise we go out."
	if got, want := ComplexSentenceRatio(hit, "en"), 0.5; got != want {
		t.Errorf("if: got %v, want %v", got, want)
	}
	if got := ComplexSentenceRatio("", "en"); got != 0 {
		t.Errorf("got %v, want 0", got)
	}
}

func TestConnectiveDensity(t *testing.T) {
	zh := "因此，我们决定留下来。所以，天气变冷了。"
	if got, want := ConnectiveDensity(zh, "zh"), 1.0; got != want {
		t.Errorf("zh: got %v, want %v", got, want)
	}
	en := "For example, this works. However, that fails."
	if got, want := ConnectiveDensity(en, "en"), 1.0; got != want {
		t.Errorf("en: got %v, want %v", got, want)
	}
	if got := ConnectiveDensity("", "en"); got != 0 {
		t.Errorf("got %v, want 0", got)
	}
}

func TestCompute(t *testing.T) {
	s := Compute("虽然下雨，我们出发了。因此大家都湿透了。", "zh")
	if s.TypeTokenRatio <= 0 || s.TypeTokenRatio > 1 {
		t.Errorf("TypeTokenRatio out of range: %v", s.TypeTokenRatio)
	}
	if s.MeanSentenceLength <= 0 {
		t.Errorf("MeanSentenceLength should be > 0: %v", s.MeanSentenceLength)
	}
	if s.ComplexSentenceRatio != 0.5 {
		t.Errorf("ComplexSentenceRatio: got %v, want 0.5", s.ComplexSentenceRatio)
	}
	if s.ConnectiveDensity != 0.5 {
		t.Errorf("ConnectiveDensity: got %v, want 0.5", s.ConnectiveDensity)
	}
}

// --- 2026-09-23 review: the two measures that were confidently wrong on the
// writing students actually hand in. Each case below is a MEASURED failure
// from that review, quoted verbatim, with the number it used to report.

// 「I like that book. After school I play. Before dinner I read.」 reported
// ComplexSentenceRatio 1.0 — three simple sentences, zero subordinate
// clauses. "that" was a determiner, "after"/"before" prepositions.
func TestComplexSentenceRatio_DeterminersAndPrepositionsAreNotClauses(t *testing.T) {
	if got := ComplexSentenceRatio("I like that book. After school I play. Before dinner I read.", "en"); got != 0 {
		t.Fatalf("no subordinate clause here; got %.2f, want 0", got)
	}
	// 「只有三个人来了。」 — 只有 as "there are only", not the 只有…才 pattern.
	if got := ComplexSentenceRatio("只有三个人来了。", "zh"); got != 0 {
		t.Fatalf("只有三个人 is a quantity, not a clause; got %.2f, want 0", got)
	}
}

// The markers that stayed, and the ones that joined, still fire.
func TestComplexSentenceRatio_RealSubordinatorsStillFire(t *testing.T) {
	if got := ComplexSentenceRatio("Although it rained, I went. I will wait until you come.", "en"); got != 1 {
		t.Fatalf("although / until are subordinators; got %.2f, want 1", got)
	}
	if got := ComplexSentenceRatio("哪怕下雨我也去。吃饭的时候不要看手机。", "zh"); got != 1 {
		t.Fatalf("哪怕 / 的时候 are subordinators; got %.2f, want 1", got)
	}
}

// 「我起床了。然后我吃饭。然后我上学。然后我回家。」 reported
// ConnectiveDensity 0.0 — a textbook 流水账 scoring zero on the one defect
// the symptom table names it for (connector_monotony / flat_chronicle).
func TestConnectiveDensity_CountsWhatAMiddleSchoolerWrites(t *testing.T) {
	got := ConnectiveDensity("我起床了。然后我吃饭。然后我上学。然后我回家。", "zh")
	if got != 0.75 {
		t.Fatalf("three 然后 over four sentences; got %.2f, want 0.75", got)
	}
	en := ConnectiveDensity("I woke up. Then I ate. So I left. But I forgot my bag.", "en")
	if en != 0.75 {
		t.Fatalf("then / so / but over four sentences; got %.2f, want 0.75", en)
	}
}

// 🚨 The zh list overlaps on purpose (但 inside 但是, 同时 inside 与此同时,
// 而 inside 而且). countZH takes the longest match and consumes it, so
// 「但是」 is one connective, not two. Summing strings.Count per marker —
// what this package did before — would have doubled exactly the writing
// that leans on them.
func TestConnectiveDensity_OverlappingMarkersCountOnce(t *testing.T) {
	if got := ConnectiveDensity("但是我来了。", "zh"); got != 1 {
		t.Fatalf("但是 is one connective, not 但 plus 但是; got %.2f, want 1", got)
	}
	if got := ConnectiveDensity("与此同时他走了。", "zh"); got != 1 {
		t.Fatalf("与此同时 is one connective, not 同时 plus 与此同时; got %.2f, want 1", got)
	}
	// 而且 / 不但 / 同时 are three separate connectives in one sentence.
	if got := ConnectiveDensity("而且我不但早到，同时还带了书。", "zh"); got != 3 {
		t.Fatalf("got %.2f, want 3", got)
	}
}

// "firstly" must not also count as "first" — the boundary rule keeps the
// two entries apart.
func TestConnectiveDensity_EnglishWordBoundariesKeepPairsApart(t *testing.T) {
	if got := ConnectiveDensity("Firstly, I disagree.", "en"); got != 1 {
		t.Fatalf("firstly is one connective, not firstly plus first; got %.2f, want 1", got)
	}
}
