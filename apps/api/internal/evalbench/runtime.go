package evalbench

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/joho/godotenv"

	"mindimprint/api/internal/gateway"
)

const FlagshipTier = "flagship"

const (
	defaultCandidateCallTimeout  = 5 * time.Minute
	defaultComparatorCallTimeout = 2 * time.Minute
)

type Runtime struct {
	profiles          map[string]ModelProfile
	provider          gateway.Provider
	candidateTimeout  time.Duration
	comparatorTimeout time.Duration
}

func NewRuntime(c Config) (*Runtime, error) {
	_ = godotenv.Load(".env.local")
	client := &http.Client{}
	return &Runtime{
		profiles: c.Models,
		provider: gateway.NewMuxProvider(map[string]gateway.Provider{
			"deepseek":  gateway.NewDeepSeekProvider(client),
			"anthropic": gateway.NewAnthropicProvider(client),
			"glm":       gateway.NewGLMProvider(client),
		}),
		candidateTimeout:  timeoutDuration(c.Timeouts.CandidateSeconds, defaultCandidateCallTimeout),
		comparatorTimeout: timeoutDuration(c.Timeouts.ComparatorSeconds, defaultComparatorCallTimeout),
	}, nil
}

func timeoutDuration(seconds int, fallback time.Duration) time.Duration {
	if seconds == 0 {
		return fallback
	}
	return time.Duration(seconds) * time.Second
}

func (r *Runtime) Resolve(profileID string) (gateway.Resolved, ModelProfile, error) {
	p, ok := r.profiles[profileID]
	if !ok {
		return gateway.Resolved{}, ModelProfile{}, fmt.Errorf("evalbench: unknown model profile %q", profileID)
	}
	var baseURL, key string
	switch p.Provider {
	case "deepseek":
		baseURL, key = "https://api.deepseek.com/v1", os.Getenv("DEEPSEEK_API_KEY")
	case "anthropic":
		baseURL, key = "https://api.anthropic.com/v1", os.Getenv("ANTHROPIC_API_KEY")
	case "glm":
		baseURL, key = "https://open.bigmodel.cn/api/paas/v4", os.Getenv("ZAI_API_KEY")
	default:
		return gateway.Resolved{}, ModelProfile{}, fmt.Errorf("evalbench: unsupported provider %q", p.Provider)
	}
	if key == "" {
		return gateway.Resolved{}, ModelProfile{}, fmt.Errorf("evalbench: missing API key for provider %q", p.Provider)
	}
	return gateway.Resolved{Provider: p.Provider, BaseURL: baseURL, Model: p.Model, APIKey: key, Tier: FlagshipTier, DefaultReasoningEffort: p.ReasoningEffort}, p, nil
}

func (r *Runtime) Observed(purpose string, recorder *CallRecorder) gateway.Provider {
	timeout := r.candidateTimeout
	if purpose == "comparator" {
		timeout = r.comparatorTimeout
	}
	return &ObservedProvider{Inner: r.provider, Recorder: recorder, Purpose: purpose, CallTimeout: timeout}
}

type CallRecord struct {
	Purpose         string              `json:"purpose"`
	Provider        string              `json:"provider"`
	Model           string              `json:"model"`
	Tier            string              `json:"tier"`
	StartedAt       time.Time           `json:"startedAt"`
	StreamReadyMs   int64               `json:"streamReadyMs"`
	FirstOutputMs   *int64              `json:"firstOutputMs,omitempty"`
	TotalMs         int64               `json:"totalMs"`
	InputTokens     *int                `json:"inputTokens,omitempty"`
	OutputTokens    *int                `json:"outputTokens,omitempty"`
	ReasoningTokens *int                `json:"reasoningTokens,omitempty"`
	ContentTokens   *int                `json:"contentTokens,omitempty"`
	StopReason      gateway.StopReason  `json:"stopReason,omitempty"`
	RequestBytes    int                 `json:"requestBytes"`
	OutputBytes     int                 `json:"outputBytes"`
	Incomplete      bool                `json:"incompleteStream,omitempty"`
	Error           string              `json:"error,omitempty"`
	Request         gateway.ChatRequest `json:"request"`
	RawOutput       string              `json:"rawOutput"`
}

// CallRecorder is safe for future multi-step/concurrent evaluators. It records
// only request content and never receives gateway.Resolved, so API keys cannot
// enter experiment artifacts.
type CallRecorder struct {
	mu    sync.Mutex
	calls []CallRecord
}

func (r *CallRecorder) Add(c CallRecord) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, c)
}

func (r *CallRecorder) Calls() []CallRecord {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]CallRecord, len(r.calls))
	copy(out, r.calls)
	return out
}

type ObservedProvider struct {
	Inner       gateway.Provider
	Recorder    *CallRecorder
	Purpose     string
	CallTimeout time.Duration
}

func (p *ObservedProvider) Stream(ctx context.Context, resolved gateway.Resolved, req gateway.ChatRequest) (<-chan gateway.StreamEvent, error) {
	started := time.Now().UTC()
	reqBytes, _ := json.Marshal(req)
	callCtx := ctx
	cancel := func() {}
	if p.CallTimeout > 0 {
		callCtx, cancel = context.WithTimeout(ctx, p.CallTimeout)
	}
	stream, err := p.Inner.Stream(callCtx, resolved, req)
	streamReady := time.Since(started)
	if err != nil {
		if callCtx.Err() != nil {
			err = callCtx.Err()
		}
		cancel()
		p.Recorder.Add(CallRecord{
			Purpose: p.Purpose, Provider: resolved.Provider, Model: resolved.Model, Tier: resolved.Tier,
			StartedAt: started, StreamReadyMs: streamReady.Milliseconds(), TotalMs: streamReady.Milliseconds(),
			RequestBytes: len(reqBytes), Error: err.Error(), Request: req,
		})
		return nil, err
	}
	out := make(chan gateway.StreamEvent)
	go func() {
		defer close(out)
		defer cancel()
		var raw strings.Builder
		var usage *gateway.ChatUsage
		var first *int64
		var stop gateway.StopReason
		seenDone := false
		incomplete := false
		var streamErr error
		defer func() {
			ended := time.Now().UTC()
			record := CallRecord{
				Purpose: p.Purpose, Provider: resolved.Provider, Model: resolved.Model, Tier: resolved.Tier,
				StartedAt: started, StreamReadyMs: streamReady.Milliseconds(), FirstOutputMs: first,
				TotalMs: ended.Sub(started).Milliseconds(), StopReason: stop, RequestBytes: len(reqBytes),
				OutputBytes: raw.Len(), Incomplete: incomplete || !seenDone, Request: req, RawOutput: raw.String(),
			}
			if streamErr != nil {
				record.Error = streamErr.Error()
			}
			if usage != nil {
				in, outTokens := usage.InputTokens, usage.OutputTokens
				record.InputTokens, record.OutputTokens = &in, &outTokens
				if usage.ReasoningTokens != nil {
					reasoning := *usage.ReasoningTokens
					record.ReasoningTokens = &reasoning
					if outTokens >= reasoning {
						content := outTokens - reasoning
						record.ContentTokens = &content
					}
				}
			}
			p.Recorder.Add(record)
		}()
		for ev := range stream {
			if first == nil && ((ev.Kind == gateway.EventTextDelta && ev.TextDelta != "") || ev.Kind == gateway.EventToolUse) {
				v := time.Since(started).Milliseconds()
				first = &v
			}
			if ev.Kind == gateway.EventTextDelta {
				raw.WriteString(ev.TextDelta)
			}
			if ev.Kind == gateway.EventUsage && ev.Usage != nil {
				u := *ev.Usage
				usage = &u
			}
			if ev.Kind == gateway.EventDone {
				seenDone, stop = true, ev.StopReason
				if ev.Incomplete {
					incomplete = true
					streamErr = errIncompleteStream
				}
			}
			select {
			case <-ctx.Done():
				streamErr = ctx.Err()
				return
			case out <- ev:
			}
		}
		if callCtx.Err() != nil && ctx.Err() == nil {
			streamErr = callCtx.Err()
		}
	}()
	return out, nil
}
