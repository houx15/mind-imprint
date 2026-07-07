package voice

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/coder/websocket"
)

func fakeVolcanoTTS(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		defer c.Close(websocket.StatusNormalClosure, "")
		ctx := r.Context()
		if _, _, err := c.Read(ctx); err != nil { // client full request
			return
		}
		// Two audio frames then SessionFinished.
		for _, chunk := range [][]byte{{0x01, 0x02}, {0x03, 0x04}} {
			_ = c.Write(ctx, websocket.MessageBinary, Message{Type: AudioOnlyServer, Flag: NoSeq, Payload: chunk}.Marshal())
		}
		_ = c.Write(ctx, websocket.MessageBinary, Message{Type: FullServerResponse, Flag: WithEvent, Event: SessionFinished, Payload: []byte("{}")}.Marshal())
	}))
}

func TestSynthesizeConcatenatesAudio(t *testing.T) {
	srv := fakeVolcanoTTS(t)
	defer srv.Close()
	ttsWSURL = "ws" + strings.TrimPrefix(srv.URL, "http")

	c := New(Config{AppID: "a", AccessKey: "k", TTSVoice: "v", TTSResourceID: "seed-tts-2.0"})
	audio, err := c.Synthesize(context.Background(), "你好", 1.0)
	if err != nil {
		t.Fatal(err)
	}
	if string(audio) != string([]byte{0x01, 0x02, 0x03, 0x04}) {
		t.Fatalf("audio = % x", audio)
	}
}
