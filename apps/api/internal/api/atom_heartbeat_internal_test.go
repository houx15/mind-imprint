package api

// This file is `package api`, NOT `package api_test`: clampHeartbeatSeconds
// is unexported, and every reading/writing test file in this directory is
// package api_test and cannot see it.

import "testing"

func TestClampHeartbeatSeconds(t *testing.T) {
	cases := []struct{ in, want int32 }{
		{60, 60},
		{120, 120},
		{600, 120}, // a tab that slept, or a client lying — 120 is the ceiling
		{0, 0},
		{-5, 0}, // never subtract from her focus time
	}
	for _, c := range cases {
		if got := clampHeartbeatSeconds(c.in); got != c.want {
			t.Errorf("clampHeartbeatSeconds(%d) = %d, want %d", c.in, got, c.want)
		}
	}
}
