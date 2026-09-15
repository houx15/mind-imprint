package store_test

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"mindimprint/api/internal/store/sqlc"
)

// assertJSONEqual compares two jsonb payloads by value, not by byte layout —
// Postgres re-serializes jsonb (e.g. adds a space after ":" and ",") so a raw
// byte comparison against what a test inserted would fail even when the
// query left the content untouched.
func assertJSONEqual(t *testing.T, got, want []byte, what string) {
	t.Helper()
	var gv, wv any
	if err := json.Unmarshal(got, &gv); err != nil {
		t.Fatalf("%s: unmarshal got %s: %v", what, got, err)
	}
	if err := json.Unmarshal(want, &wv); err != nil {
		t.Fatalf("%s: unmarshal want %s: %v", what, want, err)
	}
	if !reflect.DeepEqual(gv, wv) {
		t.Fatalf("%s = %s, want %s", what, got, want)
	}
}

// seedLiteGradingWriting creates one atom + writing + writing_version row and
// returns their ids, for tests that only need a version_id to hang a grading
// row off of.
func seedLiteGradingWriting(t *testing.T, ctx context.Context, pool *pgxpool.Pool) (atomID, versionID uuid.UUID) {
	t.Helper()
	q := sqlc.New(pool)
	a, err := q.CreateAtom(ctx, sqlc.CreateAtomParams{Kind: "writing", UserID: seededStudentID})
	if err != nil {
		t.Fatalf("CreateAtom: %v", err)
	}
	if _, err := q.CreateWriting(ctx, sqlc.CreateWritingParams{AtomID: a.ID, Title: "雨", Lang: "zh"}); err != nil {
		t.Fatalf("CreateWriting: %v", err)
	}
	v, err := q.CreateWritingVersion(ctx, sqlc.CreateWritingVersionParams{
		AtomID: a.ID, Number: 1, Title: "雨", Body: "下雨了。", WordCount: 4,
	})
	if err != nil {
		t.Fatalf("CreateWritingVersion: %v", err)
	}
	return a.ID, v.ID
}

// seedLiteGradingRow inserts a lite_grading row directly (not through
// CreateLiteGrading, which always starts at 'queued') so a test can put the
// row in whatever state — running with old content carried over from a prior
// draft, running with none, draft with an error — the ruling under test needs.
func seedLiteGradingRow(t *testing.T, ctx context.Context, pool *pgxpool.Pool, versionID, atomID uuid.UUID, status string, ai, content []byte, gradingErr *string, updatedAt *time.Time) uuid.UUID {
	t.Helper()
	id := uuid.New()
	when := time.Now()
	if updatedAt != nil {
		when = *updatedAt
	}
	_, err := pool.Exec(ctx, `
		INSERT INTO lite_grading (id, atom_id, version_id, user_id, class_id, rubric, status, ai, content, error, requested_by, updated_at)
		VALUES ($1, $2, $3, $4, $5, '{}', $6, $7, $8, $9, $4, $10)`,
		id, atomID, versionID, seededStudentID, seededClassID, status, ai, content, gradingErr, when)
	if err != nil {
		t.Fatalf("seed lite_grading: %v", err)
	}
	return id
}

// TestSetLiteGradingFailed_KeepsPreviousDraftContentAsDraft pins the
// controller ruling: a regrade (status running, but ai/content already hold
// the previous successful result) that fails goes back to 'draft' with the
// old ai/content/reviewed_at untouched, and the failure reason lands in
// error — it does not become 'failed' and does not lose the student's
// readable draft.
func TestSetLiteGradingFailed_KeepsPreviousDraftContentAsDraft(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newStoreTestPool(t)
	q := sqlc.New(pool)

	atomID, versionID := seedLiteGradingWriting(t, ctx, pool)
	prevAI := []byte(`{"overall":{"grade":"B","comment":"旧的一版"}}`)
	prevContent := []byte(`{"overall":{"grade":"B","comment":"老师改过的一版"}}`)
	id := seedLiteGradingRow(t, ctx, pool, versionID, atomID, "running", prevAI, prevContent, nil, nil)

	rows, err := q.SetLiteGradingFailed(ctx, sqlc.SetLiteGradingFailedParams{
		Error: ptr("模型调用失败：超时"), ID: id,
	})
	if err != nil {
		t.Fatalf("SetLiteGradingFailed: %v", err)
	}
	if rows != 1 {
		t.Fatalf("rows affected = %d, want 1", rows)
	}

	got, err := q.GetLiteGrading(ctx, id)
	if err != nil {
		t.Fatalf("GetLiteGrading: %v", err)
	}
	if got.Status != "draft" {
		t.Fatalf("status = %q, want draft", got.Status)
	}
	if got.Error == nil || *got.Error != "模型调用失败：超时" {
		t.Fatalf("error = %v, want the failure reason", got.Error)
	}
	assertJSONEqual(t, got.Ai, prevAI, "ai")
	assertJSONEqual(t, got.Content, prevContent, "content")
}

// TestSetLiteGradingFailed_FirstGradingWithNoContentFails pins the other half
// of the ruling: a first grading (no prior content to fall back to) that
// fails ends in 'failed', not 'draft'.
func TestSetLiteGradingFailed_FirstGradingWithNoContentFails(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newStoreTestPool(t)
	q := sqlc.New(pool)

	atomID, versionID := seedLiteGradingWriting(t, ctx, pool)
	id := seedLiteGradingRow(t, ctx, pool, versionID, atomID, "running", nil, nil, nil, nil)

	rows, err := q.SetLiteGradingFailed(ctx, sqlc.SetLiteGradingFailedParams{
		Error: ptr("模型返回的 JSON 解析失败"), ID: id,
	})
	if err != nil {
		t.Fatalf("SetLiteGradingFailed: %v", err)
	}
	if rows != 1 {
		t.Fatalf("rows affected = %d, want 1", rows)
	}

	got, err := q.GetLiteGrading(ctx, id)
	if err != nil {
		t.Fatalf("GetLiteGrading: %v", err)
	}
	if got.Status != "failed" {
		t.Fatalf("status = %q, want failed", got.Status)
	}
	if got.Error == nil || *got.Error != "模型返回的 JSON 解析失败" {
		t.Fatalf("error = %v, want the failure reason", got.Error)
	}
	if got.Content != nil {
		t.Fatalf("content = %s, want nil", got.Content)
	}
}

// TestMarkStaleLiteGradingsFailed_RunningWithContentGoesToDraft pins the same
// ruling for the sweep query: a stuck 'running' row that already carries
// draft content from a prior success goes back to 'draft' with error set; one
// with no content lands in 'failed'; a 'queued' row (never claimed) is left
// alone even if it is old.
func TestMarkStaleLiteGradingsFailed_RunningWithContentGoesToDraft(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newStoreTestPool(t)
	q := sqlc.New(pool)

	stale := time.Now().Add(-20 * time.Minute)

	_, withContentVersion := seedLiteGradingWriting(t, ctx, pool)
	atomWithContent, _ := seedLiteGradingWriting(t, ctx, pool)
	prevContent := []byte(`{"overall":{"grade":"A","comment":"之前的草稿"}}`)
	withContentID := seedLiteGradingRow(t, ctx, pool, withContentVersion, atomWithContent, "running", prevContent, prevContent, nil, &stale)

	atomNoContent, noContentVersion := seedLiteGradingWriting(t, ctx, pool)
	noContentID := seedLiteGradingRow(t, ctx, pool, noContentVersion, atomNoContent, "running", nil, nil, nil, &stale)

	atomQueued, queuedVersion := seedLiteGradingWriting(t, ctx, pool)
	queuedID := seedLiteGradingRow(t, ctx, pool, queuedVersion, atomQueued, "queued", nil, nil, nil, &stale)

	if err := q.MarkStaleLiteGradingsFailed(ctx, []uuid.UUID{withContentID, noContentID, queuedID}); err != nil {
		t.Fatalf("MarkStaleLiteGradingsFailed: %v", err)
	}

	withContentRow, err := q.GetLiteGrading(ctx, withContentID)
	if err != nil {
		t.Fatalf("GetLiteGrading(withContent): %v", err)
	}
	if withContentRow.Status != "draft" {
		t.Fatalf("withContent status = %q, want draft", withContentRow.Status)
	}
	if withContentRow.Error == nil || *withContentRow.Error != "批改超时：任务未完成" {
		t.Fatalf("withContent error = %v, want the timeout message", withContentRow.Error)
	}
	assertJSONEqual(t, withContentRow.Content, prevContent, "withContent content")

	noContentRow, err := q.GetLiteGrading(ctx, noContentID)
	if err != nil {
		t.Fatalf("GetLiteGrading(noContent): %v", err)
	}
	if noContentRow.Status != "failed" {
		t.Fatalf("noContent status = %q, want failed", noContentRow.Status)
	}
	if noContentRow.Error == nil || *noContentRow.Error != "批改超时：任务未完成" {
		t.Fatalf("noContent error = %v, want the timeout message", noContentRow.Error)
	}

	queuedRow, err := q.GetLiteGrading(ctx, queuedID)
	if err != nil {
		t.Fatalf("GetLiteGrading(queued): %v", err)
	}
	if queuedRow.Status != "queued" || queuedRow.Error != nil {
		t.Fatalf("queued row = %+v, want left alone", queuedRow)
	}
}

// TestSetLiteGradingDraft_ClearsError pins that a successful grading clears
// any error left over from a previous failed attempt on the same row.
func TestSetLiteGradingDraft_ClearsError(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newStoreTestPool(t)
	q := sqlc.New(pool)

	atomID, versionID := seedLiteGradingWriting(t, ctx, pool)
	id := seedLiteGradingRow(t, ctx, pool, versionID, atomID, "running", nil, nil, ptr("上一次失败的原因"), nil)

	result := []byte(`{"overall":{"grade":"A","comment":"这次成功了"}}`)
	rows, err := q.SetLiteGradingDraft(ctx, sqlc.SetLiteGradingDraftParams{Result: result, ID: id})
	if err != nil {
		t.Fatalf("SetLiteGradingDraft: %v", err)
	}
	if rows != 1 {
		t.Fatalf("rows affected = %d, want 1", rows)
	}

	got, err := q.GetLiteGrading(ctx, id)
	if err != nil {
		t.Fatalf("GetLiteGrading: %v", err)
	}
	if got.Status != "draft" {
		t.Fatalf("status = %q, want draft", got.Status)
	}
	if got.Error != nil {
		t.Fatalf("error = %v, want nil (cleared)", got.Error)
	}
	assertJSONEqual(t, got.Ai, result, "ai")
	assertJSONEqual(t, got.Content, result, "content")
}

// TestRequeueLiteGrading_KeepsRubricOfRowWithContent (FB-1): a regrade must
// not change the rubric of a row that already has content, because a failed
// regrade returns that row to draft with the old content, which the old
// rubric describes. A row with no content takes the new rubric at once.
// SetLiteGradingDraft then writes the rubric it was graded with.
func TestRequeueLiteGrading_KeepsRubricOfRowWithContent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newStoreTestPool(t)
	q := sqlc.New(pool)
	newRubric := []byte(`{"scale":"points","max":20,"dimensions":[{"name":"论证","note":""}],"focus":""}`)

	atomA, versionA := seedLiteGradingWriting(t, ctx, pool)
	prev := []byte(`{"overall":{"grade":"B","comment":"旧的一版"}}`)
	withContent := seedLiteGradingRow(t, ctx, pool, versionA, atomA, "draft", prev, prev, nil, nil)
	got, err := q.RequeueLiteGrading(ctx, sqlc.RequeueLiteGradingParams{ID: withContent, Rubric: newRubric, RequestedBy: seededStudentID})
	if err != nil {
		t.Fatalf("RequeueLiteGrading(with content): %v", err)
	}
	if got.Status != "queued" {
		t.Fatalf("status = %q, want queued", got.Status)
	}
	assertJSONEqual(t, got.Rubric, []byte(`{}`), "rubric of a row with content")

	atomB, versionB := seedLiteGradingWriting(t, ctx, pool)
	noContent := seedLiteGradingRow(t, ctx, pool, versionB, atomB, "failed", nil, nil, ptr("第一次失败"), nil)
	got, err = q.RequeueLiteGrading(ctx, sqlc.RequeueLiteGradingParams{ID: noContent, Rubric: newRubric, RequestedBy: seededStudentID})
	if err != nil {
		t.Fatalf("RequeueLiteGrading(no content): %v", err)
	}
	assertJSONEqual(t, got.Rubric, newRubric, "rubric of a row without content")

	if _, err := pool.Exec(ctx, `UPDATE lite_grading SET status = 'running' WHERE id = $1`, withContent); err != nil {
		t.Fatalf("mark running: %v", err)
	}
	result := []byte(`{"overall":{"grade":"15","comment":"新的一版"}}`)
	if _, err := q.SetLiteGradingDraft(ctx, sqlc.SetLiteGradingDraftParams{ID: withContent, Result: result, Rubric: newRubric}); err != nil {
		t.Fatalf("SetLiteGradingDraft: %v", err)
	}
	row, err := q.GetLiteGrading(ctx, withContent)
	if err != nil {
		t.Fatalf("GetLiteGrading: %v", err)
	}
	assertJSONEqual(t, row.Rubric, newRubric, "rubric after a successful regrade")
	assertJSONEqual(t, row.Content, result, "content after a successful regrade")
}

// TestUpdateLiteGradingContent_ClearsError pins that a teacher's save clears
// any error left over from a previous failed regrade on the same row (the
// teacher is looking at the draft that ruling kept editable).
func TestUpdateLiteGradingContent_ClearsError(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newStoreTestPool(t)
	q := sqlc.New(pool)

	atomID, versionID := seedLiteGradingWriting(t, ctx, pool)
	prevContent := []byte(`{"overall":{"grade":"B","comment":"旧内容"}}`)
	id := seedLiteGradingRow(t, ctx, pool, versionID, atomID, "draft", prevContent, prevContent, ptr("上一次重新批改失败的原因"), nil)

	newContent := []byte(`{"overall":{"grade":"A","comment":"老师改过了"}}`)
	got, err := q.UpdateLiteGradingContent(ctx, sqlc.UpdateLiteGradingContentParams{Content: newContent, ID: id})
	if err != nil {
		t.Fatalf("UpdateLiteGradingContent: %v", err)
	}
	if got.Error != nil {
		t.Fatalf("error = %v, want nil (cleared by save)", got.Error)
	}
	assertJSONEqual(t, got.Content, newContent, "content")
	if got.ReviewedAt.Time.IsZero() || !got.ReviewedAt.Valid {
		t.Fatalf("reviewed_at not set: %+v", got.ReviewedAt)
	}
}
