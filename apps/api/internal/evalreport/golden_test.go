package evalreport

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestPlaceholder_GoldenMatchesZodContract is the only automated check that
// the REAL Go placeholder JSON — not a hand-written fixture — satisfies the
// frontend's strict Zod `EvaluationReport` schema. `Validate` here only
// guards the outer envelope (see validate.go's doc comment); the deep-shape
// truth lives in packages/contracts, and until this test nothing exercised
// that boundary at all.
//
// It marshals `Placeholder(...)` and writes it to testdata/placeholder_golden.json
// (regenerated on every `go test` run — commit the file so the paired TS
// test, packages/contracts/test/placeholderGolden.test.ts, can read it
// without needing Go). That TS test imports this exact file and asserts
// `EvaluationReport.parse(golden)` does not throw.
func TestPlaceholder_GoldenMatchesZodContract(t *testing.T) {
	r := Placeholder(
		"project:phoebe-china-sustainability",
		"report:phoebe-china-sustainability",
		"student:phoebe",
		"Phoebe",
		"中国是否让地球变得更可持续？",
		"Extended Essay",
		"2026-08-06T16:30:00Z",
	)

	raw, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		t.Fatalf("marshal placeholder: %v", err)
	}

	// Sanity: the envelope Go itself is responsible for still round-trips.
	if _, err := Validate(raw); err != nil {
		t.Fatalf("golden placeholder output failed Validate: %v", err)
	}

	dir := "testdata"
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	path := filepath.Join(dir, "placeholder_golden.json")
	if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
		t.Fatalf("write golden file %s: %v", path, err)
	}
}
