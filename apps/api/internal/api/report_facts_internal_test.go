package api

// This file is `package api`, NOT `package api_test`: buildReadingCorpus,
// buildWritingCorpus, stripQuotedLines, countWordsForLang, and
// cappedGapSeconds are all unexported, and every reading/writing test file
// in this directory is package api_test and cannot see them.
//
// TestBuildReadingCorpus and TestBuildWritingCorpus are R4's enforcement
// point at the corpus-builder level: each asserts that a specific
// EXCLUDED field's text does NOT appear anywhere in the built corpus.

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/store/sqlc"
)

// R4's enforcement point. Since sub-project A a student message carries the
// sentences she POINTED AT in the article as leading "> " lines. They live in
// a row whose role says 'student' but they are the ARTICLE's words, and an
// exported picture must never put them under her name.
func TestStripQuotedLines(t *testing.T) {
	in := "> 中国的碳排放总量位居世界第一。\n> 但人均排放仍低于多数发达国家。\n\n我觉得人均更能说明责任。"
	got := stripQuotedLines(in)
	if strings.Contains(got, "碳排放总量位居世界第一") {
		t.Error("an article sentence survived into her corpus")
	}
	if !strings.Contains(got, "我觉得人均更能说明责任") {
		t.Error("her own sentence was stripped")
	}
}

// TestStripQuotedLinesEdgeCases locks in the 4 edge cases the dispatch named
// as verified only by reading, not by a test — each guards a specific,
// plausible "simplification" regression.
func TestStripQuotedLinesEdgeCases(t *testing.T) {
	t.Run("indented quote line is still stripped", func(t *testing.T) {
		// Guards: someone "simplifies" the check to strings.HasPrefix(line, ">"),
		// dropping the TrimSpace. An indented article quote would then flow
		// straight into the corpus.
		in := "   > 中国的碳排放总量位居世界第一。\n她的话"
		got := stripQuotedLines(in)
		if strings.Contains(got, "碳排放总量位居世界第一") {
			t.Error("an indented article quote line survived — TrimSpace before the prefix check must have been dropped")
		}
		if !strings.Contains(got, "她的话") {
			t.Error("her own line was stripped alongside the indented quote")
		}
	})

	t.Run("mid-line > in her own prose is not stripped", func(t *testing.T) {
		// The mirror-image R4 bug: deleting HER words because a line contains
		// ">" somewhere that isn't a leading blockquote marker.
		in := "3 > 2 是显然的"
		got := stripQuotedLines(in)
		if got != in {
			t.Errorf("a mid-line > in her own prose was stripped: got %q, want unchanged %q", got, in)
		}
	})

	t.Run("CRLF: article line goes, her line survives", func(t *testing.T) {
		in := "> foo\r\n她的话\r\n"
		got := stripQuotedLines(in)
		if strings.Contains(got, "foo") {
			t.Error("a CRLF-terminated article quote line survived")
		}
		if !strings.Contains(got, "她的话") {
			t.Error("her CRLF-terminated line was stripped")
		}
	})

	t.Run("entirely quote lines yields empty string", func(t *testing.T) {
		in := "> 第一行引用\n> 第二行引用"
		got := stripQuotedLines(in)
		if got != "" {
			t.Errorf("an all-quote message should strip to empty, got %q", got)
		}
	})
}

func TestCountWordsForLang(t *testing.T) {
	// The counter MUST branch on lang. This repo has already shipped a count
	// that counted characters for English and was ~5x wrong.
	cases := []struct {
		name, text, lang string
		want             int
	}{
		{"zh counts characters", "人均排放更能说明责任", "zh", 10},
		{"zh ignores spaces and punctuation", "人均排放，更能说明责任。", "zh", 10},
		{"en counts words", "Per capita emissions tell a fairer story", "en", 7},
		{"en collapses runs of spaces", "one   two\nthree", "en", 3},
		{"empty", "   ", "zh", 0},
		// Minor: lock in "zh is the default" — anything that isn't "en" takes
		// the character-counting branch, not just the literal string "zh".
		{"empty lang defaults to character counting", "人均排放更能说明责任", "", 10},
		{"unrecognized lang defaults to character counting", "人均排放更能说明责任", "fr", 10},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := countWordsForLang(c.text, c.lang); got != c.want {
				t.Errorf("countWordsForLang(%q,%s) = %d, want %d", c.text, c.lang, got, c.want)
			}
		})
	}
}

func TestCappedGapSeconds(t *testing.T) {
	base := time.Date(2026, 8, 29, 10, 0, 0, 0, time.UTC)
	at := func(m int) time.Time { return base.Add(time.Duration(m) * time.Minute) }
	// 0→2 (2m), 2→5 (3m), 5→45 (capped to 5m), 45→46 (1m) = 11m
	got := cappedGapSeconds([]time.Time{at(0), at(2), at(5), at(45), at(46)}, 5*time.Minute)
	if got != 11*60 {
		t.Errorf("cappedGapSeconds = %d, want %d", got, 11*60)
	}
	if cappedGapSeconds([]time.Time{at(0)}, 5*time.Minute) != 0 {
		t.Error("a single event is not a duration")
	}
	if cappedGapSeconds(nil, 5*time.Minute) != 0 {
		t.Error("no events is not a duration")
	}

	// Important: the only fixture above is already monotonically increasing,
	// so sort.Slice inside cappedGapSeconds is never exercised by it. Feed the
	// SAME timestamps out of order and assert the SAME total. Guards: someone
	// removes sort.Slice as "dead code" — no test fails until real,
	// plausible-from-async-writes-or-clock-skew unsorted input arrives, at
	// which point negative gaps get silently swallowed by `if gap < 0
	// { continue }`, undercounting her focus time invisibly.
	unsorted := []time.Time{at(45), at(0), at(46), at(5), at(2)}
	if gotUnsorted := cappedGapSeconds(unsorted, 5*time.Minute); gotUnsorted != 11*60 {
		t.Errorf("cappedGapSeconds on unsorted input = %d, want %d (same as sorted)", gotUnsorted, 11*60)
	}
}

// TestBuildReadingCorpus is R4's enforcement point for the reading builder:
// her own words (takeaway, note, stripped student message, submitted card
// field) must all be reachable, and every excluded field (the article quote
// she highlighted, the AI's framework_fill/finding, the AI's anchor quote,
// a non-submitted card's field, and an AI chat turn) must be UNREACHABLE.
func TestBuildReadingCorpus(t *testing.T) {
	atomID := uuid.New()

	takeaway := "我觉得应该多看数据来源，而不是只看结论。"

	notes := []sqlc.AtomAnnotation{
		{
			ID:      uuid.New(),
			AtomID:  atomID,
			BlockID: "b1",
			Quote:   "作者说中国的碳排放总量位居世界第一。",  // article's words — must be excluded
			Note:    "我批注：这句话没有说人均，可能有误导。", // hers — must be included
		},
	}

	msgs := []sqlc.AtomMessage{
		{
			ID:      uuid.New(),
			AtomID:  atomID,
			Seq:     1,
			Role:    "student",
			Content: "> 文章里被我划线的一句话\n\n我自己想说的是数据来源要溯源。",
		},
		{
			ID:      uuid.New(),
			AtomID:  atomID,
			Seq:     2,
			Role:    "ai",
			Content: "AI说的一句不该出现的话。",
		},
	}

	submittedFieldValues := []byte(`{
		"answer": "这篇文章的来源是权威机构发布的报告。",
		"nested": {"why": "因为它有可核查的数据支撑。"},
		"choice": "yes"
	}`)
	draftFieldValues := []byte(`{"answer": "这是一张还没提交的卡片里的话，不该出现。"}`)

	cards := []sqlc.AtomCard{
		{
			ID:            uuid.New(),
			AtomID:        atomID,
			CardID:        "craap",
			Status:        "submitted",
			FieldValues:   submittedFieldValues,
			EventTrace:    []byte(`[]`),
			CreatedAt:     time.Now(),
			SubmittedAt:   pgtype.Timestamptz{Time: time.Now(), Valid: true},
			Anchors:       []byte(`[{"quote": "文章里的锚点句子，不该出现"}]`),
			FrameworkFill: []byte(`{"finding": "AI生成的发现，不该出现"}`),
			Origin:        "ai_summoned",
		},
		{
			ID:          uuid.New(),
			AtomID:      atomID,
			CardID:      "sift",
			Status:      "draft",
			FieldValues: draftFieldValues,
			EventTrace:  []byte(`[]`),
			CreatedAt:   time.Now(),
			Origin:      "ai_summoned",
		},
	}

	corpus := buildReadingCorpus(takeaway, notes, msgs, cards, nil)

	// Included: hers.
	for _, want := range []string{
		takeaway,
		"我批注：这句话没有说人均，可能有误导。",
		"我自己想说的是数据来源要溯源。",
		"这篇文章的来源是权威机构发布的报告。",
		"因为它有可核查的数据支撑。",
	} {
		if !strings.Contains(corpus.Text, want) {
			t.Errorf("corpus is missing her own text: %q", want)
		}
	}

	// Excluded: not hers, and must be structurally unreachable.
	for _, mustNotAppear := range []string{
		"作者说中国的碳排放总量位居世界第一。",   // annotation.quote — article's
		"文章里被我划线的一句话",          // quoted line inside a student message — article's
		"AI说的一句不该出现的话。",        // an AI chat turn
		"这是一张还没提交的卡片里的话，不该出现。", // a draft (non-submitted) card
		"文章里的锚点句子，不该出现",        // anchors[].quote — article's
		"AI生成的发现，不该出现",         // framework_fill.finding — AI's
	} {
		if strings.Contains(corpus.Text, mustNotAppear) {
			t.Errorf("R4 VIOLATION: excluded text leaked into reading corpus: %q", mustNotAppear)
		}
	}
}

// TestBuildReadingCorpusDropsUnprefixedArticleLines is F1's server-side
// enforcement point at the corpus-builder level. ReadingCoachPanel.tsx's
// send() (before its own F1 fix) prefixed only the FIRST line of a
// multi-line quote with "> " — a drag-selection across a hard-wrapped
// paragraph (no blank line between its lines, so SplitBlocks never split it)
// returns exactly such a quote. This simulates that transcript shape — which
// can also already be sitting in storage from before the client fix — and
// asserts the article's SECOND line never reaches the corpus even though it
// carries no "> " prefix at all. stripQuotedLines alone (which only strips
// already-prefixed lines) cannot catch this; stripArticleLines, driven by
// the blocks parameter, is what does.
func TestBuildReadingCorpusDropsUnprefixedArticleLines(t *testing.T) {
	atomID := uuid.New()
	blocks := []Block{
		{ID: "b1", Text: "中国的碳排放总量位居世界第一。\n但人均排放仍低于多数发达国家。"},
	}
	msgs := []sqlc.AtomMessage{
		{
			ID:     uuid.New(),
			AtomID: atomID,
			Seq:    1,
			Role:   "student",
			// Only the first line is "> "-prefixed — the pre-fix client shape.
			Content: "> 中国的碳排放总量位居世界第一。\n但人均排放仍低于多数发达国家。\n\n我觉得这句话说明责任还要看人均。",
		},
	}

	corpus := buildReadingCorpus("", nil, msgs, nil, blocks)

	if strings.Contains(corpus.Text, "但人均排放仍低于多数发达国家") {
		t.Error("R4 VIOLATION: an unprefixed second line of the article survived into her corpus")
	}
	if !strings.Contains(corpus.Text, "我觉得这句话说明责任还要看人均") {
		t.Error("her own sentence was stripped along with the unprefixed article line")
	}
}

// TestBuildReadingCorpusDropsHerOwnLineThatCoincidesWithArticle documents
// F1's choice on the one ambiguity stripArticleLines cannot resolve: a line
// of HER OWN prose that happens, coincidentally, to be a literal substring
// of the article. stripArticleLines cannot tell that apart from an actual
// unprefixed article line reaching the transcript — and between "a rare
// false-positive drop" and "the exact R4 leak this file exists to close",
// dropping is the safer choice (see stripArticleLines' doc comment), so this
// test locks that choice in rather than the more generous alternative.
func TestBuildReadingCorpusDropsHerOwnLineThatCoincidesWithArticle(t *testing.T) {
	atomID := uuid.New()
	blocks := []Block{{ID: "b1", Text: "地球变暖是真实存在的。"}}
	msgs := []sqlc.AtomMessage{
		{
			ID:     uuid.New(),
			AtomID: atomID,
			Seq:    1,
			Role:   "student",
			// She typed this herself — it just happens to coincide, word for
			// word, with a sentence in the article.
			Content: "地球变暖是真实存在的。",
		},
	}

	corpus := buildReadingCorpus("", nil, msgs, nil, blocks)

	if strings.Contains(corpus.Text, "地球变暖是真实存在的") {
		t.Error("expected the coincidental line to be dropped — see the ruling in this test's doc comment")
	}
}

// TestBuildWritingCorpus is R4's enforcement point for the writing builder:
// her own words (draft, snippet, outline text, stripped student message)
// must all be reachable, and every excluded field (outline role/guide, and
// an AI chat turn) must be UNREACHABLE. writing_comment fields cannot even
// be passed to buildWritingCorpus — its signature has no such parameter —
// so there is nothing to assert there beyond "it compiles without one".
func TestBuildWritingCorpus(t *testing.T) {
	atomID := uuid.New()

	draft := "地球的可持续发展需要每个国家的共同努力。"

	snippets := []sqlc.WritingSnippet{
		{ID: uuid.New(), AtomID: atomID, Position: 0, Text: "第一段：引入话题的一句话。"},
		{ID: uuid.New(), AtomID: atomID, Position: 1, Text: "第二段：给出我的论点。"},
	}

	outline := []sqlc.WritingOutline{
		{
			ID:       uuid.New(),
			AtomID:   atomID,
			Text:     "先破题，再举证据",
			Depth:    0,
			Position: 0,
			Role:     "AI给的角色标签，不该出现",
			Guide:    []byte(`{"hint": "AI的引导语，不该出现"}`),
		},
	}

	msgs := []sqlc.AtomMessage{
		{
			ID:      uuid.New(),
			AtomID:  atomID,
			Seq:     1,
			Role:    "student",
			Content: "> 文章里被我引用的一句话\n\n我想在让步段里回应这一点。",
		},
		{
			ID:      uuid.New(),
			AtomID:  atomID,
			Seq:     2,
			Role:    "ai",
			Content: "AI在写作房间说的一句不该出现的话。",
		},
	}

	corpus := buildWritingCorpus(draft, snippets, outline, msgs)

	for _, want := range []string{
		draft,
		"第一段：引入话题的一句话。",
		"第二段：给出我的论点。",
		"先破题，再举证据",
		"我想在让步段里回应这一点。",
	} {
		if !strings.Contains(corpus.Text, want) {
			t.Errorf("corpus is missing her own text: %q", want)
		}
	}

	for _, mustNotAppear := range []string{
		"AI给的角色标签，不该出现",
		"AI的引导语，不该出现",
		"文章里被我引用的一句话",
		"AI在写作房间说的一句不该出现的话。",
	} {
		if strings.Contains(corpus.Text, mustNotAppear) {
			t.Errorf("R4 VIOLATION: excluded text leaked into writing corpus: %q", mustNotAppear)
		}
	}

	// Snippets are labeled by position, 1-indexed ("第 N 段").
	if label, ok := corpus.Where["第一段：引入话题的一句话。"]; !ok || label != "第 1 段" {
		t.Errorf("snippet at position 0 should be labeled 第 1 段, got %q (ok=%v)", label, ok)
	}
	if label, ok := corpus.Where["第二段：给出我的论点。"]; !ok || label != "第 2 段" {
		t.Errorf("snippet at position 1 should be labeled 第 2 段, got %q (ok=%v)", label, ok)
	}
}
