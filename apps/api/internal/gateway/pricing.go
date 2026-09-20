package gateway

import (
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
)

// PricingVersion identifies the rate table semantics used for estimates. The
// value is recorded by offline experiment artifacts alongside the concrete
// rates so historical reports remain interpretable after a rate-table update.
const PricingVersion = "gateway-pricing-v2"

// CNYPerUSD converts the catalog's yuan rates into the USD that cost_estimate
// stores. DashScope publishes CNY only, so the choice is between one exchange
// rate named here and an invented USD number on every DashScope row — and a
// wrong price per model is the kind of error that survives, because nothing
// downstream can tell it from a real one.
//
// 2026-09-20 · 7.1 CNY/USD. Moving it re-prices only future calls: rows already
// written keep the rate they were costed at, which is what makes a month-over-
// month comparison mean anything.
const CNYPerUSD = 7.1

// TokenPriceUSD contains USD cache-miss rates per one million input and output
// tokens. Output includes any provider-reported reasoning tokens.
type TokenPriceUSD struct {
	InputPerMillionUSD  float64
	OutputPerMillionUSD float64
}

// priceTable maps "provider/model" (the wire pair recorded on every llm_call
// row) → per-1M-token USD prices. It is DERIVED from the embedded catalog so
// rates cannot drift away from the models the resolvers actually serve: pricing
// a model is an edit to its models.json entry, in the same place its route and
// reasoning policy live.
//
// A model with no priceUsd is absent here and yields ok=false, so callers
// persist NULL/zero rather than a wrong number. Cache-miss rates are the
// conservative choice — they cannot under-report spend.
var priceTable = buildPriceTable()

func buildPriceTable() map[string]TokenPriceUSD {
	out := map[string]TokenPriceUSD{}
	cat, err := DefaultCatalog()
	if err != nil {
		// A broken catalog is reported at boot by DefaultCatalog; here it simply
		// means every model is unpriced, which records no cost rather than a wrong one.
		return out
	}
	for _, id := range cat.ModelIDs() {
		m := cat.Models[id]
		switch {
		case m.Price != nil:
			out[m.Provider+"/"+m.Model] = TokenPriceUSD{
				InputPerMillionUSD:  m.Price.InputPerMillion,
				OutputPerMillionUSD: m.Price.OutputPerMillion,
			}
		case m.PriceCNY != nil:
			out[m.Provider+"/"+m.Model] = TokenPriceUSD{
				InputPerMillionUSD:  m.PriceCNY.InputPerMillion / CNYPerUSD,
				OutputPerMillionUSD: m.PriceCNY.OutputPerMillion / CNYPerUSD,
			}
		}
	}
	return out
}

// LookupTokenPrice returns the shared USD rate for a provider/model pair. It
// is read-only so offline consumers can snapshot the exact price table used by
// a run without maintaining a second price list.
func LookupTokenPrice(provider, model string) (TokenPriceUSD, bool) {
	p, ok := priceTable[provider+"/"+model]
	return p, ok
}

// CachedInputRate is what a provider-cached input token costs as a fraction of
// the normal input rate. DashScope's implicit cache — automatic, and not
// something a caller can switch off — bills a matched prefix at 20%.
//
// Measured 2026-09-20 against the reading coach's own prompt: turn N+1 of a
// reading session reported 7,168-8,320 of ~8,300 prompt tokens as cached, i.e.
// 91-99%. That is the whole article plus the system prompt arriving at a fifth
// of list price, and it is why "the reading room sends a lot of tokens" and
// "the reading room costs a lot of money" are different statements.
const CachedInputRate = 0.2

// EstimateCost returns the USD cost estimate for a call, or ok=false when the
// model has no price entry. It assumes none of the input was cached; callers
// with a usage breakdown should prefer EstimateCostCached.
func EstimateCost(provider, model string, promptTokens, completionTokens int) (float64, bool) {
	return EstimateCostCached(provider, model, promptTokens, 0, completionTokens)
}

// EstimateCostCached prices a call whose provider reported part of the input as
// a cache hit. cachedTokens is a SUBSET of promptTokens; anything above it is
// clamped, so a provider that reports the two inconsistently cannot produce a
// negative fresh-token count and an under-estimate.
func EstimateCostCached(provider, model string, promptTokens, cachedTokens, completionTokens int) (float64, bool) {
	p, ok := LookupTokenPrice(provider, model)
	if !ok {
		return 0, false
	}
	if cachedTokens < 0 {
		cachedTokens = 0
	}
	if cachedTokens > promptTokens {
		cachedTokens = promptTokens
	}
	fresh := promptTokens - cachedTokens
	cost := float64(fresh)/1e6*p.InputPerMillionUSD +
		float64(cachedTokens)/1e6*p.InputPerMillionUSD*CachedInputRate +
		float64(completionTokens)/1e6*p.OutputPerMillionUSD
	return cost, true
}

// CostNumeric converts an estimate to a pgtype.Numeric (6 dp). ok=false → an
// invalid (NULL) Numeric.
func CostNumeric(cost float64, ok bool) pgtype.Numeric {
	var n pgtype.Numeric
	if !ok {
		return n // Valid == false → SQL NULL
	}
	_ = n.Scan(fmt.Sprintf("%.6f", cost))
	return n
}
