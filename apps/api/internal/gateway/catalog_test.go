package gateway

import (
	"context"
	"strings"
	"testing"

	"mindimprint/api/internal/config"
)

// The embedded catalog is what production boots from; a broken one must fail
// here rather than in a deploy.
func TestEmbeddedCatalogIsValid(t *testing.T) {
	cat, err := DefaultCatalog()
	if err != nil {
		t.Fatalf("embedded catalog invalid: %v", err)
	}
	if cat.Version == "" {
		t.Fatal("catalog declares no version")
	}
	for _, class := range activeClasses {
		spec, ok := cat.Lanes[class]
		if !ok {
			t.Errorf("class %q unbound", class)
			continue
		}
		if spec.Label == "" {
			t.Errorf("class %q has no label — --print-models would show a blank column", class)
		}
		if spec.LatencyBudgetMs <= 0 {
			t.Errorf("class %q states no latency budget, so routebench has nothing to score against", class)
		}
	}
	// The legacy lane names must keep resolving, because call sites migrate to
	// For(class) one class at a time rather than in a single 67-site commit.
	for _, lane := range []string{LaneChat, LaneFastChat, LaneEval} {
		if _, ok := cat.Lanes[ResolveClass(lane)]; !ok {
			t.Errorf("legacy lane %q no longer aliases a bound class", lane)
		}
	}
}

// The whole point of a class is that it carries its reasoning requirement to
// the wire. A chaperone-tier class that says nothing must keep the old
// tier-driven behaviour, or every not-yet-migrated call site changes cost
// silently the moment classes land.
func TestClassReasoningRequirementReachesTheRequestBody(t *testing.T) {
	body := `{"providers":{"p":{"kind":"openai_compatible","baseUrl":"u","apiKeyEnv":"K",
			"thinkingOff":{"enable_thinking":false},"reasoningEffortKey":"reasoning_effort"}},
		"models":{"p/m":{"provider":"p","model":"m","flagship":true,"defaultReasoningEffort":"high"}},
		"lanes":{"reflex":{"model":"p/m","tier":"chaperone","reasoning":"off"},
			"dialogue":{"model":"p/m","tier":"chaperone","reasoning":"off"},
			"compose":{"model":"p/m","tier":"chaperone","reasoning":"low"},
			"review":{"model":"p/m","tier":"flagship","reasoning":"default"},
			"assess":{"model":"p/m","tier":"flagship","reasoning":"max"},
			"digest":{"model":"p/m","tier":"chaperone","reasoning":"off"}}}`
	cat, err := ParseCatalog([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	keys := func(string) string { return "secret" }
	prov := NewCatalogProvider(nil)

	cases := []struct {
		class     string
		wantOff   bool
		wantEfort string
	}{
		{ClassDialogue, true, ""},
		{ClassCompose, false, "low"},
		// review says "default": the class has no opinion, so the MODEL's own
		// defaultReasoningEffort is what survives.
		{ClassReview, false, "high"},
		{ClassAssess, false, "max"},
	}
	for _, tc := range cases {
		t.Run(tc.class, func(t *testing.T) {
			r, err := cat.Resolve(tc.class, "", keys)
			if err != nil {
				t.Fatal(err)
			}
			got, err := prov.buildBody(r, ChatRequest{Messages: []ChatMessage{{Role: RoleUser, Content: "hi"}}})
			if err != nil {
				t.Fatal(err)
			}
			_, off := got["enable_thinking"]
			if off != tc.wantOff {
				t.Errorf("thinking-off present = %v, want %v (body: %#v)", off, tc.wantOff, got)
			}
			if tc.wantEfort == "" {
				if _, ok := got["reasoning_effort"]; ok {
					t.Errorf("thinking is off; a reasoning budget alongside it is meaningless: %#v", got)
				}
				return
			}
			if got["reasoning_effort"] != tc.wantEfort {
				t.Errorf("reasoning_effort = %v, want %q", got["reasoning_effort"], tc.wantEfort)
			}
		})
	}
}

// Fallback must not smuggle in a model the class already rejected. Falling back
// onto a model that cannot stop thinking turns a 4-second chaperone turn into a
// 40-second one — a worse outage than the one the fallback is covering for.
func TestFallbackSkipsModelsThatViolateTheClassRequirement(t *testing.T) {
	body := `{"providers":{
			"a":{"kind":"openai_compatible","baseUrl":"u","apiKeyEnv":"KA","thinkingOff":{"enable_thinking":false}},
			"b":{"kind":"openai_compatible","baseUrl":"u","apiKeyEnv":"KB","defaultModel":"b/always","thinkingOff":{"enable_thinking":false}}},
		"models":{
			"a/m":{"provider":"a","model":"m","flagship":true},
			"b/always":{"provider":"b","model":"always","flagship":true,"thinkingOffUnsupported":true}},
		"lanes":{"reflex":{"model":"a/m","tier":"chaperone","reasoning":"off"},
			"dialogue":{"model":"a/m","tier":"chaperone","reasoning":"off"},
			"compose":{"model":"a/m","tier":"chaperone"},
			"review":{"model":"a/m","tier":"flagship"},
			"assess":{"model":"a/m","tier":"flagship"},
			"digest":{"model":"a/m","tier":"chaperone","reasoning":"off"}},
		"fallbackProviders":["b"]}`
	cat, err := ParseCatalog([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	// Only provider b has a key, so the bound model is unreachable and fallback
	// is the only path left.
	onlyB := func(env string) string {
		if env == "KB" {
			return "secret"
		}
		return ""
	}
	if r, err := cat.Resolve(ClassDialogue, "", onlyB); err == nil {
		t.Errorf("dialogue fell back onto %q, which cannot stop reasoning", r.ModelID)
	}
	// compose states no reasoning requirement, so the same fallback is fine.
	if _, err := cat.Resolve(ClassCompose, "", onlyB); err != nil {
		t.Errorf("compose has no reasoning requirement and should still fall back: %v", err)
	}
}

// Every model must name a provider that exists and, if priced, price both
// directions — an unpriced-in/priced-out row would meter costs wrong.
func TestEmbeddedCatalogModelsAreCoherent(t *testing.T) {
	cat, _ := DefaultCatalog()
	for _, id := range cat.ModelIDs() {
		m := cat.Models[id]
		if _, ok := cat.Providers[m.Provider]; !ok {
			t.Errorf("%s: unknown provider %q", id, m.Provider)
		}
		if m.Price != nil && (m.Price.InputPerMillion <= 0 || m.Price.OutputPerMillion <= 0) {
			t.Errorf("%s: priced model must price both directions, got %+v", id, *m.Price)
		}
	}
}

func onlyKey(name, value string) KeyLookup {
	return func(env string) string {
		if env == name {
			return value
		}
		return ""
	}
}

// The default binding must reach DashScope, and it must keep serving deepseek-v4-pro
// so switching the platform onto the aggregator does not change the model.
func TestLanesDefaultToDashScopeDeepSeek(t *testing.T) {
	cat, _ := DefaultCatalog()
	keys := onlyKey("DASHSCOPE_API_KEY", "sk-dashscope")
	for _, lane := range []string{LaneChat, LaneFastChat, LaneEval} {
		r, err := cat.Resolve(lane, "", keys)
		if err != nil {
			t.Fatalf("lane %s: %v", lane, err)
		}
		if r.Provider != "dashscope" || r.Model != "deepseek-v4-pro" {
			t.Errorf("lane %s = %s/%s, want dashscope/deepseek-v4-pro", lane, r.Provider, r.Model)
		}
		if r.Kind != KindOpenAICompatible {
			t.Errorf("lane %s kind = %q", lane, r.Kind)
		}
		if r.APIKey != "sk-dashscope" {
			t.Errorf("lane %s: key not wired", lane)
		}
	}
}

// Tiers are what gate reasoning, so they must survive the move to the catalog.
func TestLaneTiersArePreserved(t *testing.T) {
	cat, _ := DefaultCatalog()
	keys := onlyKey("DASHSCOPE_API_KEY", "sk-dashscope")
	want := map[string]string{LaneChat: "chaperone", LaneFastChat: "chaperone", LaneEval: "flagship"}
	for lane, tier := range want {
		r, err := cat.Resolve(lane, "", keys)
		if err != nil {
			t.Fatal(err)
		}
		if r.Tier != tier {
			t.Errorf("lane %s tier = %q, want %q", lane, r.Tier, tier)
		}
	}
}

// With no DashScope key, the same wire model must still be reachable directly — this
// is what keeps local dev, CI, and a DashScope outage working.
func TestFallbackPrefersSameModelOnAnotherProvider(t *testing.T) {
	cat, _ := DefaultCatalog()
	r, err := cat.Resolve(LaneChat, "", onlyKey("DEEPSEEK_API_KEY", "sk-ds"))
	if err != nil {
		t.Fatal(err)
	}
	if r.Provider != "deepseek" || r.Model != "deepseek-v4-pro" {
		t.Fatalf("fallback = %s/%s, want deepseek/deepseek-v4-pro", r.Provider, r.Model)
	}
	// The direct route uses a DIFFERENT thinking knob than DashScope. Getting this
	// wrong silently re-enables reasoning on the chaperone lane.
	if _, ok := r.Policy.ThinkingOff["thinking"]; !ok {
		t.Errorf("direct deepseek must disable thinking via `thinking`, got %#v", r.Policy.ThinkingOff)
	}
}

func TestFallbackToAnthropicWhenOnlyKeyPresent(t *testing.T) {
	cat, _ := DefaultCatalog()
	r, err := cat.Resolve(LaneChat, "", onlyKey("ANTHROPIC_API_KEY", "sk-a"))
	if err != nil {
		t.Fatal(err)
	}
	if r.Provider != "anthropic" || r.Kind != KindAnthropic {
		t.Fatalf("fallback = %s (kind %s), want anthropic", r.Provider, r.Kind)
	}
}

func TestResolveErrorsWhenNoKeyConfigured(t *testing.T) {
	cat, _ := DefaultCatalog()
	if _, err := cat.Resolve(LaneChat, "", func(string) string { return "" }); err == nil {
		t.Fatal("want errNoProvider when nothing is configured")
	}
}

// The swap this whole change exists for: one lane moves, the others hold still.
func TestOverrideMovesOnlyTheNamedLane(t *testing.T) {
	cfg := config.Config{DashScopeKey: "sk-dashscope", ModelChat: "dashscope/qwen3.8-max"}
	rs, err := NewResolvers(cfg)
	if err != nil {
		t.Fatal(err)
	}
	chat, err := rs.Chat(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if chat.Model != "qwen3.8-max" {
		t.Errorf("chat model = %q, want qwen3.8-max", chat.Model)
	}
	eval, err := rs.Eval(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if eval.Model != "deepseek-v4-pro" {
		t.Errorf("eval model = %q — an override on one lane must not move another", eval.Model)
	}
}

// A typo must stop the boot, not serve the wrong model for a week.
func TestUnknownOverrideFailsAtBoot(t *testing.T) {
	_, err := NewResolvers(config.Config{DashScopeKey: "sk-dashscope", ModelChat: "dashscope/qwen-3.8-max"})
	if err == nil {
		t.Fatal("want boot error for a model id that is not in the catalog")
	}
	if !strings.Contains(err.Error(), "not a catalog model") {
		t.Errorf("error should name the problem and list known ids, got: %v", err)
	}
}

// 评估走旗舰模型绝不降级.
func TestEvalLaneRejectsNonFlagshipOverride(t *testing.T) {
	_, err := NewResolvers(config.Config{DashScopeKey: "sk-dashscope", ModelEval: "dashscope/qwen3.7-flash"})
	if err == nil {
		t.Fatal("eval lane must refuse a non-flagship model")
	}
	// The same model is fine on the chaperone lane.
	if _, err := NewResolvers(config.Config{DashScopeKey: "sk-dashscope", ModelChat: "dashscope/qwen3.7-flash"}); err != nil {
		t.Fatalf("chat lane may take a non-flagship model: %v", err)
	}
}

// Binding an image or embedding model to a chat lane fails at boot with the
// reason, rather than at the first student turn with a wire error.
func TestLaneRejectsNonChatModel(t *testing.T) {
	_, err := NewResolvers(config.Config{DashScopeKey: "sk-dashscope", ModelChat: "dashscope/qwen-image-3.0"})
	if err == nil {
		t.Fatal("a chat lane must refuse an image model")
	}
	if !strings.Contains(err.Error(), "not a chat model") {
		t.Errorf("error should say why, got: %v", err)
	}
}

// Missing keys must not stop the service booting — the lane simply errors per
// call, as it did before the catalog.
func TestNewResolversBootsWithNoKeys(t *testing.T) {
	rs, err := NewResolvers(config.Config{})
	if err != nil {
		t.Fatalf("no keys must not be a boot failure: %v", err)
	}
	if len(rs.Bindings) != 0 {
		t.Errorf("no keys ⇒ no bindings, got %v", rs.Bindings)
	}
	if _, err := rs.Chat(context.Background()); err == nil {
		t.Error("want a per-call error when no key is configured")
	}
}

// A brand-new model the vendor shipped this morning must be benchmarkable
// without a catalog edit first.
func TestResolveDirectAcceptsUndeclaredModel(t *testing.T) {
	cat, _ := DefaultCatalog()
	r, err := cat.ResolveDirect("dashscope", "qwen9-not-yet-catalogued", FlagshipTierName, onlyKey("DASHSCOPE_API_KEY", "sk-dashscope"))
	if err != nil {
		t.Fatal(err)
	}
	if r.BaseURL != cat.Providers["dashscope"].BaseURL {
		t.Error("undeclared model must inherit its provider's route")
	}
	if _, ok := r.Policy.ThinkingOff["enable_thinking"]; !ok {
		t.Error("undeclared model must inherit its provider's thinking policy")
	}
	if _, priced := LookupTokenPrice(r.Provider, r.Model); priced {
		t.Error("an undeclared model must be unpriced, not priced by accident")
	}
}

func TestResolveDirectRejectsUnknownProviderAndMissingKey(t *testing.T) {
	cat, _ := DefaultCatalog()
	if _, err := cat.ResolveDirect("nope", "m", "flagship", OSEnvKeyLookup); err == nil {
		t.Error("want error for unknown provider")
	}
	if _, err := cat.ResolveDirect("dashscope", "qwen3.8-max", "flagship", func(string) string { return "" }); err == nil {
		t.Error("want error when the provider's key is absent")
	}
}

// FlagshipTierName mirrors evalbench's tier constant without importing it.
const FlagshipTierName = "flagship"

func TestCatalogValidationRejectsBrokenCatalogs(t *testing.T) {
	cases := map[string]string{
		"unknown provider kind": `{"providers":{"p":{"kind":"carrier-pigeon","baseUrl":"u","apiKeyEnv":"K"}},
			"models":{"p/m":{"provider":"p","model":"m","flagship":true}},
			"lanes":{"reflex":{"model":"p/m","tier":"chaperone"},"dialogue":{"model":"p/m","tier":"chaperone"},"compose":{"model":"p/m","tier":"chaperone"},"review":{"model":"p/m","tier":"flagship"},"assess":{"model":"p/m","tier":"flagship"},"digest":{"model":"p/m","tier":"chaperone"}}}`,
		"lane binds unknown model": `{"providers":{"p":{"kind":"openai_compatible","baseUrl":"u","apiKeyEnv":"K"}},
			"models":{"p/m":{"provider":"p","model":"m","flagship":true}},
			"lanes":{"reflex":{"model":"p/m","tier":"chaperone"},"dialogue":{"model":"p/ghost","tier":"chaperone"},"compose":{"model":"p/m","tier":"chaperone"},"review":{"model":"p/m","tier":"flagship"},"assess":{"model":"p/m","tier":"flagship"},"digest":{"model":"p/m","tier":"chaperone"}}}`,
		"assess class not flagship": `{"providers":{"p":{"kind":"openai_compatible","baseUrl":"u","apiKeyEnv":"K"}},
			"models":{"p/m":{"provider":"p","model":"m"}},
			"lanes":{"reflex":{"model":"p/m","tier":"chaperone"},"dialogue":{"model":"p/m","tier":"chaperone"},"compose":{"model":"p/m","tier":"chaperone"},"review":{"model":"p/m","tier":"flagship"},"assess":{"model":"p/m","tier":"flagship"},"digest":{"model":"p/m","tier":"chaperone"}}}`,
		"model names unknown provider": `{"providers":{"p":{"kind":"openai_compatible","baseUrl":"u","apiKeyEnv":"K"}},
			"models":{"q/m":{"provider":"q","model":"m","flagship":true}},
			"lanes":{"reflex":{"model":"q/m","tier":"chaperone"},"dialogue":{"model":"q/m","tier":"chaperone"},"compose":{"model":"q/m","tier":"chaperone"},"review":{"model":"q/m","tier":"flagship"},"assess":{"model":"q/m","tier":"flagship"},"digest":{"model":"q/m","tier":"chaperone"}}}`,
		"lane binds a non-chat model": `{"providers":{"p":{"kind":"openai_compatible","baseUrl":"u","apiKeyEnv":"K"}},
			"models":{"p/m":{"provider":"p","model":"m","flagship":true,"capabilities":["image"]}},
			"lanes":{"reflex":{"model":"p/m","tier":"chaperone"},"dialogue":{"model":"p/m","tier":"chaperone"},"compose":{"model":"p/m","tier":"chaperone"},"review":{"model":"p/m","tier":"flagship"},"assess":{"model":"p/m","tier":"flagship"},"digest":{"model":"p/m","tier":"chaperone"}}}`,
		"missing class": `{"providers":{"p":{"kind":"openai_compatible","baseUrl":"u","apiKeyEnv":"K"}},
			"models":{"p/m":{"provider":"p","model":"m","flagship":true}},
			"lanes":{"dialogue":{"model":"p/m","tier":"chaperone"}}}`,
		// A class that must not reason, bound to a model that cannot stop. Today
		// this combination is caught only by a live test — or, on GLM-5.3 which
		// returns 400 rather than ignoring the field, by every student turn
		// erroring out. It has to fail at boot.
		"reasoning-off class on a model that always thinks": `{"providers":{"p":{"kind":"openai_compatible","baseUrl":"u","apiKeyEnv":"K"}},
			"models":{"p/m":{"provider":"p","model":"m","flagship":true,"thinkingOffUnsupported":true}},
			"lanes":{"reflex":{"model":"p/m","tier":"chaperone"},"dialogue":{"model":"p/m","tier":"chaperone","reasoning":"off"},"compose":{"model":"p/m","tier":"chaperone"},"review":{"model":"p/m","tier":"flagship"},"assess":{"model":"p/m","tier":"flagship"},"digest":{"model":"p/m","tier":"chaperone"}}}`,
		"unknown reasoning requirement": `{"providers":{"p":{"kind":"openai_compatible","baseUrl":"u","apiKeyEnv":"K"}},
			"models":{"p/m":{"provider":"p","model":"m","flagship":true}},
			"lanes":{"reflex":{"model":"p/m","tier":"chaperone"},"dialogue":{"model":"p/m","tier":"chaperone","reasoning":"ponder"},"compose":{"model":"p/m","tier":"chaperone"},"review":{"model":"p/m","tier":"flagship"},"assess":{"model":"p/m","tier":"flagship"},"digest":{"model":"p/m","tier":"chaperone"}}}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseCatalog([]byte(body)); err == nil {
				t.Fatalf("want validation error for %s", name)
			}
		})
	}
}

func TestModelPolicyMergePrefersModelOverProvider(t *testing.T) {
	body := `{"providers":{"p":{"kind":"openai_compatible","baseUrl":"u","apiKeyEnv":"K",
			"thinkingOff":{"enable_thinking":false},"reasoningEffortKey":"reasoning_effort","bodyExtra":{"top_p":0.9}}},
		"models":{"p/m":{"provider":"p","model":"m","flagship":true,
			"thinkingOff":{"thinking":{"type":"disabled"}},"bodyExtra":{"seed":7}}},
		"lanes":{"reflex":{"model":"p/m","tier":"chaperone"},"dialogue":{"model":"p/m","tier":"chaperone"},"compose":{"model":"p/m","tier":"chaperone"},"review":{"model":"p/m","tier":"flagship"},"assess":{"model":"p/m","tier":"flagship"},"digest":{"model":"p/m","tier":"chaperone"}}}`
	cat, err := ParseCatalog([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	pol := cat.policyFor(cat.Models["p/m"])
	if _, ok := pol.ThinkingOff["thinking"]; !ok {
		t.Error("model thinkingOff must win over the provider's")
	}
	if _, ok := pol.ThinkingOff["enable_thinking"]; ok {
		t.Error("model thinkingOff must REPLACE the provider's, not merge into it")
	}
	if pol.ReasoningEffortKey != "reasoning_effort" {
		t.Error("unset model fields must inherit from the provider")
	}
	if pol.BodyExtra["top_p"] != 0.9 || pol.BodyExtra["seed"] != float64(7) {
		t.Errorf("bodyExtra must merge key-by-key, got %#v", pol.BodyExtra)
	}
}

// A wrong class override must stop the boot, not serve the wrong model for a
// week. These are the ones that would otherwise be silent: a non-flagship model
// on 评估, and a model that cannot stop thinking on a class that must not think.
func TestClassOverrideFailsTheBootWhenItViolatesTheClass(t *testing.T) {
	withClass := func(class, model string) config.Config {
		return config.Config{DashScopeKey: "secret", ModelClass: map[string]string{class: model}}
	}
	cases := map[string]config.Config{
		"assess downgraded off flagship":         withClass(ClassAssess, "dashscope/qwen3.7-plus"),
		"dialogue on a model that always thinks": withClass(ClassDialogue, "dashscope/glm-5.3"),
		"typo'd model id":                        withClass(ClassCompose, "dashscope/qwen-does-not-exist"),
		"image model on a chat class":            withClass(ClassCompose, "dashscope/qwen-image-3.0"),
	}
	for name, cfg := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := NewResolvers(cfg); err == nil {
				t.Fatal("want a boot error")
			}
		})
	}
}

// The class variable must beat the legacy lane variable it replaced, so a
// half-migrated deployment resolves to the more specific instruction rather
// than to whichever happened to be read last.
func TestClassOverrideBeatsTheLegacyLaneVariable(t *testing.T) {
	cfg := config.Config{
		DashScopeKey: "secret",
		ModelChat:    "dashscope/qwen3.7-max",
		ModelClass:   map[string]string{ClassDialogue: "dashscope/kimi-k3"},
	}
	rs, err := NewResolvers(cfg)
	if err != nil {
		t.Fatal(err)
	}
	r, err := rs.For(ClassDialogue)(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if r.ModelID != "dashscope/kimi-k3" {
		t.Errorf("dialogue resolved to %q, want the class override to win", r.ModelID)
	}
	// The legacy alias must point at the same place, not at MODEL_CHAT.
	lr, lerr := rs.Chat(context.Background())
	if lerr != nil || lr.ModelID != r.ModelID {
		t.Errorf("legacy chat alias resolved to %q (err %v), want %q", lr.ModelID, lerr, r.ModelID)
	}
}
