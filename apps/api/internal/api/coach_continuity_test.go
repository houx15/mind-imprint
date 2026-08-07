package api_test

// coach_continuity_test.go — S1 · one continuous per-project session. The
// four-room coach persists both sides to the project's ONE thread (surface-
// tagged), a room reloads its slice via GET /coach/history, proposal-solidify
// folds the shaping turns out of the active window, and summary-on-return
// composes once (first-open-wins).

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/store/sqlc"
)

// countCoachMsgs counts chat_message rows on the project's thread for a surface,
// optionally only the active (non-folded) ones.
func countCoachMsgs(t *testing.T, pool *pgxpool.Pool, projectID, surface string, activeOnly bool) int {
	t.Helper()
	q := `SELECT count(*) FROM chat_message cm
	      JOIN chat_thread ct ON cm.thread_id = ct.id
	      WHERE ct.seeded_project_id = $1 AND cm.surface = $2`
	if activeOnly {
		q += ` AND cm.folded_at IS NULL`
	}
	var n int
	if err := pool.QueryRow(context.Background(), q, mustUUID(projectID), surface).Scan(&n); err != nil {
		t.Fatalf("countCoachMsgs: %v", err)
	}
	return n
}

// TestCoach_ContinuityAndHistory — two coach turns persist BOTH sides to the
// one thread, surface-tagged; GET /coach/history returns that surface's slice
// in order with the workspace student|ai roles.
func TestCoach_ContinuityAndHistory(t *testing.T) {
	h, cookie, _ := planTestHandler(t)
	base := "/api/v1/projects/" + seedProjectID

	for _, msg := range []string{"第一句", "第二句"} {
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", base+"/coach",
			strings.NewReader(`{"scope":"forming","user_input":"`+msg+`"}`)), cookie))
		if rr.Code != http.StatusOK {
			t.Fatalf("coach %q = %d — %s", msg, rr.Code, rr.Body)
		}
	}

	// The continuous 印记 thread is stored under one `studio` surface now, so the
	// working-room history reads that surface (Task 5 write + read).
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("GET", base+"/coach/history?surface=studio", nil), cookie))
	if rr.Code != http.StatusOK {
		t.Fatalf("history = %d — %s", rr.Code, rr.Body)
	}
	var hr struct {
		Messages []struct {
			Role string `json:"role"`
			Text string `json:"text"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &hr); err != nil {
		t.Fatalf("decode history: %v — %s", err, rr.Body)
	}
	// 2 student + 2 assistant, oldest→newest.
	if len(hr.Messages) != 4 {
		t.Fatalf("history messages = %d, want 4: %s", len(hr.Messages), rr.Body)
	}
	if hr.Messages[0].Role != "student" || hr.Messages[0].Text != "第一句" {
		t.Fatalf("msg[0] = %+v, want student/第一句", hr.Messages[0])
	}
	if hr.Messages[1].Role != "ai" {
		t.Fatalf("msg[1] role = %q, want ai", hr.Messages[1].Role)
	}
	if hr.Messages[2].Role != "student" || hr.Messages[2].Text != "第二句" {
		t.Fatalf("msg[2] = %+v, want student/第二句", hr.Messages[2])
	}
	if hr.Messages[3].Role != "ai" {
		t.Fatalf("msg[3] role = %q, want ai", hr.Messages[3].Role)
	}

	// A blank surface is a 400 (display always names a room).
	rrBad := httptest.NewRecorder()
	h.ServeHTTP(rrBad, withCookie(httptest.NewRequest("GET", base+"/coach/history", nil), cookie))
	if rrBad.Code != http.StatusBadRequest {
		t.Fatalf("history no-surface = %d, want 400", rrBad.Code)
	}
}

// TestCoach_ProposalSolidifyLeavesStudioThreadActive — Task 5 redesign: /coach
// turns live on the single continuous `studio` surface, so the legacy
// fold-on-solidify (which folds only forming/proposal_review) is intentionally
// INERT for the studio thread — the size-threshold compaction backstop is the
// sole folder now. A proposal save must therefore leave the studio turns active.
func TestCoach_ProposalSolidifyLeavesStudioThreadActive(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: fakeProvider(), ChatResolver: fakeResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)
	base := "/api/v1/projects/" + seedProjectID

	// A coach turn → one student + one assistant chat_message on the studio thread.
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", base+"/coach",
		strings.NewReader(`{"user_input":"这个题目我从哪儿下手"}`)), cookie))
	if rr.Code != http.StatusOK {
		t.Fatalf("coach = %d — %s", rr.Code, rr.Body)
	}
	if got := countCoachMsgs(t, pool, seedProjectID, "studio", true); got != 2 {
		t.Fatalf("active studio msgs before proposal = %d, want 2", got)
	}

	// First proposal save = solidify → folds forming/proposal_review, NOT studio.
	rrP := httptest.NewRecorder()
	h.ServeHTTP(rrP, withCookie(httptest.NewRequest("PUT", base+"/proposal",
		strings.NewReader(`{"objective":"论证中国是否让地球更可持续","reason":"我关心气候","activities":"读NASA/Nature","resources":"Zotero"}`)), cookie))
	if rrP.Code != http.StatusOK {
		t.Fatalf("put proposal = %d — %s", rrP.Code, rrP.Body)
	}

	// The studio thread stays active — solidify-fold does not touch it.
	if got := countCoachMsgs(t, pool, seedProjectID, "studio", true); got != 2 {
		t.Fatalf("active studio msgs after proposal = %d, want 2 (not folded by solidify)", got)
	}
}

// TestSummaryOnReturn — GET is null until composed; POST composes once
// (flagship) and stores; a second POST re-reads without a new spend.
func TestSummaryOnReturn(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider:     assessStubProvider("欢迎回来——你正在论证中国是否让地球更可持续，开题已经落定，下一步可以先把最强的反例找出来。"),
		ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)
	base := "/api/v1/projects/" + seedProjectID

	// Fill the proposal so the spine projection has content to summarise.
	rrP := httptest.NewRecorder()
	h.ServeHTTP(rrP, withCookie(httptest.NewRequest("PUT", base+"/proposal",
		strings.NewReader(`{"objective":"论证中国是否让地球更可持续","reason":"我关心气候","activities":"读NASA/Nature","resources":"Zotero"}`)), cookie))
	if rrP.Code != http.StatusOK {
		t.Fatalf("put proposal = %d — %s", rrP.Code, rrP.Body)
	}

	// GET before compose → JSON null.
	rrG := httptest.NewRecorder()
	h.ServeHTTP(rrG, withCookie(httptest.NewRequest("GET", base+"/summary", nil), cookie))
	if rrG.Code != http.StatusOK || strings.TrimSpace(rrG.Body.String()) != "null" {
		t.Fatalf("GET summary before compose = %d %q, want 200 null", rrG.Code, rrG.Body.String())
	}

	// POST composes once.
	rr1 := httptest.NewRecorder()
	h.ServeHTTP(rr1, withCookie(httptest.NewRequest("POST", base+"/summary", nil), cookie))
	if rr1.Code != http.StatusOK {
		t.Fatalf("POST summary = %d — %s", rr1.Code, rr1.Body)
	}
	var s1 struct {
		Prose string `json:"prose"`
	}
	if err := json.Unmarshal(rr1.Body.Bytes(), &s1); err != nil || strings.TrimSpace(s1.Prose) == "" {
		t.Fatalf("POST summary prose empty (err=%v): %s", err, rr1.Body)
	}

	// GET now returns the stored prose.
	rrG2 := httptest.NewRecorder()
	h.ServeHTTP(rrG2, withCookie(httptest.NewRequest("GET", base+"/summary", nil), cookie))
	if !strings.Contains(rrG2.Body.String(), `"prose"`) || strings.Contains(rrG2.Body.String(), "null") {
		t.Fatalf("GET summary after compose = %s, want stored prose", rrG2.Body)
	}

	// Second POST = first-open-wins, no new spend.
	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, withCookie(httptest.NewRequest("POST", base+"/summary", nil), cookie))
	if rr2.Code != http.StatusOK {
		t.Fatalf("second POST summary = %d — %s", rr2.Code, rr2.Body)
	}
	if n := countLLMCallsByPurpose(t, pool, seedProjectID, "summary"); n != 1 {
		t.Fatalf("summary llm_calls = %d, want 1 (compose once)", n)
	}
}
