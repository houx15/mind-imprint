package api_test

// project_writing_finish_test.go — Slice 5 (#20) · the 完成写作 milestone
// endpoints (POST /finish-writing, POST /reopen-writing) + the finishProject
// writing-not-finished gate. testcontainers-backed (same harness as
// project_finish_test.go).

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/store/sqlc"
)

// TestFinishWriting_EmptyDraftRejected — an empty draft can't be finished (422
// draft_empty); the milestone stays unset.
func TestFinishWriting_EmptyDraftRejected(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, Provider: fakeProvider(), ChatResolver: fakeResolver(), SpecByID: cards.ByID}).Handler()
	cookie := signInSeed(t, pool)
	base := "/api/v1/projects/" + seedProjectID

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", base+"/finish-writing", strings.NewReader("")), cookie))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("finish-writing (empty draft) = %d, want 422; body=%s", rec.Code, rec.Body)
	}
	var perr struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &perr); err != nil || perr.Error.Code != "draft_empty" {
		t.Fatalf("finish-writing empty code = %+v (err=%v), want draft_empty", perr, err)
	}
	var n int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM writing_finish WHERE project_id=$1 AND doc_kind='essay'`, mustUUID(seedProjectID)).Scan(&n); err != nil {
		t.Fatalf("read writing_finish: %v", err)
	}
	if n != 0 {
		t.Fatalf("writing_finish rows = %d after a rejected finish-writing, want 0", n)
	}
}

// TestFinishWriting_SetsMilestoneAndIdempotent — with a non-empty draft,
// finish-writing sets the milestone (projection.writingFinished=true) and a
// second call is a no-op 200 (idempotent).
func TestFinishWriting_SetsMilestoneAndIdempotent(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, Provider: fakeProvider(), ChatResolver: fakeResolver(), SpecByID: cards.ByID}).Handler()
	cookie := signInSeed(t, pool)
	base := "/api/v1/projects/" + seedProjectID

	// A real draft buffer.
	if r := doJSON(t, h, cookie, "PUT", base+"/buffer", `{"content":"中国的可再生能源投资规模已连续五年全球第一。"}`); r.Code != http.StatusNoContent {
		t.Fatalf("PUT buffer = %d: %s", r.Code, r.Body)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", base+"/finish-writing", strings.NewReader("")), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("finish-writing = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), `"writingFinished":true`) {
		t.Fatalf("finish-writing body = %s, want writingFinished:true", rec.Body)
	}

	// The projection now carries writingFinished=true (both rooms read it).
	recProj := httptest.NewRecorder()
	h.ServeHTTP(recProj, withCookie(httptest.NewRequest("GET", base, nil), cookie))
	if !strings.Contains(recProj.Body.String(), `"writingFinished":true`) {
		t.Fatalf("projection after finish-writing = %s, want writingFinished:true", recProj.Body)
	}

	// Idempotent: a second call is a no-op 200.
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, withCookie(httptest.NewRequest("POST", base+"/finish-writing", strings.NewReader("")), cookie))
	if rec2.Code != http.StatusOK || !strings.Contains(rec2.Body.String(), `"writingFinished":true`) {
		t.Fatalf("second finish-writing = %d body=%s, want 200 writingFinished:true", rec2.Code, rec2.Body)
	}
}

// TestReopenWriting_ClearsMilestone — 重新打开写作 (铁律②) clears the milestone so
// the draft is editable again.
func TestReopenWriting_ClearsMilestone(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, Provider: fakeProvider(), ChatResolver: fakeResolver(), SpecByID: cards.ByID}).Handler()
	cookie := signInSeed(t, pool)
	base := "/api/v1/projects/" + seedProjectID

	if err := sqlc.New(pool).SetWritingFinish(context.Background(), sqlc.SetWritingFinishParams{ProjectID: mustUUID(seedProjectID), DocKind: "essay"}); err != nil {
		t.Fatalf("seed writing finished: %v", err)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", base+"/reopen-writing", strings.NewReader("")), cookie))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"writingFinished":false`) {
		t.Fatalf("reopen-writing = %d body=%s, want 200 writingFinished:false", rec.Code, rec.Body)
	}
	recProj := httptest.NewRecorder()
	h.ServeHTTP(recProj, withCookie(httptest.NewRequest("GET", base, nil), cookie))
	if !strings.Contains(recProj.Body.String(), `"writingFinished":false`) {
		t.Fatalf("projection after reopen = %s, want writingFinished:false", recProj.Body)
	}
}

// TestReopenWriting_BlockedWhenEvaluating — once the finalize path has begun
// (evaluating/finished) reopening writing is refused (409 already_finalizing).
func TestReopenWriting_BlockedWhenEvaluating(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, Provider: fakeProvider(), ChatResolver: fakeResolver(), SpecByID: cards.ByID}).Handler()
	cookie := signInSeed(t, pool)
	base := "/api/v1/projects/" + seedProjectID

	q := sqlc.New(pool)
	if err := q.SetWritingFinish(context.Background(), sqlc.SetWritingFinishParams{ProjectID: mustUUID(seedProjectID), DocKind: "essay"}); err != nil {
		t.Fatalf("seed writing finished: %v", err)
	}
	if err := q.SetProjectEvaluating(context.Background(), mustUUID(seedProjectID)); err != nil {
		t.Fatalf("seed evaluating: %v", err)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", base+"/reopen-writing", strings.NewReader("")), cookie))
	if rec.Code != http.StatusConflict {
		t.Fatalf("reopen-writing while evaluating = %d, want 409; body=%s", rec.Code, rec.Body)
	}
}

// TestFinishProject_WritingNotFinished — the finishProject writing gate: even
// with a done reflection, finish 422s writing_not_finished until 完成写作.
func TestFinishProject_WritingNotFinished(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: assessStubProvider(dualAxisReply), ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)
	projectID := materialsTestProjectID

	markReflectionDone(t, pool, projectID) // reflection done, but writing NOT finished

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+projectID+"/finish", strings.NewReader("")), cookie))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("finish (writing not finished) = %d, want 422; body=%s", rec.Code, rec.Body)
	}
	var perr struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &perr); err != nil || perr.Error.Code != "writing_not_finished" {
		t.Fatalf("finish writing-not-finished code = %+v (err=%v), want writing_not_finished; body=%s", perr, err, rec.Body)
	}
	assertProjectStatus(t, pool, projectID, "active")
}
