package voice

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/binary"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// fakeASRServerFrame builds a SERVER_FULL_RESPONSE ASR frame carrying a
// gzip+JSON payload, mirroring the wire layout exercised by
// TestParseASRServerResponse in asr_protocol_test.go. flags is the header
// byte1 low nibble (message-type-specific-flags); pass 0b0010 to mark the
// frame as the last package.
func fakeASRServerFrame(t *testing.T, payload []byte, flags byte) []byte {
	t.Helper()

	var gz bytes.Buffer
	gw := gzip.NewWriter(&gz)
	if _, err := gw.Write(payload); err != nil {
		t.Fatalf("gzip write: %v", err)
	}
	if err := gw.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	compressed := gz.Bytes()

	var buf bytes.Buffer
	buf.WriteByte(0x11)                 // version=1, header_size=1
	buf.WriteByte((0b1001 << 4) | flags) // SERVER_FULL_RESPONSE
	buf.WriteByte(0x11)                 // JSON serialization, GZIP compression
	buf.WriteByte(0x00)                 // reserved

	sizeBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(sizeBytes, uint32(len(compressed)))
	buf.Write(sizeBytes)
	buf.Write(compressed)

	return buf.Bytes()
}

// fakeVolcanoASR mimics Volcano's ASR websocket: reads the full-client init
// request, replies with a code-0 init response, reads at least one audio
// frame, then emits one partial and one final (IsLast) transcript frame.
func fakeVolcanoASR(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		defer c.Close(websocket.StatusNormalClosure, "")
		ctx := r.Context()

		if _, _, err := c.Read(ctx); err != nil { // full-client request
			return
		}
		if err := c.Write(ctx, websocket.MessageBinary, fakeASRServerFrame(t, []byte(`{}`), 0)); err != nil {
			return
		}

		if _, _, err := c.Read(ctx); err != nil { // >=1 audio-only frame
			return
		}

		partial := []byte(`{"result":{"text":"你","is_final":false}}`)
		if err := c.Write(ctx, websocket.MessageBinary, fakeASRServerFrame(t, partial, 0)); err != nil {
			return
		}
		final := []byte(`{"result":{"text":"你好","is_final":true}}`)
		if err := c.Write(ctx, websocket.MessageBinary, fakeASRServerFrame(t, final, 0b0010)); err != nil {
			return
		}
	}))
}

func TestASRStream(t *testing.T) {
	srv := fakeVolcanoASR(t)
	defer srv.Close()
	asrWSURL = "ws" + strings.TrimPrefix(srv.URL, "http")

	c := New(Config{AppID: "a", AccessKey: "k", ASRResourceID: "volc.bigasr.sauc.duration"})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	audioIn := make(chan []byte, 1)
	audioIn <- []byte{0x00, 0x01, 0x02, 0x03}
	close(audioIn)

	out, err := c.Stream(ctx, audioIn)
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}

	var got []Transcript
	for tr := range out {
		got = append(got, tr)
	}

	want := []Transcript{
		{Text: "你", Final: false},
		{Text: "你好", Final: true},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d transcripts %#v, want %d %#v", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("transcript[%d] = %#v, want %#v", i, got[i], want[i])
		}
	}
}

// goroutineLeakSlack is the tolerance applied when comparing a post-Stream
// goroutine count back to a pre-Stream baseline. It is 0: the leaked
// asrSend goroutine this file guards against is exactly one extra
// goroutine parked forever on <-audioIn, and any nonzero slack here would
// mask that regression (verified by temporarily reverting the fix: with
// slack 0 this poll reliably observes baseline+1 and times out, whereas
// with the fix it reliably settles back to exactly baseline).
const goroutineLeakSlack = 0

// waitNoGoroutineLeak polls runtime.NumGoroutine() for up to 2s and fails
// the test if the count never settles back down to baseline (+/- slack).
// A single immediate sample is flaky here because cancelSend (and thus
// asrSend's exit) races the scheduler relative to out closing.
func waitNoGoroutineLeak(t *testing.T, baseline int) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	var last int
	for time.Now().Before(deadline) {
		last = runtime.NumGoroutine()
		if last <= baseline+goroutineLeakSlack {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	buf := make([]byte, 1<<16)
	n := runtime.Stack(buf, true)
	t.Logf("goroutine dump on leak failure:\n%s", buf[:n])
	t.Fatalf("goroutine leak: baseline=%d, still elevated=%d after 2s (asrSend likely never exited)", baseline, last)
}

// fakeVolcanoASRServerEndsFirst mimics a Volcano ASR session where the
// server ends the stream (IsLast) immediately after the init ack, before
// the client has sent or closed any audio. It then holds the connection
// open (bounded by its own timeout) rather than closing right away, so the
// test's asrReceive exit path is driven purely by the IsLast frame, not by
// a subsequent read error.
func fakeVolcanoASRServerEndsFirst(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		defer c.Close(websocket.StatusNormalClosure, "")
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		if _, _, err := c.Read(ctx); err != nil { // full-client request
			return
		}
		if err := c.Write(ctx, websocket.MessageBinary, fakeASRServerFrame(t, []byte(`{}`), 0)); err != nil {
			return
		}

		final := []byte(`{"result":{"text":"done","is_final":true}}`)
		if err := c.Write(ctx, websocket.MessageBinary, fakeASRServerFrame(t, final, 0b0010)); err != nil {
			return
		}

		// Hold the conn open past IsLast rather than closing it ourselves
		// (as a real server that has decided the session is over, but
		// hasn't yet torn down the socket, might): the client's own
		// asrReceive is what's expected to close its side once it sees
		// IsLast. Blocking on Read (rather than ctx.Done()) lets that
		// client-side close unblock this handler promptly instead of
		// forcing it to sit around for the full bounded timeout.
		c.Read(ctx)
	}))
}

// TestASRStreamServerEndsFirstNoLeak is a regression test for a goroutine
// leak: when the server sends its IsLast frame before the caller has
// finished (or closed) audioIn, and the caller's ctx is never cancelled,
// asrReceive used to exit while asrSend stayed parked forever on
// <-audioIn — closing conn does not unblock a channel receive. Stream now
// derives an internal sendCtx that is cancelled the moment the reader
// exits, independent of the outer ctx and of audioIn.
func TestASRStreamServerEndsFirstNoLeak(t *testing.T) {
	srv := fakeVolcanoASRServerEndsFirst(t)
	defer srv.Close()
	asrWSURL = "ws" + strings.TrimPrefix(srv.URL, "http")

	c := New(Config{AppID: "a", AccessKey: "k", ASRResourceID: "volc.bigasr.sauc.duration"})

	// The outer ctx intentionally outlives the test body: the leak this
	// guards against only manifests when nothing external cancels ctx.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// audioIn is intentionally never closed and nothing is ever sent on
	// it — the exact condition under which the old asrSend would block
	// forever once the reader exited on the server's IsLast frame.
	audioIn := make(chan []byte)

	// Let any already-running background goroutines settle before taking
	// the baseline sample.
	runtime.Gosched()
	time.Sleep(20 * time.Millisecond)
	baseline := runtime.NumGoroutine()

	out, err := c.Stream(ctx, audioIn)
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}

	for range out {
		// Drain until asrReceive closes out on the server's IsLast frame.
	}

	waitNoGoroutineLeak(t, baseline)
}

// fakeVolcanoASRHold mimics a Volcano ASR session that acks the init
// request, reads one audio frame, and then goes quiet — it neither emits a
// transcript nor an IsLast frame, so the stream can only end via the
// client's ctx cancellation (mirroring a real mid-conversation hang-up).
func fakeVolcanoASRHold(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		defer c.Close(websocket.StatusNormalClosure, "")
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		if _, _, err := c.Read(ctx); err != nil { // full-client request
			return
		}
		if err := c.Write(ctx, websocket.MessageBinary, fakeASRServerFrame(t, []byte(`{}`), 0)); err != nil {
			return
		}

		if _, _, err := c.Read(ctx); err != nil { // one audio-only frame
			return
		}

		// Go quiet: block on a read that only returns once the client
		// disconnects (its ctx cancellation) or our own bounded ctx times
		// out, whichever comes first.
		c.Read(ctx)
	}))
}

// TestASRStreamCtxCancelMidStream asserts that cancelling the caller's ctx
// mid-stream — with audioIn never closed and one chunk already queued —
// still closes out promptly and leaves no goroutines behind. This is the
// companion path to TestASRStreamServerEndsFirstNoLeak: here the outer ctx
// is the thing that ends the session, and both asrSend (via sendCtx, a
// child of ctx) and asrReceive (via ctx directly) must observe it.
func TestASRStreamCtxCancelMidStream(t *testing.T) {
	srv := fakeVolcanoASRHold(t)
	defer srv.Close()
	asrWSURL = "ws" + strings.TrimPrefix(srv.URL, "http")

	c := New(Config{AppID: "a", AccessKey: "k", ASRResourceID: "volc.bigasr.sauc.duration"})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	audioIn := make(chan []byte, 1)
	audioIn <- []byte{0x00, 0x01, 0x02, 0x03}
	// Intentionally never closed: asrSend must exit via ctx cancellation,
	// not via a closed audioIn.

	runtime.Gosched()
	time.Sleep(20 * time.Millisecond)
	baseline := runtime.NumGoroutine()

	out, err := c.Stream(ctx, audioIn)
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}

	// Give asrSend a moment to deliver the queued chunk before cancelling,
	// so the cancellation genuinely lands mid-stream rather than before
	// anything has been sent.
	time.Sleep(50 * time.Millisecond)
	cancel()

	deadline := time.After(5 * time.Second)
drain:
	for {
		select {
		case _, ok := <-out:
			if !ok {
				break drain
			}
		case <-deadline:
			t.Fatal("out did not close within 5s of ctx cancellation")
		}
	}

	waitNoGoroutineLeak(t, baseline)
}
