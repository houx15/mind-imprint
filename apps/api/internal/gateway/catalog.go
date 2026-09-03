package gateway

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
)

// models.json is the single source of truth for which model serves each lane,
// what it costs, and how its reasoning is steered. It is embedded so the binary
// carries its own catalog — no file to ship, no path to configure.
//
//go:embed models.json
var modelsJSON []byte

// Capability classes. A class says how much INTELLIGENCE a call needs — not
// which product feature made it. Two calls on the same page can sit in
// different classes; two unrelated features can share one.
//
// The boundaries are drawn where they change the model choice. reflex and
// dialogue both stop reasoning, but reflex can run the cheapest flash model and
// dialogue cannot — Chinese fluency is something the student feels directly.
// compose and review both emit structured results, but compose derives from
// what the student ALREADY said (deterministic system work, explicitly outside
// 铁律② per AGENTS.md) while review passes judgement on what she did not say.
// assess is separate from review for exactly one reason: it must never be
// downgraded, and merging the two would let a downgrade experiment reach 评估.
const (
	ClassReflex   = "reflex"   // one label or one routing choice; no free text
	ClassDialogue = "dialogue" // a turn the student sees as it lands
	ClassCompose  = "compose"  // a schema-shaped artifact from stated inputs
	ClassReview   = "review"   // judges the student's work; being wrong costs
	ClassAssess   = "assess"   // rubric 评估 / 回顾 / 周报 — never downgraded
	ClassDigest   = "digest"   // long input, short output; compress, don't judge
	// ClassDraw generates an image. It is the one class that does NOT drive the
	// chat loop — it answers on /images/generations (see images.go), so validate()
	// checks it for the image capability instead of chat.
	ClassDraw     = "draw"
	ClassSearch   = "search"   // reserved: web search, needs the tool loop
	ClassMultimo  = "multimodal"
)

// Classes lists every declared class in report order.
var Classes = []string{
	ClassReflex, ClassDialogue, ClassCompose,
	ClassReview, ClassAssess, ClassDigest,
	ClassDraw,
	ClassSearch, ClassMultimo,
}

// activeClasses are the classes call sites may bind today. search and
// multimodal are declared so the catalog carries their intent, but nothing
// routes to them yet — see the spec's "不在本期".
var activeClasses = []string{
	ClassReflex, ClassDialogue, ClassCompose,
	ClassReview, ClassAssess, ClassDigest,
	ClassDraw,
}

// Legacy lane names, kept as aliases so MODEL_CHAT / MODEL_FAST_CHAT /
// MODEL_EVAL and every not-yet-migrated call site keep working while call sites
// move class by class.
const (
	LaneChat     = "chat"     // → dialogue
	LaneFastChat = "fastChat" // → reflex
	LaneEval     = "eval"     // → assess
)

// laneAliases maps a legacy lane name to the class that replaced it.
var laneAliases = map[string]string{
	LaneChat:     ClassDialogue,
	LaneFastChat: ClassReflex,
	LaneEval:     ClassAssess,
}

// Reasoning requirements a class may state. The class's requirement overrides
// the model's defaultReasoningEffort and is itself overridden by an explicit
// per-request ReasoningEffort.
const (
	ReasoningOff     = "off"     // force thinking off; the class cannot afford it
	ReasoningLow     = "low"     // bounded effort
	ReasoningHigh    = "high"    //
	ReasoningMax     = "max"     //
	ReasoningDefault = "default" // whatever the model does unprompted
)

func validReasoning(s string) bool {
	switch s {
	case "", ReasoningOff, ReasoningLow, ReasoningHigh, ReasoningMax, ReasoningDefault:
		return true
	}
	return false
}

// ResolveClass maps a legacy lane name onto its class, and passes a class
// through unchanged.
func ResolveClass(name string) string {
	if c, ok := laneAliases[name]; ok {
		return c
	}
	return name
}

// ClassEnvVar is the per-class model override variable: MODEL_DIALOGUE,
// MODEL_COMPOSE, and so on. One knob per class is the point — swapping one
// class at a time is what makes a measured speed or cost difference
// attributable to that class.
func ClassEnvVar(class string) string {
	return "MODEL_" + strings.ToUpper(class)
}

// Provider kinds — the wire protocol, not the vendor. Several vendors share a
// kind, which is why adding an OpenAI-compatible vendor needs no Go.
const (
	KindOpenAICompatible = "openai_compatible"
	KindAnthropic        = "anthropic"
	// KindDashScopeImage is the画图 route: DashScope's native multimodal
	// generation endpoint. It is NOT a chat protocol — no /chat/completions, no
	// streaming — so nothing but the draw class may route to it. 2026-09-04 实测：
	// 聊天那个 maas 聚合口 404s on /images/generations even though its GET /models
	// lists qwen-image-3.0. See images.go's file header.
	KindDashScopeImage = "dashscope_image"
)

// ModelPolicy is how a model's reasoning and body defaults are steered. It is
// data, not code, because the correct knob depends on the ROUTE as much as the
// model: deepseek-v4-pro honors thinking:{type:disabled} when called directly
// but silently ignores it through the PAI aggregator, where enable_thinking:false
// is the knob that works (measured 2026-09-02).
type ModelPolicy struct {
	// ThinkingOff is merged into the body to force reasoning off. Empty means
	// this route has no such knob.
	ThinkingOff map[string]any `json:"thinkingOff,omitempty"`
	// ThinkingOffUnsupported marks a model that rejects the thinking-off knob.
	// It does NOT mean the model always reasons — see MinReasoningEffort.
	ThinkingOffUnsupported bool `json:"thinkingOffUnsupported,omitempty"`
	// MinReasoningEffort is the lowest effort this route accepts, and stands in
	// for "off" on models that refuse the thinking-off knob.
	//
	// 🚨 This field exists because treating ThinkingOffUnsupported as "cannot
	// stop reasoning" was wrong, and the mistake cost glm-5.3 three whole
	// classes. DashScope's own 400 says what to do instead:
	//
	//	code 1210: 该模型始终思考，不支持关闭思考；请使用 low、high 或 max。
	//
	// Measured 2026-09-03: ZHIPU/GLM-5.3 with reasoning_effort:"low" returns
	// ZERO reasoning tokens and a normal reply. That is thinking-off in every
	// sense the capability classes care about — reached through a different
	// knob, which is exactly the kind of per-route difference this struct is
	// data rather than code in order to absorb.
	MinReasoningEffort string `json:"minReasoningEffort,omitempty"`
	// RequiresUserMessage marks a route that rejects a system-only message list.
	//
	// 🚨 Measured 2026-09-03: ZHIPU/GLM-5.3 answers `messages 参数非法` (400,
	// code 1214) to a request carrying only a system message, while every other
	// model in the catalog accepts it. internal/pbl.GenerateLookback sends
	// exactly that shape, so pointing MODEL_ASSESS at a GLM model would have
	// 400'd in production — breaking the catalog's whole promise that swapping a
	// model is an env var and not a code change.
	RequiresUserMessage bool `json:"requiresUserMessage,omitempty"`
	// ReasoningEffortKey is the body key carrying a bounded effort ("low"/"max").
	// Empty means the route ignores effort, so we do not send it.
	ReasoningEffortKey string `json:"reasoningEffortKey,omitempty"`
	// ReasoningEffortAliases translates OUR vocabulary (off/low/high/max) into
	// the words this particular model accepts.
	//
	// 🚨 reasoning_effort is not one enum. Measured across the catalog on
	// 2026-09-03 with a 400/ok probe per (model, value):
	//
	//	qwen3.7-max/-plus/-flash, kimi-k2.6   max → 400,   xhigh → ok
	//	ZHIPU/GLM-5.3                         xhigh → 400, max   → ok
	//	qwen3.8-*, deepseek-v4-*, kimi-k3, glm-5.2   both ok
	//
	// This cost an entire benchmark round: the assess class sends "max", the
	// judge was qwen3.7-max, so EVERY judge call 400'd and the run produced a
	// recommendation table with no quality data behind it. Re-derive the matrix
	// with cmd/routebench rather than assuming a new model shares a vocabulary
	// with its own siblings — qwen3.7 and qwen3.8 do not.
	ReasoningEffortAliases map[string]string `json:"reasoningEffortAliases,omitempty"`
	// BodyExtra is merged into every request; BodyExtraWithTools only when the
	// turn carries tools.
	BodyExtra          map[string]any `json:"bodyExtra,omitempty"`
	BodyExtraWithTools map[string]any `json:"bodyExtraWithTools,omitempty"`
	// DefaultTemperature applies when the request does not set one.
	DefaultTemperature *float64 `json:"defaultTemperature,omitempty"`
	// SearchSupported marks a route that accepts enable_search / search_options.
	// Declared on the provider, because it is the channel that offers the
	// feature, not the model.
	SearchSupported bool `json:"searchSupported,omitempty"`
}

// canStopReasoning reports whether this route can be made to answer without
// reasoning — either by the thinking-off knob, or by dropping to the lowest
// effort the route accepts.
//
// The distinction matters: "rejects enable_thinking:false" and "always burns a
// reasoning budget" are different properties, and conflating them excluded
// glm-5.3 from reflex/dialogue/digest for a whole benchmark round on a
// constraint it does not actually have.
func canStopReasoning(p ModelPolicy) bool {
	return !p.ThinkingOffUnsupported || p.MinReasoningEffort != ""
}

// merge overlays non-zero fields of over onto p. Model-level policy wins over
// provider-level policy field by field, so a model overrides only what differs.
func (p ModelPolicy) merge(over ModelPolicy) ModelPolicy {
	out := p
	if len(over.ThinkingOff) > 0 {
		out.ThinkingOff = over.ThinkingOff
	}
	if over.ThinkingOffUnsupported {
		out.ThinkingOffUnsupported = true
	}
	if over.MinReasoningEffort != "" {
		out.MinReasoningEffort = over.MinReasoningEffort
	}
	if over.RequiresUserMessage {
		out.RequiresUserMessage = true
	}
	if over.SearchSupported {
		out.SearchSupported = true
	}
	if over.ReasoningEffortKey != "" {
		out.ReasoningEffortKey = over.ReasoningEffortKey
	}
	if len(over.ReasoningEffortAliases) > 0 {
		merged := make(map[string]string, len(out.ReasoningEffortAliases)+len(over.ReasoningEffortAliases))
		for k, v := range out.ReasoningEffortAliases {
			merged[k] = v
		}
		for k, v := range over.ReasoningEffortAliases {
			merged[k] = v
		}
		out.ReasoningEffortAliases = merged
	}
	if len(over.BodyExtra) > 0 {
		out.BodyExtra = mergeMaps(out.BodyExtra, over.BodyExtra)
	}
	if len(over.BodyExtraWithTools) > 0 {
		out.BodyExtraWithTools = mergeMaps(out.BodyExtraWithTools, over.BodyExtraWithTools)
	}
	if over.DefaultTemperature != nil {
		out.DefaultTemperature = over.DefaultTemperature
	}
	return out
}

func mergeMaps(base, over map[string]any) map[string]any {
	out := make(map[string]any, len(base)+len(over))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range over {
		out[k] = v
	}
	return out
}

// priceSpec is the catalog's USD-per-million wire shape. Absent ⇒ unpriced, and
// an unpriced model records no cost rather than an invented one.
type priceSpec struct {
	InputPerMillion  float64 `json:"inputPerMillion"`
	OutputPerMillion float64 `json:"outputPerMillion"`
}

// ProviderSpec is one route to models: a base URL, the env var holding its key,
// and the wire protocol its adapter speaks.
type ProviderSpec struct {
	Kind         string `json:"kind"`
	Label        string `json:"label"`
	BaseURL      string `json:"baseUrl"`
	APIKeyEnv    string `json:"apiKeyEnv"`
	DefaultModel string `json:"defaultModel"`
	ModelPolicy
}

// Model capabilities. A lane binds only a CapChat model; the rest are catalog
// metadata today, describing what this key can reach so a future coding, vision
// or image feature is a JSON edit plus its own adapter — not a re-survey of the
// vendor.
//
// CapImage / CapVideo / CapEmbedding / CapRerank models are NOT reachable
// through the chat adapter: they answer on different endpoints
// (/images/generations, /embeddings, /rerank) and will need their own provider
// Kind when a feature actually calls for them.
const (
	CapChat      = "chat"      // /chat/completions
	CapTools     = "tools"     // function calling — required by the card/tool loop
	CapVision    = "vision"    // accepts image input in chat messages
	CapCoding    = "coding"    // tuned for code
	CapImage     = "image"     // image generation
	CapVideo     = "video"     // video generation
	CapEmbedding = "embedding" // vector embeddings
	CapRerank    = "rerank"    // relevance reranking
)

// ModelSpec is one bindable model: which provider serves it, the name to put on
// the wire, whether it may serve the flagship lane, and what it costs.
type ModelSpec struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	Label    string `json:"label"`
	Flagship bool   `json:"flagship"`
	// Capabilities defaults to [chat, tools] when omitted, which is what the
	// overwhelming majority of entries are.
	Capabilities           []string   `json:"capabilities"`
	DefaultReasoningEffort string     `json:"defaultReasoningEffort"`
	Price                  *priceSpec `json:"priceUsd"`
	ModelPolicy
}

// Has reports whether the model declares a capability.
func (m ModelSpec) Has(cap string) bool {
	for _, c := range m.caps() {
		if c == cap {
			return true
		}
	}
	return false
}

func (m ModelSpec) caps() []string {
	if len(m.Capabilities) == 0 {
		return []string{CapChat, CapTools}
	}
	return m.Capabilities
}

// LaneSpec binds one capability class to a model. Everything except Model
// describes what the CLASS requires, so re-pointing a class at another model is
// a one-line edit that carries its requirements with it.
type LaneSpec struct {
	Model string `json:"model"`
	Tier  string `json:"tier"`
	Label string `json:"label,omitempty"`
	// Reasoning is this class's requirement — see the Reasoning* constants.
	// Empty means "default", which keeps pre-class catalogs behaving as before.
	Reasoning string `json:"reasoning,omitempty"`
	// LatencyBudgetMs is a TARGET, not a timeout. routebench scores against it
	// and --print-models shows it; nothing enforces it at runtime, because a
	// slow answer is better than no answer mid-turn.
	LatencyBudgetMs int `json:"latencyBudgetMs,omitempty"`
}

// Catalog is the parsed, validated models.json.
type Catalog struct {
	Version           string                  `json:"version"`
	Providers         map[string]ProviderSpec `json:"providers"`
	Models            map[string]ModelSpec    `json:"models"`
	Lanes             map[string]LaneSpec     `json:"lanes"`
	FallbackProviders []string                `json:"fallbackProviders"`
}

// KeyLookup returns the secret held by an env var name, or "" when absent. It is
// an injected seam so tests stay hermetic and so a caller can prefer typed
// config over the process environment.
type KeyLookup func(envVar string) string

// OSEnvKeyLookup reads the process environment. Used for a brand-new provider
// whose key has no typed config field yet.
func OSEnvKeyLookup(envVar string) string { return os.Getenv(envVar) }

var (
	defaultCatalogOnce sync.Once
	defaultCatalog     *Catalog
	defaultCatalogErr  error
)

// DefaultCatalog parses the embedded models.json once. A malformed catalog is a
// fatal boot error, never a silent fallback to some other model.
func DefaultCatalog() (*Catalog, error) {
	defaultCatalogOnce.Do(func() {
		defaultCatalog, defaultCatalogErr = ParseCatalog(modelsJSON)
	})
	return defaultCatalog, defaultCatalogErr
}

// ParseCatalog parses and validates catalog bytes.
func ParseCatalog(b []byte) (*Catalog, error) {
	var c Catalog
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("gateway catalog: parse: %w", err)
	}
	if err := c.validate(); err != nil {
		return nil, err
	}
	return &c, nil
}

// validate fails the boot on any catalog that could resolve to a wrong or
// absent model at runtime. Every check here is one that would otherwise surface
// as a confusing mid-session provider error.
func (c *Catalog) validate() error {
	if len(c.Providers) == 0 || len(c.Models) == 0 {
		return fmt.Errorf("gateway catalog: no providers or models declared")
	}
	for id, p := range c.Providers {
		switch p.Kind {
		case KindOpenAICompatible, KindAnthropic, KindDashScopeImage:
		default:
			return fmt.Errorf("gateway catalog: provider %q has unknown kind %q", id, p.Kind)
		}
		if p.BaseURL == "" || p.APIKeyEnv == "" {
			return fmt.Errorf("gateway catalog: provider %q needs baseUrl and apiKeyEnv", id)
		}
		if p.DefaultModel != "" {
			if _, ok := c.Models[p.DefaultModel]; !ok {
				return fmt.Errorf("gateway catalog: provider %q defaultModel %q is not a declared model", id, p.DefaultModel)
			}
		}
	}
	for id, m := range c.Models {
		if _, ok := c.Providers[m.Provider]; !ok {
			return fmt.Errorf("gateway catalog: model %q names unknown provider %q", id, m.Provider)
		}
		if m.Model == "" {
			return fmt.Errorf("gateway catalog: model %q has no wire model name", id)
		}
	}
	for _, class := range activeClasses {
		if _, ok := c.Lanes[class]; !ok {
			return fmt.Errorf("gateway catalog: class %q is not bound", class)
		}
	}
	for _, class := range sortedKeys(c.Lanes) {
		spec := c.Lanes[class]
		m, ok := c.Models[spec.Model]
		if !ok {
			return fmt.Errorf("gateway catalog: class %q binds unknown model %q", class, spec.Model)
		}
		if spec.Tier == "" {
			return fmt.Errorf("gateway catalog: class %q has no tier", class)
		}
		if !validReasoning(spec.Reasoning) {
			return fmt.Errorf("gateway catalog: class %q has unknown reasoning %q (want off/low/high/max/default)",
				class, spec.Reasoning)
		}
		// 🚨 draw 是唯一一个不跑对话循环的档：它答在 /images/generations 上
		// （见 images.go）。所以它查的是画图能力，而给它绑一个聊天模型同样是
		// 启动即失败——不然第一次生成头图才发现，而那时错的是学生的那一屏。
		if class == ClassDraw {
			if !m.Has(CapImage) {
				return fmt.Errorf("gateway catalog: draw class binds %q, which is not an image model (capabilities: %s)",
					spec.Model, strings.Join(m.caps(), ", "))
			}
		} else if !m.Has(CapChat) {
			// A class drives the chat/tool loop. Binding an image, embedding or
			// rerank model here would fail obscurely at the first turn; fail at boot
			// with the reason instead.
			return fmt.Errorf("gateway catalog: class %q binds %q, which is not a chat model (capabilities: %s)",
				class, spec.Model, strings.Join(m.caps(), ", "))
		}
		// 评估走旗舰模型绝不降级 — enforced here so a catalog edit cannot quietly
		// downgrade evaluation.
		if class == ClassAssess && !m.Flagship {
			return fmt.Errorf("gateway catalog: assess class model %q is not flagship — 评估绝不降级", spec.Model)
		}
		// A class that requires thinking OFF cannot be served by a model that has
		// no way to stop reasoning at all. Caught at boot rather than by every
		// student turn erroring out.
		if spec.Reasoning == ReasoningOff && !canStopReasoning(c.policyFor(m)) {
			return fmt.Errorf("gateway catalog: class %q requires reasoning off, but model %q cannot stop reasoning "+
				"(thinkingOffUnsupported, and no minReasoningEffort to stand in for off) — "+
				"pick another model or relax the class to a bounded effort",
				class, spec.Model)
		}
	}
	for name, class := range laneAliases {
		if _, ok := c.Lanes[class]; !ok {
			return fmt.Errorf("gateway catalog: legacy lane %q aliases class %q, which is not bound", name, class)
		}
	}
	for _, p := range c.FallbackProviders {
		if _, ok := c.Providers[p]; !ok {
			return fmt.Errorf("gateway catalog: fallbackProviders names unknown provider %q", p)
		}
	}
	return nil
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// ModelIDs returns catalog model ids in stable order.
func (c *Catalog) ModelIDs() []string {
	ids := make([]string, 0, len(c.Models))
	for id := range c.Models {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// policyFor merges provider policy with the model's overrides.
func (c *Catalog) policyFor(m ModelSpec) ModelPolicy {
	return c.Providers[m.Provider].ModelPolicy.merge(m.ModelPolicy)
}

// resolveModel builds a Resolved for one catalog model id, or ok=false when its
// provider has no key in this environment.
func (c *Catalog) resolveModel(id string, spec LaneSpec, keys KeyLookup) (Resolved, bool) {
	m, ok := c.Models[id]
	if !ok {
		return Resolved{}, false
	}
	p := c.Providers[m.Provider]
	key := keys(p.APIKeyEnv)
	if key == "" {
		return Resolved{}, false
	}
	// A bounded-effort class states its own effort; the model's default applies
	// only where the class has no opinion. "off" is carried in Reasoning rather
	// than folded into effort, because off and low are different mechanisms
	// (a thinkingOff body fragment vs a reasoning_effort key).
	effort := m.DefaultReasoningEffort
	switch spec.Reasoning {
	case ReasoningLow, ReasoningHigh, ReasoningMax:
		effort = spec.Reasoning
	}
	return Resolved{
		Provider:               m.Provider,
		Kind:                   p.Kind,
		BaseURL:                p.BaseURL,
		Model:                  m.Model,
		ModelID:                id,
		APIKey:                 key,
		Tier:                   spec.Tier,
		Reasoning:              spec.Reasoning,
		DefaultReasoningEffort: effort,
		Policy:                 c.policyFor(m),
	}, true
}

// candidatesFor lists the model ids to try for a lane, in order: the bound model
// first, then the same wire model under each fallback provider, then that
// provider's declared default. The eval lane only ever considers flagship models
// so a fallback cannot downgrade evaluation.
func (c *Catalog) candidatesFor(class, modelID string) []string {
	out := []string{modelID}
	seen := map[string]bool{modelID: true}
	wire := c.Models[modelID].Model
	primary := c.Models[modelID].Provider

	add := func(id string) {
		if id == "" || seen[id] {
			return
		}
		m, ok := c.Models[id]
		if !ok {
			return
		}
		if class == ClassAssess && !m.Flagship {
			return
		}
		// A fallback must not violate the class's reasoning requirement either:
		// falling back onto a model with no way to stop thinking would turn a
		// 4-second chaperone turn into a 40-second one. Skipping it here keeps
		// fallback a smaller change than the outage it is covering for.
		if c.Lanes[class].Reasoning == ReasoningOff && !canStopReasoning(c.policyFor(m)) {
			return
		}
		seen[id] = true
		out = append(out, id)
	}
	for _, pid := range c.FallbackProviders {
		if pid == primary {
			continue
		}
		// Prefer the identical model reached another way — the smallest possible
		// behavior change when the primary route is unavailable.
		for _, id := range c.ModelIDs() {
			if m := c.Models[id]; m.Provider == pid && m.Model == wire {
				add(id)
			}
		}
		add(c.Providers[pid].DefaultModel)
	}
	return out
}

// ResolveDirect resolves a provider + wire model name straight to a Resolved,
// bypassing lanes. It backs the offline workbench, where the point is to point
// a benchmark at an arbitrary model.
//
// A model that is NOT declared in the catalog still resolves: it inherits its
// provider's policy and is simply unpriced. That is deliberate — trying a model
// the vendor shipped this morning should not require a catalog edit first.
// Declaring it later adds its price and any per-model policy.
func (c *Catalog) ResolveDirect(providerID, wireModel, tier string, keys KeyLookup) (Resolved, error) {
	p, ok := c.Providers[providerID]
	if !ok {
		known := make([]string, 0, len(c.Providers))
		for id := range c.Providers {
			known = append(known, id)
		}
		sort.Strings(known)
		return Resolved{}, fmt.Errorf("unknown provider %q (known: %s)", providerID, strings.Join(known, ", "))
	}
	if wireModel == "" {
		return Resolved{}, fmt.Errorf("provider %q: no model named", providerID)
	}
	key := keys(p.APIKeyEnv)
	if key == "" {
		return Resolved{}, fmt.Errorf("missing API key for provider %q (set %s)", providerID, p.APIKeyEnv)
	}

	policy := p.ModelPolicy
	modelID, effort := "", ""
	for _, id := range c.ModelIDs() {
		if m := c.Models[id]; m.Provider == providerID && m.Model == wireModel {
			policy, effort, modelID = c.policyFor(m), m.DefaultReasoningEffort, id
			break
		}
	}
	if modelID == "" {
		modelID = providerID + "/" + wireModel
	}
	return Resolved{
		Provider:               providerID,
		Kind:                   p.Kind,
		BaseURL:                p.BaseURL,
		Model:                  wireModel,
		ModelID:                modelID,
		APIKey:                 key,
		Tier:                   tier,
		DefaultReasoningEffort: effort,
		Policy:                 policy,
	}, nil
}

// Resolve picks the model for a lane. override, when non-empty, names a catalog
// model id and takes precedence over the catalog's binding — this is the
// MODEL_CHAT / MODEL_FAST_CHAT / MODEL_EVAL swap. An override naming an unknown
// model, or a non-flagship model on the eval lane, is a hard error: a typo must
// fail loudly at boot rather than quietly serve the wrong model for a week.
func (c *Catalog) Resolve(lane, override string, keys KeyLookup) (Resolved, error) {
	class := ResolveClass(lane)
	spec, ok := c.Lanes[class]
	if !ok {
		return Resolved{}, fmt.Errorf("gateway: unknown class %q (known: %s)", lane, strings.Join(activeClasses, ", "))
	}
	modelID := spec.Model
	if override != "" {
		m, ok := c.Models[override]
		if !ok {
			return Resolved{}, fmt.Errorf("gateway: class %q override %q is not a catalog model (known: %s)",
				class, override, strings.Join(c.ModelIDs(), ", "))
		}
		if !m.Has(CapChat) {
			return Resolved{}, fmt.Errorf("gateway: class %q override %q is not a chat model (capabilities: %s)",
				class, override, strings.Join(m.caps(), ", "))
		}
		if class == ClassAssess && !m.Flagship {
			return Resolved{}, fmt.Errorf("gateway: class %q override %q is not flagship — 评估绝不降级", class, override)
		}
		if spec.Reasoning == ReasoningOff && !canStopReasoning(c.policyFor(m)) {
			return Resolved{}, fmt.Errorf("gateway: class %q requires reasoning off, but override %q cannot stop "+
				"reasoning (thinkingOffUnsupported, and no minReasoningEffort to stand in for off)", class, override)
		}
		modelID = override
	}
	for _, id := range c.candidatesFor(class, modelID) {
		if r, ok := c.resolveModel(id, spec, keys); ok {
			return r, nil
		}
	}
	return Resolved{}, errNoProvider
}
