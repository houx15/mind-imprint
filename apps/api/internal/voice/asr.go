// This file implements the streaming client half of the Volcano Engine ASR
// API: dial, send the init request, then run a sender goroutine (PCM up)
// and a reader goroutine (transcripts down) concurrently. It consumes the
// wire-format builders/parser from asr_protocol.go (Task 6) and the
// dial/header idiom already used by Synthesize in tts.go (Task 2).
package voice

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/coder/websocket"
	"github.com/google/uuid"
)

// asrWSURL is the Volcano Engine v3 bidirectional streaming ASR ("bigmodel")
// WebSocket endpoint. It is a package var (not a const) so tests can point
// it at a local httptest server.
var asrWSURL = "wss://openspeech.bytedance.com/api/v3/sauc/bigmodel"

// asrUser/asrAudioParams/asrReqParams/asrFullRequest mirror the JSON body
// Volcano expects in the ASR full-client-request payload (see
// backend/app/services/asr.py's _build_full_client_request in the
// learning-lamp-v2 reference).
type asrUser struct {
	UID string `json:"uid"`
}

type asrAudioParams struct {
	Format  string `json:"format"`
	Codec   string `json:"codec"`
	Rate    int    `json:"rate"`
	Bits    int    `json:"bits"`
	Channel int    `json:"channel"`
}

type asrReqParams struct {
	ModelName       string `json:"model_name"`
	EnableITN       bool   `json:"enable_itn"`
	EnablePunc      bool   `json:"enable_punc"`
	EnableDDC       bool   `json:"enable_ddc"`
	ShowUtterances  bool   `json:"show_utterances"`
	EnableNonstream bool   `json:"enable_nonstream"`
}

type asrFullRequest struct {
	User    asrUser        `json:"user"`
	Audio   asrAudioParams `json:"audio"`
	Request asrReqParams   `json:"request"`
}

// Stream dials Volcano ASR, sends the init (full-client) request, and — once
// the server acknowledges it with code 0 — spawns a sender goroutine that
// forwards audioIn as audio-only frames and a reader goroutine that parses
// server frames into Transcripts on the returned channel.
//
// Channel/goroutine lifecycle: the returned channel is closed exactly once,
// by the reader goroutine, when it exits (server IsLast frame, a read
// error, or ctx cancellation). The reader also owns the WebSocket
// connection's single Close call (deferred). The sender goroutine exits
// either because audioIn is closed and drained (it sends one final packet
// and returns) or because its context is done. That context is a
// child (sendCtx) derived from ctx and cancelled the instant the reader
// goroutine exits, for any reason — not just outer-ctx cancellation. This
// matters because closing conn does not unblock a sender parked on
// <-audioIn: without an independently-cancelled sendCtx, a server-initiated
// IsLast (ahead of the caller finishing/closing audioIn, with the outer ctx
// never cancelled) would leak the sender goroutine forever.
func (c *Client) Stream(ctx context.Context, audioIn <-chan []byte) (<-chan Transcript, error) {
	headers := http.Header{}
	headers.Set("X-Api-Resource-Id", c.cfg.ASRResourceID)
	headers.Set("X-Api-Request-Id", uuid.NewString())
	headers.Set("X-Api-Access-Key", c.cfg.AccessKey)
	headers.Set("X-Api-App-Key", c.cfg.AppID)

	conn, _, err := websocket.Dial(ctx, asrWSURL, &websocket.DialOptions{
		HTTPClient: c.httpClient,
		HTTPHeader: headers,
	})
	if err != nil {
		// Never wrap the raw dial error's response body: it may echo
		// request headers (including the access key) back verbatim.
		return nil, fmt.Errorf("voice: dial asr websocket: %w", err)
	}
	conn.SetReadLimit(readLimitBytes)

	initPayload, err := json.Marshal(asrFullRequest{
		User: asrUser{UID: "mindimprint"},
		Audio: asrAudioParams{
			Format:  "pcm",
			Codec:   "raw",
			Rate:    16000,
			Bits:    16,
			Channel: 1,
		},
		Request: asrReqParams{
			ModelName:       "bigmodel",
			EnableITN:       true,
			EnablePunc:      true,
			EnableDDC:       true,
			ShowUtterances:  true,
			EnableNonstream: false,
		},
	})
	if err != nil {
		conn.Close(websocket.StatusInternalError, "")
		return nil, fmt.Errorf("voice: marshal asr init request: %w", err)
	}

	if err := conn.Write(ctx, websocket.MessageBinary, buildFullClientRequest(1, initPayload)); err != nil {
		conn.Close(websocket.StatusInternalError, "")
		return nil, fmt.Errorf("voice: send asr init request: %w", err)
	}

	_, initData, err := conn.Read(ctx)
	if err != nil {
		conn.Close(websocket.StatusInternalError, "")
		return nil, fmt.Errorf("voice: read asr init response: %w", err)
	}
	initResp, err := parseASRResponse(initData)
	if err != nil {
		conn.Close(websocket.StatusInternalError, "")
		return nil, fmt.Errorf("voice: parse asr init response: %w", err)
	}
	if initResp.Code != 0 {
		conn.Close(websocket.StatusInternalError, "")
		// Deliberately omit initResp.PayloadMsg: Volcano error bodies can
		// echo request content, and this error may end up in logs or (via
		// an HTTP handler) a client response.
		return nil, fmt.Errorf("voice: asr init failed (code %d)", initResp.Code)
	}

	out := make(chan Transcript)

	// sendCtx is derived from ctx but cancelled independently the moment
	// asrReceive exits, regardless of why (server IsLast, read error, or
	// outer ctx cancellation). This is required because asrSend can be
	// parked on <-audioIn with no way to notice that the reader — and thus
	// the connection — is already gone: closing conn does not unblock a
	// receive on audioIn, and the caller may never close audioIn or cancel
	// ctx on its own. Without this, a server-initiated IsLast (ahead of the
	// caller finishing its audio) leaks the sender goroutine forever.
	sendCtx, cancelSend := context.WithCancel(ctx)

	go asrSend(sendCtx, conn, audioIn)
	go func() {
		defer cancelSend()
		asrReceive(ctx, conn, out)
	}()

	return out, nil
}

// asrSend forwards audioIn chunks as audio-only frames, starting at seq 2
// (the init request used seq 1). It exits without further I/O as soon as
// ctx is done, and — on a clean audioIn close — writes one final
// (is-last) empty-segment frame before returning. It never touches out and
// never closes conn: the reader goroutine owns both.
func asrSend(ctx context.Context, conn *websocket.Conn, audioIn <-chan []byte) {
	seq := int32(2)
	for {
		select {
		case <-ctx.Done():
			return
		case chunk, ok := <-audioIn:
			if !ok {
				_ = conn.Write(ctx, websocket.MessageBinary, buildAudioOnlyRequest(seq, nil, true))
				return
			}
			if err := conn.Write(ctx, websocket.MessageBinary, buildAudioOnlyRequest(seq, chunk, false)); err != nil {
				return
			}
			seq++
		}
	}
}

// asrReceive reads server frames, emits a Transcript for each one carrying
// a result payload, and stops on the server's IsLast frame, a read error,
// or ctx cancellation. It is the sole owner of both the out channel's
// close and the connection's close, guaranteeing each happens exactly
// once regardless of which exit path is taken.
func asrReceive(ctx context.Context, conn *websocket.Conn, out chan<- Transcript) {
	defer conn.Close(websocket.StatusNormalClosure, "")
	defer close(out)

	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return
		}

		resp, err := parseASRResponse(data)
		if err != nil {
			return
		}

		if resp.PayloadMsg != nil {
			if result, ok := resp.PayloadMsg["result"].(map[string]any); ok {
				text, _ := result["text"].(string)
				isFinal, _ := result["is_final"].(bool)
				select {
				case out <- Transcript{Text: text, Final: isFinal}:
				case <-ctx.Done():
					return
				}
			}
		}

		if resp.IsLast {
			return
		}
	}
}
