package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/store/sqlc"
	"mindimprint/api/internal/voice"
)

// asrStub is a VoiceService whose ASRStream ignores audioIn and just emits a
// fixed partial-then-final transcript pair, so this test can exercise the
// forward loop without needing real audio semantics. Synthesize/Voice are
// unused by this test but required to satisfy the interface.
type asrStub struct{}

func (asrStub) Synthesize(context.Context, string, float64) ([]byte, error) { return nil, nil }

func (asrStub) ASRStream(_ context.Context, _ <-chan []byte) (<-chan voice.Transcript, error) {
	out := make(chan voice.Transcript)
	go func() {
		defer close(out)
		out <- voice.Transcript{Text: "你", Final: false}
		out <- voice.Transcript{Text: "你好", Final: true}
	}()
	return out, nil
}

func (asrStub) Voice() string { return "test-voice" }

// TestVoiceASRRelaysTranscripts drives the full WS bridge: dial with the
// session cookie, send one binary audio frame then a {"type":"stop"} text
// frame, and assert the two transcripts the stub emits arrive in order as
// partial/final JSON text frames.
func TestVoiceASRRelaysTranscripts(t *testing.T) {
	pool := newAPITestPool(t)
	q := sqlc.New(pool)
	h := New(Deps{Queries: q, Pool: pool, Voice: asrStub{}, CORSOrigins: []string{"http://localhost:5174"}}).Handler()

	srv := httptest.NewServer(h)
	defer srv.Close()

	cookie := signInSeed(t, pool)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/api/v1/voice/asr"
	c, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		HTTPHeader: http.Header{"Cookie": []string{cookie.String()}},
	})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close(websocket.StatusNormalClosure, "")

	if err := c.Write(ctx, websocket.MessageBinary, []byte{0x01, 0x02, 0x03}); err != nil {
		t.Fatalf("write audio frame: %v", err)
	}
	stopBody, _ := json.Marshal(map[string]string{"type": "stop"})
	if err := c.Write(ctx, websocket.MessageText, stopBody); err != nil {
		t.Fatalf("write stop frame: %v", err)
	}

	var got []map[string]any
	for i := 0; i < 2; i++ {
		typ, data, err := c.Read(ctx)
		if err != nil {
			t.Fatalf("read frame %d: %v", i, err)
		}
		if typ != websocket.MessageText {
			t.Fatalf("frame %d: want text message got %v", i, typ)
		}
		var msg map[string]any
		if err := json.Unmarshal(data, &msg); err != nil {
			t.Fatalf("frame %d: unmarshal: %v", i, err)
		}
		got = append(got, msg)
	}

	if got[0]["type"] != "partial" || got[0]["text"] != "你" {
		t.Fatalf("frame 0: want partial/你 got %#v", got[0])
	}
	if got[1]["type"] != "final" || got[1]["text"] != "你好" {
		t.Fatalf("frame 1: want final/你好 got %#v", got[1])
	}
}

// TestVoiceASRUnavailableWhenNil asserts the nil-Voice 503 fires before any
// WebSocket upgrade is attempted, so a plain (non-upgrading) request is
// enough — no WS dial needed.
func TestVoiceASRUnavailableWhenNil(t *testing.T) {
	pool := newAPITestPool(t)
	q := sqlc.New(pool)
	h := New(Deps{Queries: q, Pool: pool}).Handler()
	cookie := signInSeed(t, pool)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/voice/asr", nil), cookie))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503 got %d %s", rec.Code, rec.Body)
	}
}
