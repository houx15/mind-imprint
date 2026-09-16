package pbl

import (
	"sort"
	"testing"
)

func TestToolCatalogIncludesEveryRegisteredTool(t *testing.T) {
	names := ToolNames()
	if len(names) != len(registry) || !sort.StringsAreSorted(names) {
		t.Fatal("catalog is incomplete or unstable")
	}
	seen := map[string]bool{}
	for _, name := range names {
		if seen[name] {
			t.Fatal("duplicate", name)
		}
		seen[name] = true
	}
	for name := range registry {
		if !seen[name] {
			t.Fatal("missing registered tool", name)
		}
	}
}
