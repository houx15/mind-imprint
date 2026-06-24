// Command synccards mirrors the canonical card JSON specs from
// packages/contracts/cards into internal/cards/specs so they can be embedded.
// It is the ONLY writer of the mirror; never hand-edit internal/cards/specs.
//
// Run from the apps/api module root: `go run ./tools/synccards` (or `make sync-cards`).
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// canonicalRel is the canonical card dir relative to apps/api/.
const canonicalRel = "../../packages/contracts/cards"

// mirrorRel is the embeddable mirror relative to apps/api/.
const mirrorRel = "internal/cards/specs"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "synccards:", err)
		os.Exit(1)
	}
}

func run() error {
	entries, err := os.ReadDir(canonicalRel)
	if err != nil {
		return fmt.Errorf("read canonical dir %s: %w", canonicalRel, err)
	}

	if err := os.MkdirAll(mirrorRel, 0o755); err != nil {
		return fmt.Errorf("mkdir mirror: %w", err)
	}

	// Remove stale mirror files so deletions in canonical propagate.
	mirrorEntries, err := os.ReadDir(mirrorRel)
	if err != nil {
		return fmt.Errorf("read mirror dir: %w", err)
	}
	for _, e := range mirrorEntries {
		if strings.HasSuffix(e.Name(), ".json") {
			if err := os.Remove(filepath.Join(mirrorRel, e.Name())); err != nil {
				return fmt.Errorf("remove stale %s: %w", e.Name(), err)
			}
		}
	}

	count := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		src := filepath.Join(canonicalRel, e.Name())
		dst := filepath.Join(mirrorRel, e.Name())
		data, err := os.ReadFile(src)
		if err != nil {
			return fmt.Errorf("read %s: %w", src, err)
		}
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", dst, err)
		}
		count++
	}
	fmt.Printf("synccards: mirrored %d card specs\n", count)
	return nil
}
