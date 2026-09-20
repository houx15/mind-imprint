package gateway

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"mindimprint/api/internal/config"
)

// KeyResolver resolves, per request, which provider/model/key to use. The seam
// exists so future per-org billing can resolve the caller's school key without
// changing the rest of the gateway.
type KeyResolver func(ctx context.Context) (Resolved, error)

// errNoProvider is the client-safe configuration error (no secret).
var errNoProvider = errors.New("no LLM provider configured")

// Resolvers holds one resolver per capability class, already bound to the
// catalog and to this process's env overrides.
type Resolvers struct {
	byClass map[string]KeyResolver

	// Chat / FastChat / Eval are the legacy lane aliases, kept so call sites can
	// migrate to For(class) one class at a time instead of in a single 67-site
	// commit. They point at dialogue / reflex / assess respectively.
	Chat     KeyResolver
	FastChat KeyResolver
	Eval     KeyResolver

	// Bindings records which catalog model each class resolved to, for
	// --print-models and boot logging. Absent for a class with no key.
	Bindings map[string]Resolved
}

// For returns the resolver for a capability class. A legacy lane name resolves
// through its alias. An unknown name returns a resolver that errors on use
// rather than nil, so a typo surfaces as a named error at the call rather than
// as a nil-pointer panic mid-turn.
func (rs Resolvers) For(class string) KeyResolver {
	if r, ok := rs.byClass[ResolveClass(class)]; ok {
		return r
	}
	return func(context.Context) (Resolved, error) {
		return Resolved{}, fmt.Errorf("gateway: no resolver for class %q", class)
	}
}

// configKeyLookup prefers the typed config field for a known provider env var
// and falls back to the process environment. The fallback is what keeps
// "new provider = one JSON entry" true: a brand-new vendor's key works from the
// environment before anyone adds a config field for it.
func configKeyLookup(cfg config.Config) KeyLookup {
	typed := map[string]string{
		"DASHSCOPE_API_KEY": cfg.DashScopeKey,
		"DEEPSEEK_API_KEY":  cfg.DeepSeekKey,
		"ANTHROPIC_API_KEY": cfg.AnthropicKey,
		"ZAI_API_KEY":       cfg.ZAIKey,
	}
	return func(envVar string) string {
		if v, ok := typed[envVar]; ok {
			return v
		}
		return OSEnvKeyLookup(envVar)
	}
}

// NewResolvers binds all three lanes from the embedded catalog, applying the
// per-lane MODEL_CHAT / MODEL_FAST_CHAT / MODEL_EVAL overrides.
//
// A model swap is therefore one env var and a restart — no rebuild, no code
// edit — which is the whole point: comparing models on ability, speed and cost
// requires changing one lane at a time and leaving the others fixed.
//
// A bad override fails HERE, at boot, rather than at the first student turn.
// Missing keys do not fail: a lane with no usable key returns errNoProvider per
// call, exactly as before, so the service still boots for non-LLM work.
func NewResolvers(cfg config.Config) (Resolvers, error) {
	cat, err := DefaultCatalog()
	if err != nil {
		return Resolvers{}, err
	}
	keys := configKeyLookup(cfg)
	overrides := classOverrides(cfg)

	out := Resolvers{byClass: map[string]KeyResolver{}, Bindings: map[string]Resolved{}}
	for _, class := range sortedKeys(cat.Lanes) {
		r, err := cat.Resolve(class, overrides[class], keys)
		switch {
		case err == nil:
			out.Bindings[class] = r
		case errors.Is(err, errNoProvider):
			// No key for this class in this environment — resolve-time error.
		default:
			// A typo'd override or a broken catalog: refuse to boot.
			return Resolvers{}, err
		}
		out.byClass[class] = laneResolver(cat, class, overrides[class], keys)
	}
	out.Chat = out.For(LaneChat)
	out.FastChat = out.For(LaneFastChat)
	out.Eval = out.For(LaneEval)
	return out, nil
}

// classOverrides collects the per-class MODEL_* variables. Precedence, most
// specific first: cfg.ModelClass (an injected map, for tests) → MODEL_<CLASS>
// from the environment → the legacy lane variable this class replaced.
//
// The legacy variable coming LAST is what makes a half-migrated deployment
// behave predictably: an operator who has already set MODEL_DIALOGUE gets it,
// even with a stale MODEL_CHAT still exported next to it.
func classOverrides(cfg config.Config) map[string]string {
	legacy := map[string]string{
		ClassDialogue: cfg.ModelChat,
		ClassReflex:   cfg.ModelFastChat,
		ClassAssess:   cfg.ModelEval,
	}
	out := map[string]string{}
	for _, class := range Classes {
		switch {
		case cfg.ModelClass[class] != "":
			out[class] = cfg.ModelClass[class]
		case os.Getenv(ClassEnvVar(class)) != "":
			out[class] = os.Getenv(ClassEnvVar(class))
		default:
			out[class] = legacy[class]
		}
	}
	return out
}

// laneResolver resolves on every call rather than caching, so a key rotated
// into the environment takes effect without a restart and so the seam stays
// per-request for future per-org billing.
func laneResolver(cat *Catalog, lane, override string, keys KeyLookup) KeyResolver {
	return func(_ context.Context) (Resolved, error) {
		return cat.Resolve(lane, override, keys)
	}
}

// DescribeBindings renders the active lane→model bindings and the full catalog
// for `api --print-models`. It prints no secrets — only which env var each
// provider reads and whether that var is currently set.
func DescribeBindings(cfg config.Config) string {
	var b strings.Builder
	cat, err := DefaultCatalog()
	if err != nil {
		fmt.Fprintf(&b, "catalog: %v\n", err)
		return b.String()
	}
	keys := configKeyLookup(cfg)

	fmt.Fprintf(&b, "catalog %s\n\nCAPABILITY CLASSES\n", cat.Version)
	overrides := classOverrides(cfg)
	for _, class := range Classes {
		spec, bound := cat.Lanes[class]
		if !bound {
			continue
		}
		src := "catalog default"
		if overrides[class] != "" {
			src = ClassEnvVar(class) + " override"
		}
		budget := "—"
		if spec.LatencyBudgetMs > 0 {
			budget = fmt.Sprintf("%.1fs", float64(spec.LatencyBudgetMs)/1000)
		}
		reserved := ""
		if class == ClassSearch || class == ClassMultimo {
			reserved = "  [reserved — nothing routes here yet]"
		}
		fmt.Fprintf(&b, "  %-11s think=%-7s budget=%-7s %s%s\n",
			class, orDefault(spec.Reasoning, "default"), budget, spec.Label, reserved)
		r, err := cat.Resolve(class, overrides[class], keys)
		if err != nil {
			fmt.Fprintf(&b, "  %-11s → %-28s tier=%-9s UNRESOLVED (%v)\n\n", "", spec.Model, spec.Tier, err)
			continue
		}
		fmt.Fprintf(&b, "  %-11s → %-28s tier=%-9s via %s (%s)\n\n", "", r.ModelID, r.Tier, r.Provider, src)
	}
	fmt.Fprintf(&b, "  legacy aliases: chat→dialogue  fastChat→reflex  eval→assess\n")

	fmt.Fprintf(&b, "\nPROVIDERS\n")
	pids := make([]string, 0, len(cat.Providers))
	for id := range cat.Providers {
		pids = append(pids, id)
	}
	sort.Strings(pids)
	for _, id := range pids {
		p := cat.Providers[id]
		state := "NO KEY"
		if keys(p.APIKeyEnv) != "" {
			state = "key set"
		}
		fmt.Fprintf(&b, "  %-10s %-18s %-8s %s\n", id, p.APIKeyEnv, state, p.BaseURL)
	}

	fmt.Fprintf(&b, "\nMODELS (bind with MODEL_REFLEX / MODEL_DIALOGUE / MODEL_COMPOSE / MODEL_REVIEW / MODEL_ASSESS / MODEL_DIGEST)\n")
	for _, id := range cat.ModelIDs() {
		m := cat.Models[id]
		// Printed in the currency the bill is written in, then in the USD the
		// cost column stores — so a reader can check a rate against the vendor's
		// price list without doing the conversion in their head.
		price := "UNPRICED — set priceUsd or priceCny in models.json to compare cost"
		switch {
		case m.Price != nil:
			price = fmt.Sprintf("$%.3f in / $%.3f out per 1M", m.Price.InputPerMillion, m.Price.OutputPerMillion)
		case m.PriceCNY != nil:
			price = fmt.Sprintf("¥%.2f in / ¥%.2f out per 1M  ($%.3f / $%.3f)",
				m.PriceCNY.InputPerMillion, m.PriceCNY.OutputPerMillion,
				m.PriceCNY.InputPerMillion/CNYPerUSD, m.PriceCNY.OutputPerMillion/CNYPerUSD)
		}
		flag := " "
		if m.Flagship {
			flag = "*"
		}
		fmt.Fprintf(&b, "  %s %-28s %s\n", flag, id, price)
	}
	fmt.Fprintf(&b, "\n  * = flagship-eligible (the assess class accepts only these)\n")
	return b.String()
}

// orDefault renders an empty catalog value as the word the catalog means by it,
// so --print-models never shows a blank column the reader has to interpret.
func orDefault(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}
