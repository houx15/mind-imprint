package routebench_test

import (
	"os/exec"
	"strings"
	"testing"
)

// The workbench must stay OUT of the running system. That is not a style
// preference — it is the property that lets the bench call real models, keep
// throwaway fixtures, and be rewritten freely next year without anyone having
// to ask whether a student request could reach it.
//
// A stray import is the easy way to lose it: someone reuses one helper from
// here inside a handler, and now a tool that makes paid model calls in a loop
// is linked into the server. This test is cheap and catches exactly that.
func TestServerDoesNotImportTheWorkbench(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", "mindimprint/api/cmd/api").Output()
	if err != nil {
		t.Skipf("go list unavailable: %v", err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.TrimSpace(line) == "mindimprint/api/internal/routebench" {
			t.Fatal("cmd/api now depends on internal/routebench — the workbench must stay out of the serving path")
		}
	}
}

// The bench CASES, by contrast, do live in the production packages, because
// they must be built from the real (unexported) prompts. That is deliberate and
// safe only while they stay pure: no database, no network, no globals. If a
// bench case ever needs a *sqlc.Queries, it has stopped being a fixture and
// started being a second implementation of the handler.
func TestBenchCasePackageStaysALeaf(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", "mindimprint/api/internal/benchcase").Output()
	if err != nil {
		t.Skipf("go list unavailable: %v", err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		switch strings.TrimSpace(line) {
		case "mindimprint/api/internal/store/sqlc", "mindimprint/api/internal/api":
			t.Fatalf("internal/benchcase pulled in %s — it must stay a leaf both packages can import", line)
		}
	}
}
