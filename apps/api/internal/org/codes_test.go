package org_test

import (
	"strings"
	"testing"

	"mindimprint/api/internal/org"
)

const ambiguous = "01OI"

func TestNewClassJoinCode(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		c, err := org.NewClassJoinCode()
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if len(c) != 9 || c[4] != '-' { // XXXX-XXXX
			t.Fatalf("bad format: %q", c)
		}
		if strings.ContainsAny(c, ambiguous) {
			t.Fatalf("ambiguous char in %q", c)
		}
		if seen[c] {
			t.Fatalf("collision: %q", c)
		}
		seen[c] = true
	}
}

func TestNewTeacherInviteCode(t *testing.T) {
	c, err := org.NewTeacherInviteCode()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.HasPrefix(c, "T-") {
		t.Fatalf("missing T- prefix: %q", c)
	}
	if strings.ContainsAny(strings.TrimPrefix(c, "T-"), ambiguous) {
		t.Fatalf("ambiguous char in %q", c)
	}
}
