package api_test

// atom_report_test.go — the two integration guarantees Task 4's generator
// makes: an unfinished atom must never spend or stamp, and two concurrent
// first-opens of a finished one must cost exactly one provider call. Both
// assert on the fake provider's CALL COUNT, never on timing — see
// writing_race_test.go's file comment for why.

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"mindimprint/api/internal/gateway"
)

// createReadingAtomHTTP mirrors createWritingAtomHTTP (writings_test.go) for
// the reading side, which no existing test file needed until now.
func createReadingAtomHTTP(t *testing.T, h http.Handler, cookie *http.Cookie, title string) string {
	t.Helper()
	rec := httptest.NewRecorder()
	body := strings.NewReader(`{"title":"` + title + `","lang":"zh"}`)
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/readings", body), cookie))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create reading = %d; body=%s", rec.Code, rec.Body)
	}
	var out struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || out.ID == "" {
		t.Fatalf("decode id: %v — body=%s", err, rec.Body)
	}
	return out.ID
}

func putReadingTakeawayHTTP(t *testing.T, h http.Handler, cookie *http.Cookie, id, text string) {
	t.Helper()
	rec := httptest.NewRecorder()
	body := strings.NewReader(`{"text":"` + text + `"}`)
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("PUT", "/api/v1/readings/"+id+"/takeaway", body), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("put takeaway = %d; body=%s", rec.Code, rec.Body)
	}
}

func finishReadingHTTP(t *testing.T, h http.Handler, cookie *http.Cookie, id string) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/readings/"+id+"/finish", nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("finish reading = %d; body=%s", rec.Code, rec.Body)
	}
}

func getReadingReportHTTP(h http.Handler, cookie *http.Cookie, id string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/readings/"+id+"/report", nil), cookie))
	return rec
}

// reportStubProvider scripts a valid moments+gains reply. The exact content
// does not matter to either test below — TestReportUnfinishedGeneratesNothing
// never reaches it, and TestReportChargesOnce only cares that it was called
// exactly once.
//
// The canned quote deliberately does NOT overlap finishedReadingID's
// takeaway text: after F4 (dedupeMomentsAgainstKeep), any moment that is a
// substring of `keep` (her takeaway, verbatim) is dropped post-generation —
// so a quote drawn from the takeaway would always vanish from the response.
// finishedReadingID plants a separate student chat message carrying this
// exact phrase so a moment quoting it survives BOTH validateMoments (a
// literal substring of the corpus) and dedupeMomentsAgainstKeep (not a
// substring of keep) — the shape TestPublicPayloadCarriesNothingExtra needs
// to see a non-empty `moments` key on the wire.
func reportStubProvider() *gateway.StubProvider {
	return gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: `{"moments":[{"quote":"数据来源要能查到出处","where":"和印记聊的时候"}],"gains":["她学会了先看信息的来源，而不是先信结论"]}`},
		{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 80, OutputTokens: 40}},
		{Kind: gateway.EventDone, StopReason: gateway.StopStop},
	})
}

// TestReportUnfinishedGeneratesNothing — the lesson sub-project A paid for:
// a generate-if-absent endpoint reachable early must not burn its one
// generation on partial data, permanently.
func TestReportUnfinishedGeneratesNothing(t *testing.T) {
	prov := &countingProvider{inner: reportStubProvider()}
	h, cookie, q, _ := liteHandlerWithProvider(t, prov)
	id := createReadingAtomHTTP(t, h, cookie, "还没读完的一篇")

	rec := getReadingReportHTTP(h, cookie, id)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET report = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var out struct {
		Report json.RawMessage `json:"report"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v — body=%s", err, rec.Body)
	}
	if string(out.Report) != "null" {
		t.Fatalf("report = %s, want null for an unfinished atom", out.Report)
	}
	if n := prov.count(); n != 0 {
		t.Fatalf("provider called %d times for an unfinished atom, want 0", n)
	}

	atomID := mustUUID(id)
	if _, err := q.GetAtomReport(context.Background(), atomID); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("GetAtomReport = %v, want pgx.ErrNoRows — nothing should have been stamped", err)
	}
}

// TestReportChargesOnce — two concurrent first-opens of a finished reading's
// report cost exactly one provider call. The advisory lock in
// ensureAtomReport is what makes the loser block before it reaches the
// provider; without it both requests read an empty atom_report, both
// generate, and both charge.
func TestReportChargesOnce(t *testing.T) {
	prov := &countingProvider{inner: reportStubProvider(), delay: 250 * time.Millisecond}
	h, cookie, _, _ := liteHandlerWithProvider(t, prov)
	id := createReadingAtomHTTP(t, h, cookie, "关于气候变化的一篇")
	putReadingTakeawayHTTP(t, h, cookie, id, "我觉得应该多看数据来源，而不是只看结论")
	finishReadingHTTP(t, h, cookie, id)

	var wg sync.WaitGroup
	recs := make([]*httptest.ResponseRecorder, 2)
	start := make(chan struct{})
	for i := range recs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			recs[i] = getReadingReportHTTP(h, cookie, id)
		}(i)
	}
	close(start)
	wg.Wait()

	for i, rec := range recs {
		if rec.Code != http.StatusOK {
			t.Fatalf("report[%d] = %d, want 200; body=%s", i, rec.Code, rec.Body)
		}
	}
	if n := prov.count(); n != 1 {
		t.Fatalf("model called %d times for two concurrent first-opens, want 1", n)
	}
}
