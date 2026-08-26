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

// priceTable maps "provider/model" → per-1M-token USD prices (input, output).
// Unpriced models yield ok=false so callers persist a NULL cost rather than a
// wrong number. Update when adding a model.
var priceTable = map[string]TokenPriceUSD{
	// Both tiers name deepseek-v4-pro. The v4-flash row is kept (priced) so any
	// call that ever resolves flash still meters correctly — flash was measured
	// for the chaperone but reverted (see NewKeyResolver). Cache-miss prices per
	// DeepSeek's published USD table (the conservative choice — cannot
	// under-report spend).
	"deepseek/deepseek-v4-flash":         {InputPerMillionUSD: 0.14, OutputPerMillionUSD: 0.28},
	"deepseek/deepseek-v4-pro":           {InputPerMillionUSD: 0.435, OutputPerMillionUSD: 0.87},
	"anthropic/claude-3-5-sonnet-latest": {InputPerMillionUSD: 3.00, OutputPerMillionUSD: 15.00},
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
