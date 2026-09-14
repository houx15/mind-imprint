package api

// lite_create_helpers_internal_test.go — the creation helpers that a student's
// own create buttons and an assigned practice's 开始 share. The handler tests
// (readings_test.go, writings_test.go, pbl_projects_test.go …) prove the
// handlers still answer the same; these prove the helpers themselves, called
// with no HTTP request, no entitlement check and no homepage gate.
//
// package api (not api_test): the helpers are unexported.

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"mindimprint/api/internal/library"
	"mindimprint/api/internal/store/sqlc"
)

func newCreateHelpersTestAPI(t *testing.T) (*API, *pgxpool.Pool) {
	t.Helper()
	pool := NewTestDB(t)
	q := sqlc.New(pool)
	return New(Deps{Queries: q, Pool: pool}), pool
}

func countAtoms(t *testing.T, pool *pgxpool.Pool, kind string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM atom WHERE kind = $1`, kind).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestCreateLibraryReadingForDedupes(t *testing.T) {
	a, _ := newCreateHelpersTestAPI(t)
	ctx := context.Background()
	art := library.All()[0]
	slug, tier := art.Slug, art.Levels[0].Tier

	first, resumed, err := a.createLibraryReadingFor(ctx, SeedUserID, slug, tier)
	if err != nil || resumed {
		t.Fatalf("first: %v resumed=%v", err, resumed)
	}
	second, resumed, err := a.createLibraryReadingFor(ctx, SeedUserID, slug, tier)
	if err != nil || !resumed || second != first {
		t.Fatalf("second: %v resumed=%v same=%v", err, resumed, second == first)
	}
	fresh, err := a.createLibraryReadingFreshFor(ctx, SeedUserID, slug, tier)
	if err != nil || fresh == first {
		t.Fatalf("fresh: %v same=%v", err, fresh == first)
	}
	src, err := a.d.Queries.GetReadingSource(ctx, fresh)
	if err != nil || src.Body == "" {
		t.Fatalf("fresh source = %+v err=%v", src, err)
	}
}

func TestCreateLibraryReadingForSentinels(t *testing.T) {
	a, pool := newCreateHelpersTestAPI(t)
	ctx := context.Background()
	if _, _, err := a.createLibraryReadingFor(ctx, SeedUserID, "no-such-article", 1); !errors.Is(err, errLibraryArticleNotFound) {
		t.Fatalf("unknown slug: err = %v", err)
	}
	if _, _, err := a.createLibraryReadingFor(ctx, SeedUserID, library.All()[0].Slug, 99); !errors.Is(err, errLibraryTierInvalid) {
		t.Fatalf("unknown tier: err = %v", err)
	}
	if n := countAtoms(t, pool, "reading"); n != 0 {
		t.Fatalf("reading atoms after refused creates = %d, want 0", n)
	}
}

func TestSuggestLibraryTierForEmptyShelf(t *testing.T) {
	a, _ := newCreateHelpersTestAPI(t)
	got, err := a.suggestLibraryTierFor(context.Background(), SeedUserID)
	if err != nil || got != library.SuggestTier(0, 0) {
		t.Fatalf("tier = %d err=%v, want %d", got, err, library.SuggestTier(0, 0))
	}
}

func TestCreateReadingWithSourceForText(t *testing.T) {
	a, _ := newCreateHelpersTestAPI(t)
	ctx := context.Background()
	id, err := a.createReadingWithSourceFor(ctx, SeedUserID, "雨的文章", "zh", "", "第一段。\n\n第二段。")
	if err != nil {
		t.Fatal(err)
	}
	rd, err := a.d.Queries.GetReading(ctx, id)
	if err != nil || rd.Title != "雨的文章" || rd.Lang != "zh" {
		t.Fatalf("reading = %+v err=%v", rd, err)
	}
	src, err := a.d.Queries.GetReadingSource(ctx, id)
	if err != nil || src.Title != "雨的文章" || len(SplitBlocks(src.Body)) != 2 || src.SourceUrl != nil {
		t.Fatalf("source = %+v err=%v", src, err)
	}
}

// A `javascript:` URL is later rendered as a link on the teacher's item page;
// it must be refused before any fetch and before any row is written.
func TestCreateReadingWithSourceForRejectsNonHTTPURL(t *testing.T) {
	a, pool := newCreateHelpersTestAPI(t)
	_, err := a.createReadingWithSourceFor(context.Background(), SeedUserID, "t", "zh", "javascript:alert(1)", "")
	if !errors.Is(err, errInvalidSourceURL) {
		t.Fatalf("err = %v, want errInvalidSourceURL", err)
	}
	if n := countAtoms(t, pool, "reading"); n != 0 {
		t.Fatalf("reading atoms = %d, want 0", n)
	}
}

func TestCreateReadingWithSourceForNoFetcher(t *testing.T) {
	a, pool := newCreateHelpersTestAPI(t)
	_, err := a.createReadingWithSourceFor(context.Background(), SeedUserID, "t", "zh", "https://example.com/a", "")
	if !errors.Is(err, errFetchUnavailable) {
		t.Fatalf("err = %v, want errFetchUnavailable", err)
	}
	if n := countAtoms(t, pool, "reading"); n != 0 {
		t.Fatalf("reading atoms = %d, want 0", n)
	}
}

func TestCreateReadingWithSourceForEmptyText(t *testing.T) {
	a, pool := newCreateHelpersTestAPI(t)
	_, err := a.createReadingWithSourceFor(context.Background(), SeedUserID, "t", "zh", "", "   ")
	if !errors.Is(err, errMissingSourceText) {
		t.Fatalf("err = %v, want errMissingSourceText", err)
	}
	if n := countAtoms(t, pool, "reading"); n != 0 {
		t.Fatalf("reading atoms = %d, want 0", n)
	}
}

func TestCreateWritingForSetsTargetWords(t *testing.T) {
	a, _ := newCreateHelpersTestAPI(t)
	ctx := context.Background()
	tw := int32(800)
	id, err := a.createWritingFor(ctx, SeedUserID, "写一篇关于雨的记叙文", "zh", &tw)
	if err != nil {
		t.Fatal(err)
	}
	w, err := a.d.Queries.GetWriting(ctx, id)
	if err != nil || w.TargetWords == nil || *w.TargetWords != 800 || w.Title == "" {
		t.Fatalf("writing = %+v err=%v", w, err)
	}
	msgs, err := a.d.Queries.ListAtomMessages(ctx, id)
	if err != nil || len(msgs) != 1 || msgs[0].Role != "student" || msgs[0].Content != "写一篇关于雨的记叙文" {
		t.Fatalf("messages = %+v err=%v", msgs, err)
	}
}

func TestCreateWritingForNilTargetWords(t *testing.T) {
	a, _ := newCreateHelpersTestAPI(t)
	ctx := context.Background()
	id, err := a.createWritingFor(ctx, SeedUserID, "写一篇议论文", "en", nil)
	if err != nil {
		t.Fatal(err)
	}
	w, err := a.d.Queries.GetWriting(ctx, id)
	if err != nil || w.TargetWords != nil || w.Lang != "en" {
		t.Fatalf("writing = %+v err=%v", w, err)
	}
}

// TestAssignedWritingPromptStaysOutOfReportCorpus: a report's 金句 are labelled
// as her own words and are picked only from buildWritingCorpus. An assigned
// writing's prompt is the teacher's text. It used to be stored as her first
// student message, which put it in that corpus. Now it lives on
// writing.assigned_prompt and no message is written.
func TestAssignedWritingPromptStaysOutOfReportCorpus(t *testing.T) {
	a, pool := newCreateHelpersTestAPI(t)
	ctx := context.Background()
	prompt := "写一篇关于雨的记叙文，写出雨停之前的那一刻"

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	id, err := createAssignedWritingInTx(ctx, a.d.Queries.WithTx(tx), SeedUserID, "雨", prompt, "zh", 800)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	w, err := a.d.Queries.GetWriting(ctx, id)
	if err != nil || w.Title != "雨" || w.AssignedPrompt == nil || *w.AssignedPrompt != prompt ||
		w.TargetWords == nil || *w.TargetWords != 800 {
		t.Fatalf("writing = %+v err=%v", w, err)
	}
	outline, err := a.d.Queries.ListWritingOutline(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	msgs, err := a.writingAllMessages(ctx, a.d.Queries, id, outline)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 0 {
		t.Fatalf("messages after an assigned start = %+v, want none", msgs)
	}
	if corpus := buildWritingCorpus("", nil, outline, msgs); strings.Contains(corpus.Text, prompt) {
		t.Fatalf("the teacher's prompt is in her report corpus: %q", corpus.Text)
	}
}

func TestCreatePblProjectForSkipsNoGate(t *testing.T) {
	a, _ := newCreateHelpersTestAPI(t)
	// The seed student has no published homepage, so the handler's gate would
	// refuse her. The helper does not check it; the caller does.
	p, err := a.createPblProjectFor(context.Background(), SeedUserID, "怎样让校园少用一次性杯子？")
	if err != nil || p.Idea == "" || p.Name == "" {
		t.Fatalf("project = %+v err=%v", p, err)
	}
}
