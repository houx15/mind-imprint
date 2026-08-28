package api

// atom_report_share_internal_test.go — newShareToken is unexported, so this
// test lives in package api (see atom_report_share_test.go for the
// HTTP-level guarantees, which run as package api_test like every other
// test file in this directory).

import "testing"

// TestShareTokenIsUnguessable — R1's first compensating control: 32 hex
// chars (16 random bytes), never repeating across 200 draws. This does not
// prove cryptographic randomness (no test can), but it does pin the SHAPE
// (length) that guessability depends on and would catch a regression to a
// shorter or non-hex encoding.
func TestShareTokenIsUnguessable(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		tok, err := newShareToken()
		if err != nil {
			t.Fatal(err)
		}
		if len(tok) != 32 {
			t.Fatalf("token %q is %d chars, want 32", tok, len(tok))
		}
		if seen[tok] {
			t.Fatal("newShareToken repeated a token")
		}
		seen[tok] = true
	}
}
