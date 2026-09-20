package store_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"mindimprint/api/internal/store/sqlc"
)

// TestWritingStore_DefaultsOnCreate — a new writing starts at the beginning
// of the four-stage map, with no length decided yet, and not finished.
func TestWritingStore_DefaultsOnCreate(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	pool := newStoreTestPool(t)
	q := sqlc.New(pool)
	ctx := context.Background()

	var uid uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT id FROM users LIMIT 1`).Scan(&uid); err != nil {
		t.Fatalf("seed user: %v", err)
	}

	a, err := q.CreateAtom(ctx, sqlc.CreateAtomParams{Kind: "writing", UserID: uid})
	if err != nil {
		t.Fatalf("CreateAtom: %v", err)
	}
	w, err := q.CreateWriting(ctx, sqlc.CreateWritingParams{AtomID: a.ID, Title: "中国是否让地球变得更可持续？", Lang: "zh"})
	if err != nil {
		t.Fatalf("CreateWriting: %v", err)
	}
	if w.Stage != "outline" {
		t.Fatalf("stage = %q, want \"outline\"", w.Stage)
	}
	if w.TargetWords != nil {
		t.Fatalf("target_words = %v, want NULL", w.TargetWords)
	}
	if w.Status != "active" {
		t.Fatalf("status = %q, want \"active\"", w.Status)
	}
}

// TestWritingStore_ListWritingsByUser_OwnershipAndOrder — the writing list
// scopes to the caller and comes back newest first, joined through atom.
func TestWritingStore_ListWritingsByUser_OwnershipAndOrder(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	pool := newStoreTestPool(t)
	q := sqlc.New(pool)
	ctx := context.Background()

	var uid uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT id FROM users LIMIT 1`).Scan(&uid); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	otherUID := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO users (id, school_id, email, display_name, password_hash, role, avatar_color)
		SELECT $1, school_id, 'other-writing-owner@example.com', 'Other', password_hash, role, avatar_color FROM users WHERE id = $2`,
		otherUID, uid); err != nil {
		t.Fatalf("seed other user: %v", err)
	}

	otherAtom, err := q.CreateAtom(ctx, sqlc.CreateAtomParams{Kind: "writing", UserID: otherUID})
	if err != nil {
		t.Fatalf("CreateAtom(other): %v", err)
	}
	if _, err := q.CreateWriting(ctx, sqlc.CreateWritingParams{AtomID: otherAtom.ID, Title: "not mine", Lang: "zh"}); err != nil {
		t.Fatalf("CreateWriting(other): %v", err)
	}

	a1, err := q.CreateAtom(ctx, sqlc.CreateAtomParams{Kind: "writing", UserID: uid})
	if err != nil {
		t.Fatalf("CreateAtom 1: %v", err)
	}
	if _, err := q.CreateWriting(ctx, sqlc.CreateWritingParams{AtomID: a1.ID, Title: "第一篇", Lang: "zh"}); err != nil {
		t.Fatalf("CreateWriting 1: %v", err)
	}
	a2, err := q.CreateAtom(ctx, sqlc.CreateAtomParams{Kind: "writing", UserID: uid})
	if err != nil {
		t.Fatalf("CreateAtom 2: %v", err)
	}
	if _, err := q.CreateWriting(ctx, sqlc.CreateWritingParams{AtomID: a2.ID, Title: "第二篇", Lang: "zh"}); err != nil {
		t.Fatalf("CreateWriting 2: %v", err)
	}

	// Also seed a reading atom for the same user — it must not leak into the
	// writing list, since ListWritingsByUser filters on atom.kind = 'writing'.
	rAtom, err := q.CreateAtom(ctx, sqlc.CreateAtomParams{Kind: "reading", UserID: uid})
	if err != nil {
		t.Fatalf("CreateAtom(reading): %v", err)
	}
	if _, err := q.CreateReading(ctx, sqlc.CreateReadingParams{AtomID: rAtom.ID, Title: "不是写作", Lang: "zh"}); err != nil {
		t.Fatalf("CreateReading: %v", err)
	}

	list, err := q.ListWritingsByUser(ctx, uid)
	if err != nil {
		t.Fatalf("ListWritingsByUser: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("list = %+v, want exactly my 2 writings", list)
	}
	if list[0].Title != "第二篇" || list[1].Title != "第一篇" {
		t.Fatalf("list order = [%q, %q], want newest first [第二篇, 第一篇]", list[0].Title, list[1].Title)
	}
}

// TestWritingStore_CascadesFromAtom — deleting the atom empties outline,
// snippet and draft too, the same invariant reading already has.
func TestWritingStore_CascadesFromAtom(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	pool := newStoreTestPool(t)
	q := sqlc.New(pool)
	ctx := context.Background()

	var uid uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT id FROM users LIMIT 1`).Scan(&uid); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	a, err := q.CreateAtom(ctx, sqlc.CreateAtomParams{Kind: "writing", UserID: uid})
	if err != nil {
		t.Fatalf("CreateAtom: %v", err)
	}
	if _, err := q.CreateWriting(ctx, sqlc.CreateWritingParams{AtomID: a.ID, Title: "t", Lang: "zh"}); err != nil {
		t.Fatalf("CreateWriting: %v", err)
	}
	if _, err := q.ReplaceWritingOutline(ctx, sqlc.ReplaceWritingOutlineParams{
		AtomID:    a.ID,
		Texts:     []string{"引子", "论证"},
		Roles:     []string{"", ""},
		Depths:    []int32{0, 0},
		Positions: []int32{0, 1},
		Sources:   []string{"", ""},
		Kinds:     []string{"opening", "thesis"},
		Methods:   []string{"", ""},
	}); err != nil {
		t.Fatalf("ReplaceWritingOutline: %v", err)
	}
	if _, err := q.UpsertWritingSnippet(ctx, sqlc.UpsertWritingSnippetParams{
		AtomID: a.ID, Position: 0, Text: "第一段的内容",
	}); err != nil {
		t.Fatalf("UpsertWritingSnippet: %v", err)
	}
	if _, err := q.UpsertWritingDraft(ctx, sqlc.UpsertWritingDraftParams{AtomID: a.ID, Body: "全文"}); err != nil {
		t.Fatalf("UpsertWritingDraft: %v", err)
	}

	if _, err := pool.Exec(ctx, `DELETE FROM atom WHERE id = $1`, a.ID); err != nil {
		t.Fatalf("delete atom: %v", err)
	}

	for table, name := range map[string]string{
		"writing":         "writing",
		"writing_outline": "writing_outline",
		"writing_snippet": "writing_snippet",
		"writing_draft":   "writing_draft",
	} {
		var n int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM `+table+` WHERE atom_id = $1`, a.ID).Scan(&n); err != nil {
			t.Fatalf("count %s: %v", name, err)
		}
		if n != 0 {
			t.Fatalf("orphaned %d rows in %s after atom delete", n, name)
		}
	}
}

// TestWritingStore_StageCheckRejectsInvalidValue — stage is a fixed five-value
// vocabulary; anything else must bounce off the CHECK constraint.
func TestWritingStore_StageCheckRejectsInvalidValue(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	pool := newStoreTestPool(t)
	q := sqlc.New(pool)
	ctx := context.Background()

	var uid uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT id FROM users LIMIT 1`).Scan(&uid); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	a, err := q.CreateAtom(ctx, sqlc.CreateAtomParams{Kind: "writing", UserID: uid})
	if err != nil {
		t.Fatalf("CreateAtom: %v", err)
	}
	if _, err := q.CreateWriting(ctx, sqlc.CreateWritingParams{AtomID: a.ID, Title: "t", Lang: "zh"}); err != nil {
		t.Fatalf("CreateWriting: %v", err)
	}

	if _, err := q.SetWritingStage(ctx, sqlc.SetWritingStageParams{AtomID: a.ID, Stage: "drafting"}); err == nil {
		t.Fatal("invalid stage \"drafting\" accepted, want a check-violation")
	}

	// A legal value still goes through, and all five values the CHECK
	// constraint permits are individually acceptable — including 'finished',
	// which coexists with status='finished' as a separate fact (how far she
	// got vs. whether she's done).
	//
	// 'ideate' is deliberately still in this list. The four-step map collapsed
	// to three on 2026-08-27 and 0100 migrated every row off it, but the CHECK
	// constraint was left permissive on purpose (tightening it means rebuilding
	// it, and a database that tolerates a value nothing writes costs nothing).
	// The narrowing lives one layer up, in validWritingStages — so this test
	// asserting the DB still accepts 'ideate' and the API test asserting a
	// 400 for it are both correct, and are describing different layers.
	for _, stage := range []string{"ideate", "outline", "snippets", "draft", "finished"} {
		w, err := q.SetWritingStage(ctx, sqlc.SetWritingStageParams{AtomID: a.ID, Stage: stage})
		if err != nil {
			t.Fatalf("SetWritingStage(%q): %v", stage, err)
		}
		if w.Stage != stage {
			t.Fatalf("stage = %q, want %q", w.Stage, stage)
		}
	}
}

// TestWritingStore_TargetWordsAndFinish — target_words is set once decided in
// 构思, and finishing is idempotent and guarded like reading's.
func TestWritingStore_TargetWordsAndFinish(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	pool := newStoreTestPool(t)
	q := sqlc.New(pool)
	ctx := context.Background()

	var uid uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT id FROM users LIMIT 1`).Scan(&uid); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	a, err := q.CreateAtom(ctx, sqlc.CreateAtomParams{Kind: "writing", UserID: uid})
	if err != nil {
		t.Fatalf("CreateAtom: %v", err)
	}
	if _, err := q.CreateWriting(ctx, sqlc.CreateWritingParams{AtomID: a.ID, Title: "t", Lang: "zh"}); err != nil {
		t.Fatalf("CreateWriting: %v", err)
	}

	words := int32(800)
	if err := q.SetWritingTargetWords(ctx, sqlc.SetWritingTargetWordsParams{AtomID: a.ID, TargetWords: &words}); err != nil {
		t.Fatalf("SetWritingTargetWords: %v", err)
	}
	got, err := q.GetWriting(ctx, a.ID)
	if err != nil {
		t.Fatalf("GetWriting: %v", err)
	}
	if got.TargetWords == nil || *got.TargetWords != 800 {
		t.Fatalf("target_words = %v, want 800", got.TargetWords)
	}

	if err := q.RenameWriting(ctx, sqlc.RenameWritingParams{AtomID: a.ID, Title: "改标题"}); err != nil {
		t.Fatalf("RenameWriting: %v", err)
	}

	if err := q.SetWritingFinished(ctx, a.ID); err != nil {
		t.Fatalf("SetWritingFinished: %v", err)
	}
	finished, err := q.GetWriting(ctx, a.ID)
	if err != nil {
		t.Fatalf("GetWriting after finish: %v", err)
	}
	if finished.Status != "finished" || finished.Title != "改标题" {
		t.Fatalf("after finish = %+v, want status=finished title=改标题", finished)
	}
	if !finished.FinishedAt.Valid {
		t.Fatal("finished_at not set")
	}
	// `status` and `stage` are independent facts and must stay that way.
	// `status` answers "is this piece done"; `stage` answers "how far through
	// 结构→段落→成稿 did she actually get", and the report reads the
	// latter. This writing finished without ever leaving the first step — a student
	// who skips straight to a finished draft, which is a real thing students
	// do and a thing 过程即数据 says we record rather than tidy away.
	//
	// Nothing else guards this: a future edit to SetWritingFinished that also
	// forced stage='finished' would pass every other assertion in this file
	// while silently erasing the skip from every report.
	if finished.Stage != "outline" {
		t.Fatalf("finish moved stage to %q; status and stage must stay independent", finished.Stage)
	}
	firstFinishedAt := finished.FinishedAt.Time

	// Second finish is a no-op (guarded), so finished_at does not drift.
	if err := q.SetWritingFinished(ctx, a.ID); err != nil {
		t.Fatalf("SetWritingFinished (second): %v", err)
	}
	again, err := q.GetWriting(ctx, a.ID)
	if err != nil {
		t.Fatalf("GetWriting after second finish: %v", err)
	}
	if !again.FinishedAt.Valid || !again.FinishedAt.Time.Equal(firstFinishedAt) {
		t.Fatalf("finished_at drifted on second finish: %v -> %v", firstFinishedAt, again.FinishedAt)
	}
}

// TestWritingStore_OutlineSnippetDraftRoundTrip — outline is replaced whole,
// snippets are upserted one at a time keyed by position, draft is a single
// upserted body.
func TestWritingStore_OutlineSnippetDraftRoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	pool := newStoreTestPool(t)
	q := sqlc.New(pool)
	ctx := context.Background()

	var uid uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT id FROM users LIMIT 1`).Scan(&uid); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	a, err := q.CreateAtom(ctx, sqlc.CreateAtomParams{Kind: "writing", UserID: uid})
	if err != nil {
		t.Fatalf("CreateAtom: %v", err)
	}
	if _, err := q.CreateWriting(ctx, sqlc.CreateWritingParams{AtomID: a.ID, Title: "t", Lang: "zh"}); err != nil {
		t.Fatalf("CreateWriting: %v", err)
	}

	// Outline: full replace, ordered by position.
	if _, err := q.ReplaceWritingOutline(ctx, sqlc.ReplaceWritingOutlineParams{
		AtomID:    a.ID,
		Texts:     []string{"引子", "论证一", "结论"},
		Roles:     []string{"", "", ""},
		Depths:    []int32{0, 0, 0},
		Positions: []int32{0, 1, 2},
		Sources:   []string{"", "", ""},
		Kinds:     []string{"opening", "thesis", "closing"},
		Methods:   []string{"", "", ""},
	}); err != nil {
		t.Fatalf("ReplaceWritingOutline: %v", err)
	}
	outline, err := q.ListWritingOutline(ctx, a.ID)
	if err != nil {
		t.Fatalf("ListWritingOutline: %v", err)
	}
	if len(outline) != 3 || outline[0].Text != "引子" || outline[2].Text != "结论" {
		t.Fatalf("outline = %+v, want 3 items in position order", outline)
	}

	// A second replace call REPLACES, not appends.
	if _, err := q.ReplaceWritingOutline(ctx, sqlc.ReplaceWritingOutlineParams{
		AtomID:    a.ID,
		Texts:     []string{"仅一项"},
		Roles:     []string{""},
		Depths:    []int32{0},
		Positions: []int32{0},
		Sources:   []string{""},
		Kinds:     []string{"thesis"},
		Methods:   []string{""},
	}); err != nil {
		t.Fatalf("ReplaceWritingOutline (2nd): %v", err)
	}
	outline2, err := q.ListWritingOutline(ctx, a.ID)
	if err != nil {
		t.Fatalf("ListWritingOutline (2nd): %v", err)
	}
	if len(outline2) != 1 || outline2[0].Text != "仅一项" {
		t.Fatalf("outline after replace = %+v, want exactly [仅一项]", outline2)
	}

	// Snippets: upsert is keyed by (atom_id, position) — writing the same
	// position again edits in place instead of duplicating.
	if _, err := q.UpsertWritingSnippet(ctx, sqlc.UpsertWritingSnippetParams{
		AtomID: a.ID, Position: 0, Text: "第一版",
	}); err != nil {
		t.Fatalf("UpsertWritingSnippet: %v", err)
	}
	if _, err := q.UpsertWritingSnippet(ctx, sqlc.UpsertWritingSnippetParams{
		AtomID: a.ID, Position: 0, Text: "第二版",
	}); err != nil {
		t.Fatalf("UpsertWritingSnippet (edit): %v", err)
	}
	snippets, err := q.ListWritingSnippets(ctx, a.ID)
	if err != nil {
		t.Fatalf("ListWritingSnippets: %v", err)
	}
	if len(snippets) != 1 || snippets[0].Text != "第二版" {
		t.Fatalf("snippets = %+v, want exactly one edited-in-place row", snippets)
	}

	// Draft: single upserted body, composed from snippets (but composition
	// itself is a later task — this only checks the storage round trip).
	if _, err := q.UpsertWritingDraft(ctx, sqlc.UpsertWritingDraftParams{AtomID: a.ID, Body: "第二版"}); err != nil {
		t.Fatalf("UpsertWritingDraft: %v", err)
	}
	draft, err := q.GetWritingDraft(ctx, a.ID)
	if err != nil {
		t.Fatalf("GetWritingDraft: %v", err)
	}
	if draft.Body != "第二版" {
		t.Fatalf("draft.Body = %q, want %q", draft.Body, "第二版")
	}
	if _, err := q.UpsertWritingDraft(ctx, sqlc.UpsertWritingDraftParams{AtomID: a.ID, Body: "第二版，改过"}); err != nil {
		t.Fatalf("UpsertWritingDraft (edit): %v", err)
	}
	draft2, err := q.GetWritingDraft(ctx, a.ID)
	if err != nil {
		t.Fatalf("GetWritingDraft (2nd): %v", err)
	}
	if draft2.Body != "第二版，改过" {
		t.Fatalf("draft.Body after edit = %q, want %q", draft2.Body, "第二版，改过")
	}
}
