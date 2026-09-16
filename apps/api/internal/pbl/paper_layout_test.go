package pbl

import (
	"math"
	"strings"
	"testing"
)

func TestPaperLayoutPrintableBounds(t *testing.T) {
	base := PaperLayout{Format: "a4-portrait", Title: "纸面原型", Notice: "未观察、未验证", Elements: []PaperElement{{Kind: "text", X: 12, Y: 35, Width: 60, Height: 6, FontSize: 4, Text: "尺寸待测量"}}}
	if err := base.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		edit func(*PaperElement)
	}{
		{"outside", func(e *PaperElement) { e.X = 190 }},
		{"nonfinite", func(e *PaperElement) { e.Y = math.Inf(1) }},
		{"text-overflow", func(e *PaperElement) { e.Width = 4 }},
		{"multiline", func(e *PaperElement) { e.Text = "待测\n量" }},
		{"active-content", func(e *PaperElement) { e.Kind = "script" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := base
			p.Elements = append([]PaperElement(nil), base.Elements...)
			tc.edit(&p.Elements[0])
			if p.Validate() == nil {
				t.Fatal("accepted invalid printable layout")
			}
		})
	}
	if !strings.Contains(base.Markdown(), "尺寸待测量") {
		t.Fatal("review lost visible text")
	}
}

func TestPaperAllowsRendererManagedTextForeground(t *testing.T) {
	label := PaperElement{Kind: "text", X: 16, Y: 249, Width: 60, Height: 6, FontSize: 3.5, Text: "高度：待确认"}
	shape := PaperElement{Kind: "rect", X: 30, Y: 250, Width: 22, Height: 14, Fill: "white"}
	p := PaperLayout{Format: "a4-portrait", Title: "原型", Notice: "未验证", Elements: []PaperElement{label, shape}}
	if err := p.Validate(); err != nil {
		t.Fatalf("renderer places text above shapes: %v", err)
	}
	p.Elements = []PaperElement{shape, label}
	if err := p.Validate(); err != nil {
		t.Fatalf("background behind text is valid: %v", err)
	}
}

func TestPaperTextHeightUsesFontMetrics(t *testing.T) {
	p := PaperLayout{Format: "a4-portrait", Title: "原型", Notice: "未验证", Elements: []PaperElement{{Kind: "text", X: 12, Y: 40, Width: 60, Height: 0, FontSize: 4, Text: "尺寸待确认"}}}
	if err := p.Validate(); err != nil {
		t.Fatalf("model should not need to calculate line height: %v", err)
	}
	p.Elements[0].Y = 274
	if p.Validate() == nil {
		t.Fatal("font-derived height must still fit page")
	}
}
