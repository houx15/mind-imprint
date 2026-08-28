package api_test

// reading_questions_test.go — Task 8's HTTP-level test: the single-charge
// guarantee for GET /readings/{id}/questions. validateReadingQuestions
// itself (the anchorQuote substring guarantee) is tested separately in
// reading_questions_internal_test.go (package api — it is unexported).
//
// Reuses countingProvider from writing_race_test.go (same package, same test
// binary) — that file is this repo's reference implementation of exactly
// this race shape: two concurrent first-opens, one provider call.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

const readingQuestionsTestBody = "中国的碳排放总量位居世界第一。\n\n但人均排放仍低于多数发达国家。\n\n这使得责任的衡量方式成为一个持续的争论。"

const readingQuestionsTestReply = `{"questions":[` +
	`{"text":"总量第一意味着什么样的责任？","anchorQuote":"中国的碳排放总量位居世界第一"},` +
	`{"text":"人均排放和总量，哪个更该被用来衡量责任？","anchorQuote":"人均排放仍低于多数发达国家"}]}`

// readingQuestionsThinTestBody/Reply is the "fewer than two survive" case —
// Task 8 fix round 1's regression fixture. Fewer than two anchorable
// questions is exactly what a thin article, or a model that reaches for a
// generic question, produces: the SECOND draft's anchorQuote ("环境保护很重要")
// is not a substring of the body, so validateReadingQuestions drops it and
// only one survivor is left — below the 2-survivor floor, so the endpoint
// must show NONE.
const readingQuestionsThinTestBody = "全球气温在过去五十年持续上升。"

const readingQuestionsThinTestReply = `{"questions":[` +
	`{"text":"气温上升的原因是什么？","anchorQuote":"全球气温在过去五十年持续上升"},` +
	`{"text":"你怎么看待环保？","anchorQuote":"环境保护很重要"}]}`

type readingQuestionDTOForTest struct {
	ID          string `json:"id"`
	Text        string `json:"text"`
	AnchorQuote string `json:"anchorQuote"`
	AnchorBlock string `json:"anchorBlock"`
}

func getReadingQuestionsHTTP(t *testing.T, h http.Handler, cookie *http.Cookie, id string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/readings/"+id+"/questions", nil), cookie))
	return rec
}

// TestReadingQuestionsChargesOnce — two concurrent first-opens of a finished
// reading with no stored questions must cost exactly ONE provider call. The
// advisory lock in getReadingQuestions (reading_questions.go) is taken
// BEFORE the provider call, so the loser blocks, wakes after the winner's
// commit, and re-reads the winner's rows instead of ever reaching the model.
//
// Not timing-dependent: whichever order the two requests land in, the answer
// is one call — if the second arrives during the first's model call it
// blocks on the lock; if it arrives after the commit its own cheap pre-check
// (outside any lock) already finds the stored rows.
func TestReadingQuestionsChargesOnce(t *testing.T) {
	prov := &countingProvider{inner: readingStubProvider(readingQuestionsTestReply), delay: 250 * time.Millisecond}
	h, cookie, _, _ := liteHandlerWithProvider(t, prov)
	id := createReadingAtom(t, h, cookie)
	if rec := putSource(t, h, cookie, id, readingQuestionsTestBody); rec.Code != http.StatusOK {
		t.Fatalf("put source = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	finishReadingAtom(t, h, cookie, id)

	var wg sync.WaitGroup
	recs := make([]*httptest.ResponseRecorder, 2)
	start := make(chan struct{})
	for i := range recs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			recs[i] = getReadingQuestionsHTTP(t, h, cookie, id)
		}(i)
	}
	close(start)
	wg.Wait()

	idSets := make([][]string, 2)
	for i, rec := range recs {
		if rec.Code != http.StatusOK {
			t.Fatalf("questions[%d] = %d, want 200; body=%s", i, rec.Code, rec.Body)
		}
		var out struct {
			Questions []readingQuestionDTOForTest `json:"questions"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("decode questions[%d]: %v — body=%s", i, err, rec.Body)
		}
		if len(out.Questions) != 2 {
			t.Fatalf("questions[%d] returned %d, want the 2 survivors: %+v", i, len(out.Questions), out.Questions)
		}
		ids := make([]string, len(out.Questions))
		for j, q := range out.Questions {
			if q.AnchorQuote == "" {
				t.Fatalf("questions[%d][%d] has no anchorQuote: %+v", i, j, q)
			}
			ids[j] = q.ID
		}
		idSets[i] = ids
	}
	if idSets[0][0] != idSets[1][0] || idSets[0][1] != idSets[1][1] {
		t.Fatalf("the two racers saw DIFFERENT question ids: %v vs %v — a second generation happened", idSets[0], idSets[1])
	}

	if n := prov.count(); n != 1 {
		t.Fatalf("model called %d times for one first-open race, want 1 — a concurrent double-open is billed twice", n)
	}
}

// TestReadingQuestionsThinArticle_NoRetryOnReopen — Task 8 fix round 1's
// regression test. Before the fix, "generated" was inferred from
// reading_question having rows; a thin article that validates down to ZERO
// survivors leaves no rows, which is indistinguishable from "never
// generated" — so every reopen of the finished reading's screen re-called
// the flagship model, forever, showing nothing each time. The fix
// (reading.questions_at, migration 0104) records the ATTEMPT itself,
// independent of the outcome, so a second open must NOT call the model
// again.
func TestReadingQuestionsThinArticle_NoRetryOnReopen(t *testing.T) {
	prov := &countingProvider{inner: readingStubProvider(readingQuestionsThinTestReply)}
	h, cookie, _, _ := liteHandlerWithProvider(t, prov)
	id := createReadingAtom(t, h, cookie)
	if rec := putSource(t, h, cookie, id, readingQuestionsThinTestBody); rec.Code != http.StatusOK {
		t.Fatalf("put source = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	finishReadingAtom(t, h, cookie, id)

	first := getReadingQuestionsHTTP(t, h, cookie, id)
	if first.Code != http.StatusOK {
		t.Fatalf("first open = %d, want 200; body=%s", first.Code, first.Body)
	}
	var firstOut struct {
		Questions []readingQuestionDTOForTest `json:"questions"`
	}
	if err := json.Unmarshal(first.Body.Bytes(), &firstOut); err != nil {
		t.Fatalf("decode first open: %v — body=%s", err, first.Body)
	}
	if len(firstOut.Questions) != 0 {
		t.Fatalf("first open returned %d questions, want 0 (only 1 of 2 drafts survives validation): %+v",
			len(firstOut.Questions), firstOut.Questions)
	}
	if n := prov.count(); n != 1 {
		t.Fatalf("model called %d times on first open, want 1", n)
	}

	// Reopen — the room's finished-reading screen can be mounted any number
	// of times from her history. This must be free.
	second := getReadingQuestionsHTTP(t, h, cookie, id)
	if second.Code != http.StatusOK {
		t.Fatalf("second open = %d, want 200; body=%s", second.Code, second.Body)
	}
	var secondOut struct {
		Questions []readingQuestionDTOForTest `json:"questions"`
	}
	if err := json.Unmarshal(second.Body.Bytes(), &secondOut); err != nil {
		t.Fatalf("decode second open: %v — body=%s", err, second.Body)
	}
	if len(secondOut.Questions) != 0 {
		t.Fatalf("second open returned %d questions, want 0: %+v", len(secondOut.Questions), secondOut.Questions)
	}
	if n := prov.count(); n != 1 {
		t.Fatalf("model called %d times after a second open of a thin-article reading, want 1 — "+
			"a zero-survivor outcome must not look like \"never generated\" and re-trigger the model", n)
	}
}
