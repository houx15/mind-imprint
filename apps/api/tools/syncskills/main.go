// Command syncskills mirrors the canonical skill JSON specs from
// packages/contracts/skills into internal/skills/specs so they can be embedded.
// It is the ONLY writer of the mirror; never hand-edit internal/skills/specs.
//
// Run from the apps/api module root: `go run ./tools/syncskills` (or `make sync-skills`).
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const canonicalRel = "../../packages/contracts/skills"
const mirrorRel = "internal/skills/specs"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "syncskills:", err)
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
		data, err := os.ReadFile(filepath.Join(canonicalRel, e.Name()))
		if err != nil {
			return fmt.Errorf("read %s: %w", e.Name(), err)
		}
		if err := os.WriteFile(filepath.Join(mirrorRel, e.Name()), data, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", e.Name(), err)
		}
		count++
	}
	fmt.Printf("syncskills: mirrored %d skill specs\n", count)
	return nil
}
