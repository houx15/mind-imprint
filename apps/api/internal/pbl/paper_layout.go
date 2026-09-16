package pbl

import (
	"fmt"
	"math"
	"strings"
	"unicode/utf8"
)

// PaperLayout contains inert vector primitives, never model-authored HTML/SVG.
// Coordinates are print millimetres, not measurements of the depicted object.
type PaperLayout struct {
	Format   string         `json:"format"`
	Title    string         `json:"title"`
	Notice   string         `json:"notice"`
	Elements []PaperElement `json:"elements"`
}
type PaperElement struct {
	Kind     string  `json:"kind"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	Width    float64 `json:"width"`
	Height   float64 `json:"height"`
	Text     string  `json:"text,omitempty"`
	FontSize float64 `json:"fontSize,omitempty"`
	Fill     string  `json:"fill,omitempty"`
	Dashed   bool    `json:"dashed,omitempty"`
}

func (p PaperLayout) Validate() error {
	if p.Format != "a4-portrait" || strings.TrimSpace(p.Title) == "" || utf8.RuneCountInString(p.Title) > 28 || strings.TrimSpace(p.Notice) == "" || utf8.RuneCountInString(p.Notice) > 60 {
		return fmt.Errorf("纸面成果需要A4竖向格式、28字内标题和60字内说明")
	}
	if strings.ContainsAny(p.Title+p.Notice, "\r\n") {
		return fmt.Errorf("纸面标题和说明须为单行")
	}
	if len(p.Elements) == 0 || len(p.Elements) > 120 {
		return fmt.Errorf("纸面成果须有1至120个图形或文字元素")
	}
	for i, e := range p.Elements {
		for _, v := range []float64{e.X, e.Y, e.Width, e.Height, e.FontSize} {
			if math.IsNaN(v) || math.IsInf(v, 0) {
				return fmt.Errorf("纸面元素%d坐标无效", i+1)
			}
		}
		height := e.Height
		if e.Kind == "text" {
			height = math.Max(height, e.FontSize*1.3)
		}
		if e.X < 12 || e.Y < 35 || e.Width < 0 || e.Height < 0 || e.X+e.Width > 198 || e.Y+height > 277 {
			return fmt.Errorf("纸面元素%d超出可打印区域：x12至198，y35至277", i+1)
		}
		if e.Fill != "" && e.Fill != "none" && e.Fill != "white" && e.Fill != "light" && e.Fill != "dark" {
			return fmt.Errorf("纸面元素%d填充色无效", i+1)
		}
		switch e.Kind {
		case "rect", "ellipse":
			if e.Width <= 0 || e.Height <= 0 {
				return fmt.Errorf("纸面图形%d需要正的宽高", i+1)
			}
		case "line":
			if e.Width == 0 && e.Height == 0 {
				return fmt.Errorf("纸面线段%d长度为零", i+1)
			}
		case "text":
			if e.FontSize < 3 || e.FontSize > 8 {
				return fmt.Errorf("纸面文字%d字号为%gmm，须在3至8mm之间", i+1, e.FontSize)
			}
			if strings.TrimSpace(e.Text) == "" || strings.ContainsAny(e.Text, "\r\n") {
				return fmt.Errorf("纸面文字%d须为非空单行，换行请拆成多个text元素", i+1)
			}
			width := 0.0
			for _, r := range e.Text {
				if r < 128 {
					width += e.FontSize * 0.65
				} else {
					width += e.FontSize
				}
			}
			if width > e.Width {
				return fmt.Errorf("纸面文字%d过长，请缩短或拆成多行元素", i+1)
			}
		default:
			return fmt.Errorf("纸面元素%d类型不支持，只允许rect/ellipse/line/text", i+1)
		}
	}
	return nil
}

func (p PaperLayout) Markdown() string {
	lines := []string{"# " + p.Title, p.Notice, "图形版请查看纸面预览。以下为页面文字记录："}
	for _, e := range p.Elements {
		if e.Kind == "text" {
			lines = append(lines, e.Text)
		}
	}
	return strings.Join(lines, "\n\n")
}
