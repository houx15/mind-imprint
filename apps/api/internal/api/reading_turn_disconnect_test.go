package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"mindimprint/api/internal/gateway"
)

// reading_turn_disconnect_test.go — proves postLiteReadingTurn's work is
// independent of the REQUEST's context, i.e. that reading_turn.go really does
// run on turnCtx = context.WithTimeout(context.WithoutCancel(r.Context()),
// 150*time.Second) rather than r.Context() itself.
//
// httptest cannot simulate a real TCP disconnect, but it does not need to:
// net/http's only observable effect of a disconnect is cancelling the
// request's context. So this test cancels the request context itself, at the
// precise moment execution has moved past everything that legitimately still
// runs on r.Context() (session auth, loadOwnedReadingAtom, HasEntitlement)
// and into the model call — which is exactly the span the fix protects.

// disconnectSignalProvider blocks inside Stream until the test says "go",
// first announcing (via started) that it has been reached. The events are
// then emitted only if the ctx it was actually called with is NOT yet
// cancelled at that moment — checked once, synchronously, right after the
// test's cancellation has already happened-before the unblock signal, so the
// outcome is deterministic rather than a select-on-two-ready-cases race.
//
// This mirrors a real provider: a client whose underlying request context is
// already cancelled gets nothing back, not a partial reply.
//
// agent.RouteReading retries once on an unparseable/empty reply, so Stream
// may be called a second time (exactly what happens against the buggy
// r.Context() version, whose first attempt yields no text at all). started is
// therefore only closed once (sync.Once); unblock, once closed, lets every
// subsequent receive through immediately.
type disconnectSignalProvider struct {
	script      []gateway.StreamEvent
	started     chan struct{}
	startedOnce sync.Once
	unblock     chan struct{}
}

func (p *disconnectSignalProvider) Stream(ctx context.Context, _ gateway.Resolved, _ gateway.ChatRequest) (<-chan gateway.StreamEvent, error) {
	p.startedOnce.Do(func() { close(p.started) })
	<-p.unblock
	out := make(chan gateway.StreamEvent, len(p.script))
	if ctx.Err() != nil {
		close(out)
		return out, nil
	}
	for _, ev := range p.script {
		out <- ev
	}
	close(out)
	return out, nil
}

// TestReadingTurn_SurvivesClientDisconnect — regression guard for the fix at
// reading_turn.go:316-317 (mirroring coach.go:138-142). Before that fix the
// turn ran on r.Context(): a client disconnect mid-call would cancel the
// model call AND the transaction below it, so a turn that had already been
// billed (the flagship call happened) would leave NEITHER the transcript NOR
// the llm_call metering row behind — money spent, nothing recorded.
func TestReadingTurn_SurvivesClientDisconnect(t *testing.T) {
	script := []gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: `{"decision":"respond","reply":"没事，我还在，接着说。"}`},
		{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 111, OutputTokens: 22}},
		{Kind: gateway.EventDone, StopReason: gateway.StopStop},
	}
	prov := &disconnectSignalProvider{script: script, started: make(chan struct{}), unblock: make(chan struct{})}
	h, cookie, _, pool := liteHandlerWithProvider(t, prov)
	id := createReadingAtom(t, h, cookie)
	if rec := putSource(t, h, cookie, id, turnArticle); rec.Code != http.StatusOK {
		t.Fatalf("put source = %d; body=%s", rec.Code, rec.Body)
	}

	reqCtx, cancelReq := context.WithCancel(context.Background())
	req := withCookie(httptest.NewRequest("POST", "/api/v1/readings/"+id+"/turn",
		strings.NewReader(`{"text":"这篇在说什么"}`)), cookie).WithContext(reqCtx)

	rec := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		h.ServeHTTP(rec, req)
		close(done)
	}()

	// Wait until the handler has reached the model call — proving session
	// auth, atom ownership and the entitlement check (all legitimately on
	// r.Context()) already succeeded — then simulate the client disconnecting
	// exactly as net/http would on a refresh or tab-close: cancel the
	// request's own context.
	select {
	case <-prov.started:
	case <-time.After(5 * time.Second):
		t.Fatal("provider was never reached")
	}
	cancelReq()
	close(prov.unblock)

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("handler never returned")
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("turn after client disconnect = %d, want 200; body=%s", rec.Code, rec.Body)
	}

	msgs := listMessages(t, h, cookie, id)
	if len(msgs) != 2 {
		t.Fatalf("messages after disconnect = %d, want 2 (student+ai persisted despite the request context dying mid-call) — %+v", len(msgs), msgs)
	}

	var n int
	if err := pool.QueryRow(t.Context(),
		`SELECT count(*) FROM llm_call WHERE atom_id = $1`, mustUUID(id)).Scan(&n); err != nil {
		t.Fatalf("count llm_call: %v", err)
	}
	if n != 1 {
		t.Fatalf("llm_call rows = %d, want 1 — the metered call must survive the disconnect too", n)
	}
}
