package pbl

import (
	"reflect"
	"testing"
)

func TestPaperEditsPreserveUntouchedGeometry(t *testing.T) {
	p := PaperLayout{Format: "a4-portrait", Title: "原型", Notice: "未验证", Elements: []PaperElement{{Kind: "rect", X: 12, Y: 35, Width: 100, Height: 30}, {Kind: "text", X: 16, Y: 40, Width: 60, Height: 0, FontSize: 4, Text: "尺寸待确认"}}}
	old := p.Elements[1]
	next := old
	next.X = 20
	got, err := p.ApplyEdits([]PaperEdit{{Old: old, New: next}})
	if err != nil {
		t.Fatal(err)
	}
	if got.Elements[0] != p.Elements[0] || p.Elements[1] != old || got.Elements[1] != next || got.Title != p.Title {
		t.Fatal("unrequested changes or source mutation")
	}
	for _, edits := range [][]PaperEdit{{{Old: next, New: old}}, {{Old: old, New: next}, {Old: old, New: next}}, {{Old: old, New: PaperElement{Kind: "rect", X: 199, Y: 40, Width: 10, Height: 10}}}} {
		failed, err := p.ApplyEdits(edits)
		if err == nil || !reflect.DeepEqual(failed, p) {
			t.Fatal("invalid edits changed document")
		}
	}
}
