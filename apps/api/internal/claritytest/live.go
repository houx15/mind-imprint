// Package claritytest supports opt-in synthetic prompt regression experiments.
// It is imported only by tests; never by the running service.
package claritytest

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/joho/godotenv"

	"mindimprint/api/internal/config"
	"mindimprint/api/internal/gateway"
)

// Run records the production-built request, visible response and validator
// result. Resolved credentials and model reasoning are never serialized.
type Collector func(context.Context, gateway.Provider, gateway.Resolved, gateway.ChatRequest) (gateway.ChatResult, error)

func Run(t *testing.T, class string, req gateway.ChatRequest, check func(string) error, collectors ...Collector) {
	t.Helper()
	// Capture synthetic fixtures without resolving credentials or calling a provider.
	if dir := os.Getenv("PROMPT_CAPTURE_DIR"); dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		body, err := json.MarshalIndent(struct {
			Class   string
			Request gateway.ChatRequest
		}{class, req}, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, strings.ReplaceAll(t.Name(), "/", "-")+".json")
		if err := os.WriteFile(path, body, 0644); err != nil {
			t.Fatal(err)
		}
		return
	}
	if os.Getenv("CLARITY_LIVE") != "1" {
		if verifyRequestSnapshot(t, class, req) {
			return
		}
		t.Skip("no offline snapshot; set CLARITY_LIVE=1 for paid synthetic prompt regression")
	}
	if path := os.Getenv("CLARITY_ENV_FILE"); path != "" {
		values, err := godotenv.Read(path)
		if err != nil {
			t.Fatal("cannot read requested environment file")
		}
		for k, v := range values {
			if (strings.HasPrefix(k, "MODEL_") || k == "DASHSCOPE_API_KEY" || k == "DEEPSEEK_API_KEY" || k == "ANTHROPIC_API_KEY") && os.Getenv(k) == "" {
				t.Setenv(k, v)
			}
		}
	}
	cfg := config.Config{DashScopeKey: os.Getenv("DASHSCOPE_API_KEY"), DeepSeekKey: os.Getenv("DEEPSEEK_API_KEY"), AnthropicKey: os.Getenv("ANTHROPIC_API_KEY"), ModelChat: os.Getenv("MODEL_CHAT"), ModelFastChat: os.Getenv("MODEL_FAST_CHAT"), ModelEval: os.Getenv("MODEL_EVAL")}
	rs, err := gateway.NewResolvers(cfg)
	if err != nil {
		t.Fatal("cannot resolve model catalog")
	}
	r, err := rs.For(class)(context.Background())
	if err != nil {
		t.Fatal("no configured provider for requested class")
	}
	if expected := os.Getenv("CLARITY_EXPECT_MODEL"); expected != "" && r.ModelID != expected {
		t.Fatalf("resolved model %q does not match required %q", r.ModelID, expected)
	}
	wire := &modelTransport{}
	provider := gateway.NewMuxProvider(map[string]gateway.Provider{gateway.KindOpenAICompatible: gateway.NewCatalogProvider(&http.Client{Timeout: 3 * time.Minute, Transport: wire}), gateway.KindAnthropic: gateway.NewAnthropicProvider(&http.Client{Timeout: 3 * time.Minute, Transport: wire})})
	n := 1
	if s := os.Getenv("CLARITY_SAMPLES"); s != "" {
		n, err = strconv.Atoi(s)
		if err != nil || n < 1 || n > 5 {
			t.Fatal("CLARITY_SAMPLES must be 1..5")
		}
	}
	dir := os.Getenv("CLARITY_OUT")
	if dir == "" {
		t.Fatal("CLARITY_OUT is required to preserve evidence")
	}
	if err = os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < n; i++ {
		path := filepath.Join(dir, strings.ReplaceAll(t.Name(), "/", "-")+"-"+strconv.Itoa(i+1)+".json")
		if _, e := os.Stat(path); e == nil {
			t.Fatal("evidence already exists; use a fresh CLARITY_OUT directory")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		start := time.Now()
		collect := Collector(gateway.Collect)
		if len(collectors) == 1 {
			collect = collectors[0]
		}
		res, callErr := collect(ctx, provider, r, req)
		cancel()
		issue := ""
		if callErr != nil {
			issue = "provider request failed"
			if len(collectors) == 1 {
				issue = "production pipeline did not return a valid result; see test log"
			}
		} else if os.Getenv("CLARITY_EXPECT_WIRE_MODEL") != "" && wire.model != os.Getenv("CLARITY_EXPECT_WIRE_MODEL") {
			issue = "upstream response model does not match required model"
		} else if check != nil {
			if e := check(res.Text); e != nil {
				issue = e.Error()
			}
		}
		record := struct {
			Case, Class, Model          string
			WireModel, ResponseModel    string
			Sample                      int
			Millis                      int64
			Request                     gateway.ChatRequest
			Text                        string
			Usage                       gateway.ChatUsage
			StopReason, ValidationError string
		}{t.Name(), class, r.ModelID, r.Model, wire.model, i + 1, time.Since(start).Milliseconds(), req, res.Text, res.Usage, res.StopReason, issue}
		body, e := json.MarshalIndent(record, "", "  ")
		if e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(path, body, 0644); e != nil {
			t.Fatal(e)
		}
		t.Logf("%s sample=%d model=%s ms=%d input=%d output=%d valid=%t", t.Name(), i+1, r.ModelID, record.Millis, res.Usage.InputTokens, res.Usage.OutputTokens, issue == "")
		if issue != "" && os.Getenv("CLARITY_BASELINE") != "1" {
			t.Errorf("%s (visible output preserved in evidence)", issue)
		}
	}
}

// Capture only the upstream model name, never response reasoning or credentials.
// Buffering affects streaming delivery in this test helper only, not the service.
type modelTransport struct{ model string }

func (m *modelTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	m.model = ""
	resp, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, err
	}
	resp.Body = io.NopCloser(bytes.NewReader(body))
	for _, line := range bytes.Split(body, []byte("\n")) {
		line = bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data:")))
		var envelope struct {
			Model string `json:"model"`
		}
		if json.Unmarshal(line, &envelope) == nil && envelope.Model != "" {
			m.model = envelope.Model
		}
	}
	return resp, nil
}
