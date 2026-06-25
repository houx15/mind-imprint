package gateway

import "testing"

func TestEstimateCost(t *testing.T) {
	c, ok := EstimateCost("deepseek", "deepseek-chat", 1_000_000, 1_000_000)
	if !ok || c <= 0 { t.Fatalf("priced model: ok=%v c=%v", ok, c) }
	if _, ok := EstimateCost("deepseek", "unknown-model", 10, 10); ok {
		t.Fatal("unknown model must be unpriced")
	}
	n := CostNumeric(EstimateCost("deepseek", "deepseek-chat", 100, 100))
	if !n.Valid { t.Fatal("priced cost must yield a valid Numeric") }
	if CostNumeric(0, false).Valid { t.Fatal("unpriced cost must be NULL Numeric") }
}
