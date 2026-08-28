package gateway

import (
	"context"
	"fmt"
	"net/http"
)

// GLMProvider streams GLM Chat Completions through Zhipu's OpenAI-compatible
// endpoint. The current integration targets GLM-5.3-Flash and its required
// thinking-on policy.
type GLMProvider struct {
	http *http.Client
}

// NewGLMProvider returns a GLMProvider using the given HTTP client.
func NewGLMProvider(c *http.Client) *GLMProvider {
	if c == nil {
		c = http.DefaultClient
	}
	return &GLMProvider{http: c}
}

func (p *GLMProvider) buildBody(r Resolved, req ChatRequest) (map[string]any, error) {
	if req.DisableThinking {
		return nil, fmt.Errorf("%w: glm-5.3-flash does not support disabling thinking", errStreamFailed)
	}
	body := buildOpenAICompatibleBody(r, req, 16000)
	if req.Temperature == nil {
		body["temperature"] = 1.0
	}
	body["top_p"] = 0.95
	body["thinking"] = map[string]any{"type": "enabled", "clear_thinking": false}
	effort := req.ReasoningEffort
	if effort == "" {
		effort = r.DefaultReasoningEffort
	}
	if effort == "" {
		effort = "max"
	}
	body["reasoning_effort"] = effort
	if len(req.Tools) > 0 {
		body["tool_stream"] = true
	}
	return body, nil
}

// Stream issues the GLM streaming request and emits only visible text, tool use,
// and token usage. Reasoning content is deliberately never surfaced.
func (p *GLMProvider) Stream(ctx context.Context, r Resolved, req ChatRequest) (<-chan StreamEvent, error) {
	body, err := p.buildBody(r, req)
	if err != nil {
		return nil, err
	}
	return streamOpenAICompatible(ctx, p.http, r, body, "glm")
}
