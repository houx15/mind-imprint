package api

import "context"

// HasEntitlement is the paid-access seam (membership ≠ structural belonging).
// Stubbed open for everyone in P1; future billing reads a subscription or
// token-balance model keyed by school/org without changing any call site —
// mirrors the gateway keyResolver seam. Checked before /turn and /evaluate.
func HasEntitlement(_ context.Context, _ User) (bool, error) {
	return true, nil
}
