package gateway

import (
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
)

// PricingVersion identifies the rate table semantics used for estimates. The
// value is recorded by offline experiment artifacts alongside the concrete
// rates so historical reports remain interpretable after a rate-table update.
const PricingVersion = "gateway-pricing-v1"

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
		if m.Price == nil {
			continue
		}
		out[m.Provider+"/"+m.Model] = TokenPriceUSD{
			InputPerMillionUSD:  m.Price.InputPerMillion,
			OutputPerMillionUSD: m.Price.OutputPerMillion,
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

// EstimateCost returns the USD cost estimate for a call, or ok=false when the
// model has no price entry.
func EstimateCost(provider, model string, promptTokens, completionTokens int) (float64, bool) {
	p, ok := LookupTokenPrice(provider, model)
	if !ok {
		return 0, false
	}
	cost := float64(promptTokens)/1e6*p.InputPerMillionUSD + float64(completionTokens)/1e6*p.OutputPerMillionUSD
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
