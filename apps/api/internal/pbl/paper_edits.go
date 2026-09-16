package pbl

import "fmt"

// PaperEdit matches an immutable source element exactly; omitted elements stay
// byte-for-byte equivalent after decoding. All edits apply before validation.
type PaperEdit struct {
	Old PaperElement `json:"old"`
	New PaperElement `json:"new"`
}

func (p PaperLayout) ApplyEdits(edits []PaperEdit) (PaperLayout, error) {
	if len(edits) == 0 || len(edits) > 16 {
		return p, fmt.Errorf("图形局部修改须有1至16处")
	}
	result := p
	result.Elements = append([]PaperElement(nil), p.Elements...)
	used := map[int]bool{}
	for _, edit := range edits {
		match := -1
		for i, element := range p.Elements {
			if element == edit.Old {
				if match >= 0 {
					return p, fmt.Errorf("待修改图形不唯一，请提供完整布局修订")
				}
				match = i
			}
		}
		if match < 0 {
			return p, fmt.Errorf("未找到完全匹配的原图形，请从当前paperLayout复制完整old元素")
		}
		if used[match] {
			return p, fmt.Errorf("同一图形不能重复修改")
		}
		if edit.Old == edit.New {
			return p, fmt.Errorf("图形修改前后相同")
		}
		used[match] = true
		result.Elements[match] = edit.New
	}
	if err := result.Validate(); err != nil {
		return p, err
	}
	return result, nil
}
