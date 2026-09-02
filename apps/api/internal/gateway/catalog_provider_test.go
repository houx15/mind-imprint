package gateway

import (
	"testing"
)

func paiResolved(t *testing.T, lane string) Resolved {
	t.Helper()
	cat, _ := DefaultCatalog()
	r, err := cat.Resolve(lane, "", onlyKey("PAI_API_KEY", "sk-pai"))
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// The regression this refactor exists to prevent: through PAI, the body that
// works directly against DeepSeek (`thinking:{type:disabled}`) is silently
// ignored and reasoning keeps running — 4,000-7,000 completion tokens and
// 40-66s per chaperone turn. The chaperone body must carry PAI's own knob.
func TestChaperoneTurnDisablesThinkingWithTheKnobPAIHonors(t *testing.T) {
	p := NewCatalogProvider(nil)
	body, err := p.buildBody(paiResolved(t, LaneChat), ChatRequest{
		Messages: []ChatMessage{{Role: RoleUser, Content: "hi"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if body["enable_thinking"] != false {
		t.Errorf("chaperone via PAI must send enable_thinking:false, got %#v", body["enable_thinking"])
	}
	if _, ok := body["thinking"]; ok {
		t.Error("must not send the direct-DeepSeek knob through PAI — it is ignored there")
	}
}

// The flagship reviewer/eval seam keeps reasoning on.
func TestFlagshipTurnKeepsThinking(t *testing.T) {
	p := NewCatalogProvider(nil)
	body, err := p.buildBody(paiResolved(t, LaneEval), ChatRequest{
		Messages: []ChatMessage{{Role: RoleUser, Content: "hi"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := body["enable_thinking"]; ok {
		t.Error("flagship must not disable thinking — 评估绝不降级")
	}
}

// A flagship call may still ask for a bounded budget.
func TestReasoningEffortIsSentOnFlagshipOnly(t *testing.T) {
	p := NewCatalogProvider(nil)
	req := ChatRequest{Messages: []ChatMessage{{Role: RoleUser, Content: "hi"}}, ReasoningEffort: "low"}

	flagship, err := p.buildBody(paiResolved(t, LaneEval), req)
	if err != nil {
		t.Fatal(err)
	}
	if flagship["reasoning_effort"] != "low" {
		t.Errorf("flagship reasoning_effort = %#v, want low", flagship["reasoning_effort"])
	}

	chap, err := p.buildBody(paiResolved(t, LaneChat), req)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := chap["reasoning_effort"]; ok {
		t.Error("thinking is already off; a reasoning budget alongside it is meaningless")
	}
}

// A route that declares no effort key must not receive the field at all.
func TestReasoningEffortSkippedWhenRouteIgnoresIt(t *testing.T) {
	p := NewCatalogProvider(nil)
	r := Resolved{Model: "m", Tier: "flagship"} // zero policy: no effort key
	body, err := p.buildBody(r, ChatRequest{ReasoningEffort: "low"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := body["reasoning_effort"]; ok {
		t.Error("must not send a field the route ignores")
	}
}

// An explicit request to stop reasoning on a model that cannot is an error, not
// a silently-dropped field.
func TestDisableThinkingErrorsWhenUnsupported(t *testing.T) {
	p := NewCatalogProvider(nil)
	r := Resolved{Model: "glm-5.3-flash", Tier: "flagship", Policy: ModelPolicy{ThinkingOffUnsupported: true}}
	if _, err := p.buildBody(r, ChatRequest{DisableThinking: true}); err == nil {
		t.Fatal("want an error rather than a silently ignored DisableThinking")
	}
}

// A chaperone-TIER call on a model that cannot stop reasoning proceeds — the
// tier is a preference, only an explicit request is a hard requirement.
func TestChaperoneTierToleratesAlwaysThinkingModel(t *testing.T) {
	p := NewCatalogProvider(nil)
	r := Resolved{Model: "m", Tier: "chaperone", Policy: ModelPolicy{ThinkingOffUnsupported: true}}
	if _, err := p.buildBody(r, ChatRequest{}); err != nil {
		t.Fatalf("tier preference must not fail the call: %v", err)
	}
}

func TestBodyExtraAndToolExtraApply(t *testing.T) {
	p := NewCatalogProvider(nil)
	temp := 1.0
	r := Resolved{Model: "m", Tier: "flagship", Policy: ModelPolicy{
		BodyExtra:          map[string]any{"top_p": 0.95},
		BodyExtraWithTools: map[string]any{"tool_stream": true},
		DefaultTemperature: &temp,
	}}

	plain, err := p.buildBody(r, ChatRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if plain["top_p"] != 0.95 {
		t.Errorf("bodyExtra not applied: %#v", plain["top_p"])
	}
	if plain["temperature"] != 1.0 {
		t.Errorf("defaultTemperature not applied: %#v", plain["temperature"])
	}
	if _, ok := plain["tool_stream"]; ok {
		t.Error("tool-only extras must not appear on a toolless turn")
	}

	withTools, err := p.buildBody(r, ChatRequest{Tools: []ChatTool{{Name: "t"}}})
	if err != nil {
		t.Fatal(err)
	}
	if withTools["tool_stream"] != true {
		t.Error("tool extras must apply when the turn carries tools")
	}
}

func TestRequestTemperatureBeatsDefault(t *testing.T) {
	p := NewCatalogProvider(nil)
	def := 1.0
	want := 0.2
	body, err := p.buildBody(
		Resolved{Model: "m", Tier: "flagship", Policy: ModelPolicy{DefaultTemperature: &def}},
		ChatRequest{Temperature: &want},
	)
	if err != nil {
		t.Fatal(err)
	}
	if body["temperature"] != 0.2 {
		t.Errorf("temperature = %#v, want the request's 0.2", body["temperature"])
	}
}

// Dispatch is by wire protocol, so one adapter serves every OpenAI-compatible
// vendor in the catalog.
func TestMuxDispatchesOnKind(t *testing.T) {
	oai := NewStubProvider([]StreamEvent{{Kind: EventTextDelta, TextDelta: "oai"}, {Kind: EventDone}})
	ant := NewStubProvider([]StreamEvent{{Kind: EventTextDelta, TextDelta: "ant"}, {Kind: EventDone}})
	m := NewMuxProvider(map[string]Provider{KindOpenAICompatible: oai, KindAnthropic: ant})

	res, err := Collect(t.Context(), m, Resolved{Provider: "pai", Kind: KindOpenAICompatible}, ChatRequest{})
	if err != nil || res.Text != "oai" {
		t.Fatalf("kind dispatch: got %q err=%v", res.Text, err)
	}
	if _, err := m.Stream(t.Context(), Resolved{Provider: "pai", Kind: "unknown"}, ChatRequest{}); err == nil {
		t.Fatal("want an error for an unregistered kind")
	}
}
