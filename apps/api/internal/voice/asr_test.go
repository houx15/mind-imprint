package voice

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/binary"
	"net/http"
	"net/http/httptest"
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
