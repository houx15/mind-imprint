package voice

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/coder/websocket"
	"github.com/google/uuid"
)

// ttsWSURL is the Volcano Engine v3 unidirectional TTS WebSocket endpoint.
// It is a package var (not a const) so tests can point it at a local
// httptest server.
var ttsWSURL = "wss://openspeech.bytedance.com/api/v3/tts/unidirectional/stream"

// readLimitBytes bounds a single WebSocket frame read from Volcano. Audio
// responses are chunked, but the limit is generous to tolerate any single
// oversized frame without risking unbounded memory growth.
const readLimitBytes = 10 << 20 // 10 MiB

// Client is a Volcano Engine voice API client. It holds no I/O state
// between calls: Synthesize dials a fresh WebSocket connection per
// invocation and closes it before returning.
type Client struct {
	cfg        Config
	httpClient *http.Client
}

// New constructs a Client from cfg. cfg.AccessKey and cfg.AppID are
// Volcano credentials; the caller is responsible for sourcing them from
// server-side environment/config only — Client never logs or otherwise
// exposes them.
func New(cfg Config) *Client {
	return &Client{cfg: cfg, httpClient: http.DefaultClient}
}

// ttsAudioParams and ttsRequest mirror the JSON body expected by Volcano's
// v3 unidirectional TTS stream API (see backend/app/services/tts.py in the
// learning-lamp-v2 reference for the equivalent Python dict).
type ttsAudioParams struct {
	Format          string `json:"format"`
	SampleRate      int    `json:"sample_rate"`
	EnableTimestamp bool   `json:"enable_timestamp"`
}

type ttsReqParams struct {
	Speaker     string         `json:"speaker"`
	AudioParams ttsAudioParams `json:"audio_params"`
	Text        string         `json:"text"`
	SpeedRatio  float64        `json:"speed_ratio"`
	VolumeRatio float64        `json:"volume_ratio"`
	Additions   string         `json:"additions"`
}

type ttsUser struct {
	UID string `json:"uid"`
}

type ttsRequest struct {
	User      ttsUser      `json:"user"`
	ReqParams ttsReqParams `json:"req_params"`
}

// Synthesize sends text to Volcano TTS over a WebSocket connection and
// returns the concatenated audio bytes (mp3) once the server reports
// SessionFinished.
//
// Live-probe note (spec §12): Volcano's real endpoint has not been
// confirmed to accept audio_params.format == "mp3" for this resource id —
// only the fake test server in tts_test.go has verified the wire format.
// If a live call rejects "mp3", switch ttsAudioParams.Format to "pcm" here.
func (c *Client) Synthesize(ctx context.Context, text string, speed float64) ([]byte, error) {
	req := ttsRequest{
		User: ttsUser{UID: uuid.NewString()},
		ReqParams: ttsReqParams{
			Speaker: c.cfg.TTSVoice,
			AudioParams: ttsAudioParams{
				Format:          "mp3",
				SampleRate:      24000,
				EnableTimestamp: false,
			},
			Text:        text,
			SpeedRatio:  speed,
			VolumeRatio: 1.0,
			Additions:   `{"disable_markdown_filter":false}`,
		},
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("voice: marshal tts request: %w", err)
	}

	headers := http.Header{}
	headers.Set("X-Api-App-Key", c.cfg.AppID)
	headers.Set("X-Api-Access-Key", c.cfg.AccessKey)
	headers.Set("X-Api-Resource-Id", c.cfg.TTSResourceID)
	headers.Set("X-Api-Connect-Id", uuid.NewString())

	conn, _, err := websocket.Dial(ctx, ttsWSURL, &websocket.DialOptions{
		HTTPClient: c.httpClient,
		HTTPHeader: headers,
	})
	if err != nil {
		// Never wrap the raw dial error's response body: it may echo
		// request headers (including the access key) back verbatim.
		return nil, fmt.Errorf("voice: dial tts websocket: %w", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")
	conn.SetReadLimit(readLimitBytes)

	if err := conn.Write(ctx, websocket.MessageBinary, FullClientRequest(body).Marshal()); err != nil {
		return nil, fmt.Errorf("voice: send tts request: %w", err)
	}

	var audio []byte
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return nil, fmt.Errorf("voice: read tts frame: %w", err)
		}

		msg, err := ParseMessage(data)
		if err != nil {
			return nil, fmt.Errorf("voice: parse tts frame: %w", err)
		}

		switch msg.Type {
		case AudioOnlyServer:
			audio = append(audio, msg.Payload...)
		case FullServerResponse:
			if msg.Event == SessionFinished {
				return audio, nil
			}
		case Error:
			// Deliberately omit msg.Payload: Volcano error bodies can
			// echo request content, and this error may end up in logs
			// or (via an HTTP handler) a client response.
			return nil, fmt.Errorf("voice: tts error from server (code %d)", msg.ErrorCode)
		}
	}
}
