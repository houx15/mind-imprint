package pbl

import (
	"errors"
	"strings"
	"unicode/utf8"
)

// PrintLayout is a single-sided A4 landscape accordion strip. Panels are in
// physical left-to-right order; the reverse is blank. Content and review text
// are derived from the same panels so a revision cannot silently diverge.
type PrintLayout struct {
	Format string       `json:"format"`
	Panels []PrintPanel `json:"panels"`
}

type PrintPanel struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

func (p PrintLayout) Validate() error {
	if p.Format != "a4-accordion-six" || len(p.Panels) != 6 {
		return errors.New("折页需要六个面，采用A4横向单面手风琴折")
	}
	for _, panel := range p.Panels {
		if strings.TrimSpace(panel.Title) == "" || strings.TrimSpace(panel.Body) == "" {
			return errors.New("折页每一面都需要标题和内容")
		}
		// A bounded content budget is not proof of fit: the browser must also
		// show overflow before students print. Never silently clip text.
		if utf8.RuneCountInString(panel.Title) > 24 || utf8.RuneCountInString(panel.Body) > 360 {
			return errors.New("折页面内容过长，请缩短后重新生成")
		}
	}
	return nil
}

func (p PrintLayout) Markdown() string {
	parts := make([]string, 0, len(p.Panels))
	for _, panel := range p.Panels {
		parts = append(parts, "## "+strings.TrimSpace(panel.Title)+"\n\n"+strings.TrimSpace(panel.Body))
	}
	return strings.Join(parts, "\n\n")
}

// ApplyEdits matches each replacement inside exactly one original panel field.
// It preserves every untargeted panel and never matches newly inserted text.
func (p PrintLayout) ApplyEdits(edits []TextEdit) (PrintLayout, error) {
	if err := p.Validate(); err != nil {
		return PrintLayout{}, err
	}
	if len(edits) == 0 || len(edits) > 16 {
		return PrintLayout{}, errors.New("局部修改需要1至16处替换")
	}
	fields := make([]string, 0, 12)
	for _, panel := range p.Panels {
		fields = append(fields, panel.Title, panel.Body)
	}
	groups := make(map[int][]TextEdit)
	for _, edit := range edits {
		if edit.Old == "" {
			return PrintLayout{}, errors.New("折页修改需要准确原文")
		}
		count, target := 0, -1
		for i, field := range fields {
			if n := strings.Count(field, edit.Old); n > 0 {
				count += n
				target = i
			}
		}
		if count != 1 {
			return PrintLayout{}, errors.New("修改原文须在一个折页面内唯一匹配，请包含相邻文字")
		}
		groups[target] = append(groups[target], edit)
	}
	out := PrintLayout{Format: p.Format, Panels: append([]PrintPanel(nil), p.Panels...)}
	for target, changes := range groups {
		updated, err := ApplyTextEdits(fields[target], changes)
		if err != nil {
			return PrintLayout{}, err
		}
		if target%2 == 0 {
			out.Panels[target/2].Title = updated
		} else {
			out.Panels[target/2].Body = updated
		}
	}
	return out, out.Validate()
}
