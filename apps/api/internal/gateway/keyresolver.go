package gateway

import (
	"context"
	"errors"
	"fmt"
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

// Resolvers holds one resolver per lane, already bound to the catalog and to
// this process's env overrides.
type Resolvers struct {
	Chat     KeyResolver
	FastChat KeyResolver
	Eval     KeyResolver
	// Bindings records which catalog model each lane resolved to, for
	// --print-models and boot logging. Absent for a lane with no key.
	Bindings map[string]Resolved
}

// configKeyLookup prefers the typed config field for a known provider env var
// and falls back to the process environment. The fallback is what keeps
// "new provider = one JSON entry" true: a brand-new vendor's key works from the
// environment before anyone adds a config field for it.
func configKeyLookup(cfg config.Config) KeyLookup {
	typed := map[string]string{
		"PAI_API_KEY":       cfg.PAIKey,
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
	overrides := map[string]string{
		LaneChat:     cfg.ModelChat,
		LaneFastChat: cfg.ModelFastChat,
		LaneEval:     cfg.ModelEval,
	}

	out := Resolvers{Bindings: map[string]Resolved{}}
	for _, lane := range []string{LaneChat, LaneFastChat, LaneEval} {
		r, err := cat.Resolve(lane, overrides[lane], keys)
		switch {
		case err == nil:
			out.Bindings[lane] = r
		case errors.Is(err, errNoProvider):
			// No key for this lane in this environment — resolve-time error.
		default:
			// A typo'd override or a broken catalog: refuse to boot.
			return Resolvers{}, err
		}
		out.set(lane, laneResolver(cat, lane, overrides[lane], keys))
	}
	return out, nil
}

func (rs *Resolvers) set(lane string, r KeyResolver) {
	switch lane {
	case LaneChat:
		rs.Chat = r
	case LaneFastChat:
		rs.FastChat = r
	case LaneEval:
		rs.Eval = r
	}
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

	fmt.Fprintf(&b, "catalog %s\n\nACTIVE LANES\n", cat.Version)
	overrides := map[string]string{
		LaneChat:     cfg.ModelChat,
		LaneFastChat: cfg.ModelFastChat,
		LaneEval:     cfg.ModelEval,
	}
	envVar := map[string]string{
		LaneChat:     "MODEL_CHAT",
		LaneFastChat: "MODEL_FAST_CHAT",
		LaneEval:     "MODEL_EVAL",
	}
	for _, lane := range []string{LaneChat, LaneFastChat, LaneEval} {
		spec := cat.Lanes[lane]
		src := "catalog default"
		if overrides[lane] != "" {
			src = envVar[lane] + " override"
		}
		r, err := cat.Resolve(lane, overrides[lane], keys)
		if err != nil {
			fmt.Fprintf(&b, "  %-9s %-28s tier=%-9s UNRESOLVED (%v)\n", lane, spec.Model, spec.Tier, err)
			continue
		}
		fmt.Fprintf(&b, "  %-9s %-28s tier=%-9s via %s (%s)\n", lane, r.ModelID, r.Tier, r.Provider, src)
	}

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

	fmt.Fprintf(&b, "\nMODELS (bind with MODEL_CHAT / MODEL_FAST_CHAT / MODEL_EVAL)\n")
	for _, id := range cat.ModelIDs() {
		m := cat.Models[id]
		price := "UNPRICED — set priceUsd in models.json to compare cost"
		if m.Price != nil {
			price = fmt.Sprintf("$%.3f in / $%.3f out per 1M", m.Price.InputPerMillion, m.Price.OutputPerMillion)
		}
		flag := " "
		if m.Flagship {
			flag = "*"
		}
		fmt.Fprintf(&b, "  %s %-28s %s\n", flag, id, price)
	}
	fmt.Fprintf(&b, "\n  * = flagship-eligible (the eval lane accepts only these)\n")
	return b.String()
}
