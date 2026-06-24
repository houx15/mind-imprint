package cards

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCatalog(t *testing.T) {
	specs, err := Catalog()
	if err != nil {
		t.Fatalf("Catalog: %v", err)
	}
	if len(specs) != 33 {
		t.Fatalf("catalog has %d specs, want 33", len(specs))
	}

	seen := map[string]bool{}
	for _, s := range specs {
		if s.ID == "" {
			t.Fatal("spec with empty id")
		}
		if seen[s.ID] {
			t.Fatalf("duplicate id %q", s.ID)
		}
		seen[s.ID] = true
	}
}

func TestByID(t *testing.T) {
	got, ok := ByID("ai-boundary")
	if !ok {
		t.Fatal("ai-boundary not found")
	}
	if got.NameEN != "AI Boundary & Hallucination Check" {
		t.Fatalf("name_en = %q", got.NameEN)
	}
	if got.Category != "AI伦理" {
		t.Fatalf("category = %q", got.Category)
	}

	if _, ok := ByID("does-not-exist"); ok {
		t.Fatal("unknown id should not resolve")
	}
}

// TestMirrorMatchesCanonical fails if someone edited a canonical card without
// re-running `make sync-cards`. It compares the embedded mirror byte-for-byte
// against packages/contracts/cards.
func TestMirrorMatchesCanonical(t *testing.T) {
	const canonicalRel = "../../../../packages/contracts/cards"
	canonicalEntries, err := os.ReadDir(canonicalRel)
	if err != nil {
		t.Fatalf("read canonical dir: %v", err)
	}

	canonicalJSON := map[string][]byte{}
	for _, e := range canonicalEntries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(canonicalRel, e.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		canonicalJSON[e.Name()] = b
	}

	mirrorEntries, err := specFS.ReadDir("specs")
	if err != nil {
		t.Fatalf("read embedded specs: %v", err)
	}
	mirrorJSON := map[string][]byte{}
	for _, e := range mirrorEntries {
		b, err := specFS.ReadFile("specs/" + e.Name())
		if err != nil {
			t.Fatalf("read embedded %s: %v", e.Name(), err)
		}
		mirrorJSON[e.Name()] = b
	}

	if len(canonicalJSON) != len(mirrorJSON) {
		t.Fatalf("file count drift: canonical=%d mirror=%d (run `make sync-cards`)",
			len(canonicalJSON), len(mirrorJSON))
	}
	for name, want := range canonicalJSON {
		got, ok := mirrorJSON[name]
		if !ok {
			t.Fatalf("mirror missing %s (run `make sync-cards`)", name)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("mirror drift in %s (run `make sync-cards`)", name)
		}
	}
}
