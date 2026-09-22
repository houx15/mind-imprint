package promptassembly

import (
	"fmt"
	"testing"
)

func TestSectionsPreserveBytesAndSnapshotIsolation(t *testing.T) {
	var b Builder
	b.Mark("empty", "context", "context.go")
	b.Mark("rule", "instruction", "rule.go")
	b.WriteString("规则\n")
	b.Mark("facts", "context", "context.go")
	fmt.Fprintf(&b, "事实：%s\n", "🦉")
	d := b.Document(Selection{ID: "history", Total: 20, Included: 14, Unit: "messages"})
	if d.Text != "规则\n事实：🦉\n" || len(d.Sections) != 2 {
		t.Fatalf("changed bytes/empty sections: %#v", d)
	}
	end := 0
	for _, s := range d.Sections {
		if s.Start != end || s.End <= s.Start {
			t.Fatalf("noncontiguous section: %#v", s)
		}
		end = s.End
	}
	if end != len(d.Text) {
		t.Fatal("unattributed bytes")
	}
	d.Sections[0].ID = "changed"
	d.Selections[0].Total = 0
	b.Mark("next", "instruction", "next.go")
	b.WriteString("next")
	next := b.Document()
	if next.Sections[0].ID != "rule" || len(next.Sections) != 3 || d.Text != "规则\n事实：🦉\n" {
		t.Fatal("snapshots share mutable metadata")
	}
}
