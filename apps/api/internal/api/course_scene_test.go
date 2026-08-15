package api_test

// course_scene_test.go — Course Runtime Slice 7: POST /api/v1/courses/{slug}/scene.
// The endpoint generates Opening/Closing narration on the chaperone tier and
// NEVER 500s on a generation failure: an LLM error degrades to the caller's
// authored fallback text (fallbackUsed=true), and a missing Voice/OSS degrades
// to text-only (fallbackUsed=false, no audioUrl). These tests wire nil Voice/OSS
// so no real TTS/bucket is touched.

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/store/sqlc"
)

// sceneErrProvider is a gateway.Provider whose Stream always fails the model
// call — exercising the endpoint's LLM-down fallback path.
type sceneErrProvider struct{ err error }

func (p sceneErrProvider) Stream(context.Context, gateway.Resolved, gateway.ChatRequest) (<-chan gateway.StreamEvent, error) {
	return nil, p.err
}

func sceneResult(t *testing.T, rec *httptest.ResponseRecorder) struct {
	Text            string   `json:"text"`
	AudioURL        string   `json:"audioUrl"`
	GeneratedAt     string   `json:"generatedAt"`
	UsedSignalTypes []string `json:"usedSignalTypes"`
	FallbackUsed    bool     `json:"fallbackUsed"`
} {
	t.Helper()
	var resp struct {
		Result struct {
			Text            string   `json:"text"`
			AudioURL        string   `json:"audioUrl"`
			GeneratedAt     string   `json:"generatedAt"`
			UsedSignalTypes []string `json:"usedSignalTypes"`
			FallbackUsed    bool     `json:"fallbackUsed"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode scene result: %v — body %s", err, rec.Body)
	}
	return resp.Result
}

func openingSceneBody() []byte {
	b, _ := json.Marshal(map[string]any{
		"which": "opening",
		"facts": map[string]any{
			"title":            "一条网络信息，该不该信",
			"estimatedMinutes": 12,
			"objectives":       []string{"学会用 CRAAP 判断信源"},
			"learningPreview":  []string{"给一条说法做溯源体检"},
		},
		"allowedSignals": []string{},
		"signalEvidence": map[string]string{},
		"fallback":       map[string]any{"text": "同学你好，我们开始今天的课。"},
	})
	return b
}

// TestSceneGeneratesOpening — a valid opening request with a working provider
// and nil Voice/OSS returns 200 with the model's text, fallbackUsed=false, no
// audioUrl, and meters exactly one scene llm_call row.
func TestSceneGeneratesOpening(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: fakeProvider(), ChatResolver: fakeResolver(),
	}).Handler()
	cookie := signInSeed(t, pool)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/courses/a-mid/scene", bytes.NewReader(openingSceneBody())), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("scene: %d %s", rec.Code, rec.Body)
	}
	res := sceneResult(t, rec)
	if res.Text != "先说说你打算怎么把这条证据接上主张？" {
		t.Fatalf("text = %q, want the fake provider's completion", res.Text)
	}
	if res.FallbackUsed {
		t.Fatalf("fallbackUsed should be false on a successful generation")
	}
	if res.AudioURL != "" {
		t.Fatalf("audioUrl should be empty with nil Voice/OSS, got %q", res.AudioURL)
	}
	if res.GeneratedAt == "" {
		t.Fatalf("generatedAt must be stamped by the server")
	}

	var surface, purpose string
	if err := pool.QueryRow(context.Background(),
		`SELECT surface, purpose FROM llm_call WHERE user_id = $1 ORDER BY created_at DESC LIMIT 1`, SeedUserID,
	).Scan(&surface, &purpose); err != nil {
		t.Fatalf("query llm_call: %v", err)
	}
	if surface != "course" || purpose != "scene" {
		t.Fatalf("llm_call surface=%q purpose=%q, want course/scene", surface, purpose)
	}
}

// TestSceneFallsBackOnLLMError — a provider that fails still returns 200 with
// the authored fallback text and fallbackUsed=true (never a 500).
func TestSceneFallsBackOnLLMError(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: sceneErrProvider{err: context.DeadlineExceeded}, ChatResolver: fakeResolver(),
	}).Handler()
	cookie := signInSeed(t, pool)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/courses/a-mid/scene", bytes.NewReader(openingSceneBody())), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("scene (llm down): want 200, got %d %s", rec.Code, rec.Body)
	}
	res := sceneResult(t, rec)
	if res.Text != "同学你好，我们开始今天的课。" {
		t.Fatalf("text = %q, want the authored fallback", res.Text)
	}
	if !res.FallbackUsed {
		t.Fatalf("fallbackUsed must be true when generation fails")
	}
}

// TestSceneRejectsUnknownWhich — an unknown `which` is a 400, before any model call.
func TestSceneRejectsUnknownWhich(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: fakeProvider(), ChatResolver: fakeResolver(),
	}).Handler()
	cookie := signInSeed(t, pool)

	body, _ := json.Marshal(map[string]any{"which": "middle", "fallback": map[string]any{"text": "x"}})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/courses/a-mid/scene", bytes.NewReader(body)), cookie))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unknown which: want 400, got %d %s", rec.Code, rec.Body)
	}
}

// TestSceneRequiresAuth — the endpoint sits behind `protected`; unauthenticated
// requests are 401.
func TestSceneRequiresAuth(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: fakeProvider(), ChatResolver: fakeResolver(),
	}).Handler()

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("POST", "/api/v1/courses/a-mid/scene", bytes.NewReader(openingSceneBody())))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauth: want 401, got %d", rec.Code)
	}
}
