package api_test

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/store/sqlc"
	"mindimprint/api/internal/voice"
)

// stubVoice is a test double for VoiceService: Synthesize counts calls and
// returns fixed audio; ASRStream is unused by this test (Task 8 territory).
type stubVoice struct {
	calls int
	audio []byte
}

func (s *stubVoice) Synthesize(_ context.Context, _ string, _ float64) ([]byte, error) {
	s.calls++
	return s.audio, nil
}

func (s *stubVoice) ASRStream(context.Context, <-chan []byte) (<-chan voice.Transcript, error) {
	return nil, errors.New("n/a")
}

func TestVoiceTTSCachesAndServes(t *testing.T) {
	pool := newAPITestPool(t)
	q := sqlc.New(pool)
	stub := &stubVoice{audio: []byte("MP3")}
	h := New(Deps{Queries: q, Pool: pool, Voice: stub}).Handler()
	cookie := signInSeed(t, pool)

	body := `{"text":"你好"}`
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/voice/tts", strings.NewReader(body)), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("1st POST: want 200 got %d %s", rec.Code, rec.Body)
	}
	if !bytes.Equal(rec.Body.Bytes(), []byte("MP3")) {
		t.Fatalf("1st POST body: want MP3 got %q", rec.Body.Bytes())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "audio/mpeg" {
		t.Fatalf("1st POST content-type: want audio/mpeg got %q", ct)
	}
	if stub.calls != 1 {
		t.Fatalf("1st POST: want 1 synth call got %d", stub.calls)
	}

	// Identical 2nd POST should be served from cache — no new synth call.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/voice/tts", strings.NewReader(body)), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("2nd POST: want 200 got %d %s", rec.Code, rec.Body)
	}
	if !bytes.Equal(rec.Body.Bytes(), []byte("MP3")) {
		t.Fatalf("2nd POST body: want MP3 got %q", rec.Body.Bytes())
	}
	if stub.calls != 1 {
		t.Fatalf("2nd POST: want cache hit (still 1 synth call) got %d", stub.calls)
	}
}

func TestVoiceTTSUnavailableWhenNil(t *testing.T) {
	pool := newAPITestPool(t)
	q := sqlc.New(pool)
	h := New(Deps{Queries: q, Pool: pool}).Handler()
	cookie := signInSeed(t, pool)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/voice/tts", strings.NewReader(`{"text":"你好"}`)), cookie))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503 got %d %s", rec.Code, rec.Body)
	}
}
