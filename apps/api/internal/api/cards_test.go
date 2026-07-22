package api

// cards_test.go — Task 6 (Slice N3c): validateEventTrace unit tests.
// traceKinds (cards.go) is a closed set that mirrors the Zod TraceEvent
// union in packages/contracts/src/envelope.ts. Both must move together —
// a one-sided change 400s every L2/L3 submit. These tests prove both
// directions: the two new kinds are accepted, an unknown kind is still
// rejected, and a pre-existing kind still validates unchanged (additive
// guarantee).

import "testing"

func TestValidateEventTrace_AcceptsSpanLocatedAndSpanNotFound(t *testing.T) {
	raw := []byte(`[
		{"kind":"span_located","dimension":"D1_来源意识","block_id":"b_3","at":"2026-07-22T00:00:00Z"},
		{"kind":"span_not_found","dimension":"D1_来源意识","at":"2026-07-22T00:00:00Z"}
	]`)
	if err := validateEventTrace(raw); err != nil {
		t.Fatalf("expected span_located/span_not_found to validate, got error: %v", err)
	}
}

func TestValidateEventTrace_RejectsUnknownKind(t *testing.T) {
	raw := []byte(`[{"kind":"wiggle","at":"2026-07-22T00:00:00Z"}]`)
	if err := validateEventTrace(raw); err == nil {
		t.Fatal("expected an unknown trace kind to be rejected, got nil error")
	}
}

func TestValidateEventTrace_StillAcceptsPreExistingKinds(t *testing.T) {
	raw := []byte(`[
		{"kind":"field_change","path":"sift.stop","at":"2026-07-22T00:00:00Z"},
		{"kind":"step_expand","step_key":"craap","at":"2026-07-22T00:00:00Z"},
		{"kind":"note_open","step_key":"sift","at":"2026-07-22T00:00:00Z"},
		{"kind":"skip","at":"2026-07-22T00:00:00Z"},
		{"kind":"submit","at":"2026-07-22T00:00:00Z"}
	]`)
	if err := validateEventTrace(raw); err != nil {
		t.Fatalf("expected pre-existing kinds to still validate unchanged, got error: %v", err)
	}
}
