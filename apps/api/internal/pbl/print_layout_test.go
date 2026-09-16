package pbl

import (
	"reflect"
	"strings"
	"testing"
)

func TestFoldoutEditsPreserveOtherPanelsAndRejectAmbiguity(t *testing.T) {
	p := PrintLayout{Format: "a4-accordion-six"}
	for _, title := range []string{"封面", "厨房", "客厅", "卫生间", "待验证清单", "封底"} {
		p.Panels = append(p.Panels, PrintPanel{Title: title, Body: "规则待核对"})
	}
	p.Panels[0].Body = "结构来自学生推测。规则待核对"
	updated, err := p.ApplyEdits([]TextEdit{{Old: "结构来自学生推测", New: "结构由学生选择，尚未验证"}})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(p.Panels[1:], updated.Panels[1:]) || p.Panels[0].Body != "结构来自学生推测。规则待核对" {
		t.Fatal("untargeted or original panels mutated")
	}
	if updated.Panels[0].Body != "结构由学生选择，尚未验证。规则待核对" {
		t.Fatal("replacement missing")
	}
	if _, err := p.ApplyEdits([]TextEdit{{Old: "规则待核对", New: "错误"}}); err == nil {
		t.Fatal("ambiguous edit accepted")
	}
	if _, err := p.ApplyEdits([]TextEdit{{Old: "封面\n\n结构", New: "错误"}}); err == nil {
		t.Fatal("cross-field edit accepted")
	}
	if _, err := p.ApplyEdits([]TextEdit{{Old: "结构来自学生推测", New: strings.Repeat("字", 361)}}); err == nil {
		t.Fatal("oversized revision accepted")
	}
}

func TestPrintLayoutHasOneOrderedContentSource(t *testing.T) {
	p := PrintLayout{Format: "a4-accordion-six"}
	for _, title := range []string{"封面", "厨房", "客厅", "卫生间", "待验证清单", "封底"} {
		p.Panels = append(p.Panels, PrintPanel{Title: title, Body: "分类答案：待核对"})
	}
	if err := p.Validate(); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(p.Markdown(), "## 封面\n\n分类答案：待核对") || strings.Index(p.Markdown(), "## 厨房") > strings.Index(p.Markdown(), "## 客厅") {
		t.Fatal("physical panel order changed")
	}
	p.Panels[2].Body = strings.Repeat("字", 361)
	if p.Validate() == nil {
		t.Fatal("oversized panel accepted")
	}
	p.Panels = p.Panels[:5]
	if p.Validate() == nil {
		t.Fatal("missing panel accepted")
	}
}
