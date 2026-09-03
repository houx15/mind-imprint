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

// TestReportFirstOpenCostsNothing — the FIRST open of a finished reading's
// report makes no provider call at all, and still returns a real report.
//
// This is the 2026-09-03 two-phase split (see ensureAtomReport). The bug it
// fixes was reported as 「印记正在把这次读的东西整理成一份报告，稍等一下。—
// it never finishes」: the one model call is an `assess`-class call, i.e.
// `reasoning: "max"` with a 180s latency budget, and it used to run INSIDE
// this GET, whose own ctx is capped at 150s. So she waited up to two and a
// half minutes on a static grey sentence and, on a long session, the cap
// fired first and killed the transaction — no report was ever stored.
//
// What is pinned here is the property that makes the waiting stop: the
// deterministic half is committed and served without ever reaching a
// provider.
func TestReportFirstOpenCostsNothing(t *testing.T) {
	prov := &countingProvider{inner: reportStubProvider()}
	h, cookie, _, _ := liteHandlerWithProvider(t, prov)
	id := createReadingAtomHTTP(t, h, cookie, "关于气候变化的一篇")
	putReadingTakeawayHTTP(t, h, cookie, id, "我觉得应该多看数据来源，而不是只看结论")
	finishReadingHTTP(t, h, cookie, id)

	rec := getReadingReportHTTP(h, cookie, id)
	if rec.Code != http.StatusOK {
		t.Fatalf("first open = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	if n := prov.count(); n != 0 {
		t.Fatalf("first open called the model %d times, want 0 — she must not wait on it", n)
	}

	var out struct {
		Report struct {
			Stats []struct {
				Key string `json:"key"`
			} `json:"stats"`
			Keep         *struct{ Text string } `json:"keep"`
			ProsePending bool                   `json:"prosePending"`
		} `json:"report"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v — body=%s", err, rec.Body)
	}
	// A real report, not a placeholder: this is the whole point of phase 1.
	if len(out.Report.Stats) == 0 {
		t.Errorf("phase 1 must carry her stats — body=%s", rec.Body)
	}
	// Her OWN takeaway needs no model, so 我的收获 is populated on the very
	// first open.
	if out.Report.Keep == nil || out.Report.Keep.Text == "" {
		t.Errorf("phase 1 must carry her own 收获 — body=%s", rec.Body)
	}
	if !out.Report.ProsePending {
		t.Errorf("phase 1 must flag prosePending so the client asks again — body=%s", rec.Body)
	}
}

// TestReportSecondOpenAddsProseOnce — the follow-up request is what pays for
// the prose, and it pays exactly once: a third open serves the stored,
// completed report without going near the provider.
func TestReportSecondOpenAddsProseOnce(t *testing.T) {
	prov := &countingProvider{inner: reportStubProvider()}
	h, cookie, _, _ := liteHandlerWithProvider(t, prov)
	id := createReadingAtomHTTP(t, h, cookie, "关于气候变化的一篇")
	putReadingTakeawayHTTP(t, h, cookie, id, "我觉得应该多看数据来源，而不是只看结论")
	finishReadingHTTP(t, h, cookie, id)

	if rec := getReadingReportHTTP(h, cookie, id); rec.Code != http.StatusOK {
		t.Fatalf("first open = %d; body=%s", rec.Code, rec.Body)
	}
	second := getReadingReportHTTP(h, cookie, id)
	if second.Code != http.StatusOK {
		t.Fatalf("second open = %d; body=%s", second.Code, second.Body)
	}
	if n := prov.count(); n != 1 {
		t.Fatalf("model called %d times across two opens, want 1", n)
	}

	var out struct {
		Report struct {
			ProsePending bool `json:"prosePending"`
		} `json:"report"`
	}
	if err := json.Unmarshal(second.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v — body=%s", err, second.Body)
	}
	if out.Report.ProsePending {
		t.Errorf("after the prose lands the report must stop asking to be re-fetched — body=%s", second.Body)
	}

	// 🚨 The idempotency that matters for cost: reopening a finished report
	// forever after is free.
	if rec := getReadingReportHTTP(h, cookie, id); rec.Code != http.StatusOK {
		t.Fatalf("third open = %d; body=%s", rec.Code, rec.Body)
	}
	if n := prov.count(); n != 1 {
		t.Fatalf("model called %d times across three opens, want 1", n)
	}
}

// TestReportThinSessionStopsAskingToBeRefetched — 🚨 the stuck-flag case,
// found by the first live walk (2026-09-04) rather than by any test.
//
// A reading finished with no takeaway, no notes and nothing she said has an
// EMPTY CORPUS, so generateReportProse correctly makes no model call at all.
// The first cut of the two-phase split keyed the pending flag off "did we get
// any moments or gains", which made that indistinguishable from "the call
// failed" — so the report stayed `prosePending` forever: every open
// re-attempted, the client re-fetched every time, and the 处理中 line never
// went away on a report that was already as complete as it would ever be.
//
// The flag now follows reportProse.Retryable: nothing to work from is
// TERMINAL, and only a genuine failure is worth another try.
func TestReportThinSessionStopsAskingToBeRefetched(t *testing.T) {
	prov := &countingProvider{inner: reportStubProvider()}
	h, cookie, _, _ := liteHandlerWithProvider(t, prov)
	// No takeaway, no notes, no chat — deliberately the thinnest finishable
	// reading there is.
	id := createReadingAtomHTTP(t, h, cookie, "关于气候变化的一篇")
	finishReadingHTTP(t, h, cookie, id)

	if rec := getReadingReportHTTP(h, cookie, id); rec.Code != http.StatusOK {
		t.Fatalf("phase 1 = %d; body=%s", rec.Code, rec.Body)
	}
	second := getReadingReportHTTP(h, cookie, id)
	if second.Code != http.StatusOK {
		t.Fatalf("phase 2 = %d; body=%s", second.Code, second.Body)
	}
	var out struct {
		Report struct {
			ProsePending bool `json:"prosePending"`
		} `json:"report"`
	}
	if err := json.Unmarshal(second.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v — body=%s", err, second.Body)
	}
	if out.Report.ProsePending {
		t.Fatalf("a report with nothing to write prose about must stop asking to be re-fetched — body=%s", second.Body)
	}
	// And it never reached a provider: there was nothing to ask about.
	if n := prov.count(); n != 0 {
		t.Errorf("empty corpus called the model %d times, want 0", n)
	}
}

// TestReportChargesOnce — two concurrent opens of a finished reading's
// report whose prose is still pending cost exactly one provider call. The
// advisory lock plus the re-read under it are what make the loser drop its
// own generation rather than overwrite the winner's.
func TestReportChargesOnce(t *testing.T) {
	prov := &countingProvider{inner: reportStubProvider(), delay: 250 * time.Millisecond}
	h, cookie, _, _ := liteHandlerWithProvider(t, prov)
	id := createReadingAtomHTTP(t, h, cookie, "关于气候变化的一篇")
	putReadingTakeawayHTTP(t, h, cookie, id, "我觉得应该多看数据来源，而不是只看结论")
	finishReadingHTTP(t, h, cookie, id)
	// Phase 1 first, so both racers below are phase-2 requests — that is the
	// path with a model call in it, and therefore the only one that can
	// double-charge. (Phase 1 cannot: it makes no provider call at all, which
	// TestReportFirstOpenCostsNothing pins separately.)
	if rec := getReadingReportHTTP(h, cookie, id); rec.Code != http.StatusOK {
		t.Fatalf("phase 1 = %d; body=%s", rec.Code, rec.Body)
	}

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
