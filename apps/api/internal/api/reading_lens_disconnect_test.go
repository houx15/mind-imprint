package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"mindimprint/api/internal/gateway"
)

// reading_lens_disconnect_test.go — the same regression guard
// reading_turn_disconnect_test.go holds over the coach turn, applied to the
// two STUDENT-driven endpoints in reading_lens.go.
//
// /evaluate is the most abandonable call in the whole product: measured at
// 45–62 seconds, and it is the moment she is staring at a sentence deciding
// whether it was the right one. A refresh at second 50 on the pre-fix code
// cancelled r.Context() and with it (a) the flagship model call, (b) the
// llm_call metering row and (c) the framework_fill write — money spent, and
// under 铁律④ a piece of evidence the P2 report is generated from silently
// never recorded.
//
// Both tests reuse disconnectSignalProvider (reading_turn_disconnect_test.go):
// it blocks inside Stream until the test has cancelled the REQUEST context,
// then emits its script only if the context it was actually handed is still
// alive. So it answers exactly one question — did the handler run this on
// r.Context() or on its own detached turn context?

const disconnectStudentQuote = "但同一时期，中国的碳排放总量仍居全球第一。"

// postEvaluate fires the 选句复核 with the request context under the caller's
// control, so the test can cancel it mid-model-call.
func postEvaluateCtx(ctx context.Context, h http.Handler, cookie *http.Cookie, id, cid string) (*httptest.ResponseRecorder, *http.Request) {
	body := `{"block_id":"b2","start":0,"end":` +
		strconv.Itoa(len([]rune(disconnectStudentQuote))) +
		`,"quote":"` + disconnectStudentQuote + `","dimension":"currency"}`
	req := withCookie(httptest.NewRequest("POST",
		"/api/v1/readings/"+id+"/cards/"+cid+"/evaluate", strings.NewReader(body)), cookie).WithContext(ctx)
	return httptest.NewRecorder(), req
}

// disconnectScript is routerStubProvider's event script, hoisted so
// disconnectSignalProvider can replay the SAME three events behind its
// blocking gate.
func disconnectScript(jsonOut string) []gateway.StreamEvent {
	return []gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: jsonOut},
		{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 120, OutputTokens: 40}},
		{Kind: gateway.EventDone, StopReason: gateway.StopStop},
	}
}

// TestEvaluateSelection_SurvivesClientDisconnect — the Critical one. After the
// student's browser goes away mid-review, the review she already paid for must
// still be metered AND persisted on the card row.
func TestEvaluateSelection_SurvivesClientDisconnect(t *testing.T) {
	prov := &disconnectSignalProvider{
		script:  disconnectScript(evalScript),
		started: make(chan struct{}), unblock: make(chan struct{}),
	}
	h, cookie, _, pool := liteHandlerWithProvider(t, prov)
	id := createReadingAtom(t, h, cookie)
	if rec := putSource(t, h, cookie, id, turnArticle); rec.Code != http.StatusOK {
		t.Fatalf("put source = %d; body=%s", rec.Code, rec.Body)
	}
	cid := seedCard(t, pool, id, "b1")

	reqCtx, cancelReq := context.WithCancel(context.Background())
	rec, req := postEvaluateCtx(reqCtx, h, cookie, id, cid)

	done := make(chan struct{})
	go func() {
		h.ServeHTTP(rec, req)
		close(done)
	}()

	select {
	case <-prov.started:
	case <-time.After(5 * time.Second):
		t.Fatal("provider was never reached")
	}
	cancelReq()
	close(prov.unblock)

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("handler never returned")
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("evaluate after client disconnect = %d, want 200; body=%s", rec.Code, rec.Body)
	}

	var n int
	if err := pool.QueryRow(t.Context(),
		`SELECT count(*) FROM llm_call WHERE atom_id = $1 AND purpose = 'read_eval'`,
		mustUUID(id)).Scan(&n); err != nil {
		t.Fatalf("count llm_call: %v", err)
	}
	if n != 1 {
		t.Fatalf("read_eval llm_call rows = %d, want 1 — a flagship call that reached a "+
			"provider costs money whether or not she is still on the page", n)
	}

	var raw []byte
	if err := pool.QueryRow(t.Context(),
		`SELECT framework_fill FROM atom_card WHERE id = $1`, mustUUID(cid)).Scan(&raw); err != nil {
		t.Fatalf("read framework_fill: %v", err)
	}
	var stored struct {
		Finding  string `json:"finding"`
		Degraded bool   `json:"degraded"`
	}
	if err := json.Unmarshal(raw, &stored); err != nil {
		t.Fatalf("decode framework_fill: %v — raw=%s", err, raw)
	}
	if stored.Finding != "这句给出了与全文乐观基调相反的事实。" {
		t.Fatalf("framework_fill.finding = %q, want the model's real review persisted "+
			"despite the disconnect (raw=%s)", stored.Finding, raw)
	}
	if stored.Degraded {
		t.Fatalf("framework_fill.degraded = true, want false — the model answered; "+
			"raw=%s", raw)
	}
}

// TestSummonCard_SurvivesClientDisconnect — the 透镜库 half. Cheaper than
// /evaluate but the same three losses: the grounding call is billed, and the
// card row she asked for is the thing the whole loop hangs on.
func TestSummonCard_SurvivesClientDisconnect(t *testing.T) {
	prov := &disconnectSignalProvider{
		script:  disconnectScript(exampleScript),
		started: make(chan struct{}), unblock: make(chan struct{}),
	}
	h, cookie, _, pool := liteHandlerWithProvider(t, prov)
	id := createReadingAtom(t, h, cookie)
	if rec := putSource(t, h, cookie, id, turnArticle); rec.Code != http.StatusOK {
		t.Fatalf("put source = %d; body=%s", rec.Code, rec.Body)
	}

	reqCtx, cancelReq := context.WithCancel(context.Background())
	req := withCookie(httptest.NewRequest("POST", "/api/v1/readings/"+id+"/summon",
		strings.NewReader(`{"cardId":"craap"}`)), cookie).WithContext(reqCtx)
	rec := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		h.ServeHTTP(rec, req)
		close(done)
	}()

	select {
	case <-prov.started:
	case <-time.After(5 * time.Second):
		t.Fatal("provider was never reached")
	}
	cancelReq()
	close(prov.unblock)

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("handler never returned")
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("summon after client disconnect = %d, want 200; body=%s", rec.Code, rec.Body)
	}

	var cards int
	if err := pool.QueryRow(t.Context(),
		`SELECT count(*) FROM atom_card WHERE atom_id = $1`, mustUUID(id)).Scan(&cards); err != nil {
		t.Fatalf("count atom_card: %v", err)
	}
	if cards != 1 {
		t.Fatalf("atom_card rows = %d, want 1 — the lens she asked for must be minted "+
			"even if she navigated away while it was being grounded", cards)
	}

	var n int
	if err := pool.QueryRow(t.Context(),
		`SELECT count(*) FROM llm_call WHERE atom_id = $1 AND purpose = 'read_card_example'`,
		mustUUID(id)).Scan(&n); err != nil {
		t.Fatalf("count llm_call: %v", err)
	}
	if n != 1 {
		t.Fatalf("read_card_example llm_call rows = %d, want 1", n)
	}
}
