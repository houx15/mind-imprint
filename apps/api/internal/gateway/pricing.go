package gateway

import (
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
)

// priceTable maps "provider/model" → per-1M-token USD prices (input, output).
// Unpriced models yield ok=false so callers persist a NULL cost rather than a
// wrong number. Update when adding a model.
var priceTable = map[string]struct{ in, out float64 }{
	// Both tiers (chaperone + flagship) name deepseek-v4-pro; cache-miss prices
	// (the conservative choice — cannot under-report spend).
	"deepseek/deepseek-v4-pro":           {0.435, 0.87},
	"anthropic/claude-3-5-sonnet-latest": {3.00, 15.00},
}

// EstimateCost returns the USD cost estimate for a call, or ok=false when the
// model has no price entry.
func EstimateCost(provider, model string, promptTokens, completionTokens int) (float64, bool) {
	p, ok := priceTable[provider+"/"+model]
	if !ok {
		return 0, false
	}
	cost := float64(promptTokens)/1e6*p.in + float64(completionTokens)/1e6*p.out
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
