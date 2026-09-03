package gateway

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"mindimprint/api/internal/config"
)

// A route that refuses the thinking-off knob but accepts a floor effort must be
// steered with the floor, not skipped.
//
// This is the shape of a bug that cannot be seen by reading the code: send
// enable_thinking:false to ZHIPU/GLM-5.3 and DashScope answers 400 code 1210
// telling you to use low/high/max instead. Sending nothing at all is worse than
// the 400 — the model then reasons at its default (max), which on a dialogue
// turn is a 40-second wait where the class promised four.
func TestReasoningOffUsesTheFloorEffortWhenThinkingOffIsRefused(t *testing.T) {
	cat, err := DefaultCatalog()
	if err != nil {
		t.Fatal(err)
	}
	pol := cat.policyFor(cat.Models["dashscope/glm-5.3"])
	if pol.MinReasoningEffort == "" {
		t.Fatal("glm-5.3 must declare a floor effort, else it is banned from three classes")
	}

	p := &CatalogProvider{}
	r := Resolved{
		Provider:  "dashscope",
		Model:     "ZHIPU/GLM-5.3",
		Reasoning: ReasoningOff,
		Policy:    pol,
	}
	body, err := p.buildBody(r, ChatRequest{Messages: []ChatMessage{{Role: RoleUser, Content: "hi"}}})
	if err != nil {
		t.Fatalf("buildBody: %v", err)
	}
	if got := body["reasoning_effort"]; got != pol.MinReasoningEffort {
		t.Errorf("reasoning_effort = %v, want the floor %q", got, pol.MinReasoningEffort)
	}
	if _, sent := body["enable_thinking"]; sent {
		t.Error("enable_thinking must not be sent to a route that answers 400 to it")
	}
}

// A model that refuses thinking-off AND declares no floor still cannot serve a
// reasoning-off class. Without this, the previous test's fix would quietly
// re-admit every always-thinking model to the fast classes.
func TestReasoningOffStillRejectsAModelWithNoFloor(t *testing.T) {
	cfg := config.Config{
		DashScopeKey: "secret",
		ModelClass:   map[string]string{ClassDialogue: "dashscope/kimi-k2.7-code"},
	}
	if _, err := NewResolvers(cfg); err == nil {
		t.Fatal("want a boot error: kimi-k2.7-code has no way to stop reasoning")
	}
}

// GenerateLookback sends a system message and nothing else. Most models accept
// that; ZHIPU/GLM-5.3 answers `messages 参数非法` (400, code 1214). The catalog
// promises that swapping a model is an env var, so the request shape has to bend
// for the route that needs it — and must NOT bend for the ones that do not,
// since every other model was benchmarked on the unmodified shape.
func TestSystemOnlyRequestGainsAUserTurnOnlyWhereTheRouteDemandsIt(t *testing.T) {
	cat, err := DefaultCatalog()
	if err != nil {
		t.Fatal(err)
	}
	p := &CatalogProvider{}
	sysOnly := ChatRequest{Messages: []ChatMessage{{Role: RoleSystem, Content: "生成回顾。"}}}

	roles := func(modelID string) []string {
		m := cat.Models[modelID]
		body, err := p.buildBody(Resolved{Model: m.Model, Policy: cat.policyFor(m)}, sysOnly)
		if err != nil {
			t.Fatalf("%s: %v", modelID, err)
		}
		msgs, _ := body["messages"].([]map[string]any)
		out := make([]string, len(msgs))
		for i, msg := range msgs {
			out[i], _ = msg["role"].(string)
		}
		return out
	}

	if got := roles("dashscope/glm-5.3"); len(got) != 2 || got[1] != RoleUser {
		t.Errorf("glm-5.3 roles = %v, want a user turn appended", got)
	}
	if got := roles("dashscope/deepseek-v4-pro"); len(got) != 1 {
		t.Errorf("deepseek roles = %v, want the request untouched", got)
	}
}

// The provider's own words are the whole point of an error. Withheld on auth
// statuses, where the body may quote the credential back and adds nothing.
func TestUpstreamErrorBodyIsCarriedExceptOnAuthFailures(t *testing.T) {
	serve := func(status int, body string) (*httptest.Server, error) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(body))
		}))
		_, err := streamOpenAICompatible(t.Context(), srv.Client(),
			Resolved{BaseURL: srv.URL, Model: "m", APIKey: "sk-should-never-appear"},
			map[string]any{}, "dashscope")
		return srv, err
	}

	srv, err := serve(http.StatusBadRequest, `{"error":{"message":"该模型始终思考","code":"1210"}}`)
	defer srv.Close()
	if err == nil || !strings.Contains(err.Error(), "1210") {
		t.Errorf("a 400 must carry the provider's words, got: %v", err)
	}

	srv2, err2 := serve(http.StatusUnauthorized, `{"error":{"message":"Incorrect API key: sk-should-never-appear"}}`)
	defer srv2.Close()
	if err2 == nil || strings.Contains(err2.Error(), "sk-should-never-appear") {
		t.Errorf("a 401 body must not be echoed, got: %v", err2)
	}
}
