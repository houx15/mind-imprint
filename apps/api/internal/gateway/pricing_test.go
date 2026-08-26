package gateway

import "testing"

func TestEstimateCost(t *testing.T) {
	rate, ok := LookupTokenPrice("deepseek", "deepseek-v4-pro")
	if !ok || rate.InputPerMillionUSD != 0.435 || rate.OutputPerMillionUSD != 0.87 {
		t.Fatalf("unexpected deepseek-v4-pro price: %#v ok=%v", rate, ok)
	}
	c, ok := EstimateCost("deepseek", "deepseek-v4-pro", 1_000_000, 1_000_000)
	if !ok || c != rate.InputPerMillionUSD+rate.OutputPerMillionUSD {
		t.Fatalf("priced model: ok=%v c=%v", ok, c)
	}
	if _, ok := LookupTokenPrice("deepseek", "unknown-model"); ok {
		t.Fatal("unknown model must be unpriced")
	}
	if _, ok := EstimateCost("deepseek", "unknown-model", 10, 10); ok {
		t.Fatal("unknown model must be unpriced")
	}
	n := CostNumeric(EstimateCost("deepseek", "deepseek-v4-pro", 100, 100))
	if !n.Valid {
		t.Fatal("priced cost must yield a valid Numeric")
	}
	if CostNumeric(0, false).Valid {
		t.Fatal("unpriced cost must be NULL Numeric")
	}
}

func TestCostNumericSemanticsSplit(t *testing.T) {
	// llm_call path (NOT NULL column): an unpriced model records explicit $0.00.
	z := CostNumeric(0, true)
	if !z.Valid {
		t.Fatal("llm_call path: explicit $0.00 must be a valid zero Numeric, not NULL")
	}
	// evaluation path (nullable column): an unpriced model records NULL.
	if CostNumeric(0, false).Valid {
		t.Fatal("evaluation path: unpriced cost must be NULL (invalid) Numeric")
	}
}
