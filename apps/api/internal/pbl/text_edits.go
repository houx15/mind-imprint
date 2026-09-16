package pbl

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

type TextEdit struct {
	Old string `json:"old"`
	New string `json:"new"`
}

// ApplyTextEdits matches every edit against the original document. Insertions
// cannot become targets for another edit, and ambiguous/overlapping edits fail.
func ApplyTextEdits(body string, edits []TextEdit) (string, error) {
	if len(edits) == 0 || len(edits) > 16 {
		return "", errors.New("局部修改需要1至16处替换")
	}
	type change struct {
		start, end  int
		replacement string
	}
	changes := make([]change, 0, len(edits))
	for index, e := range edits {
		if e.Old == "" {
			return "", fmt.Errorf("第%d处修改缺少原文", index+1)
		}
		if e.Old == e.New {
			return "", fmt.Errorf("第%d处修改与原文相同，请删除该处无变化的替换", index+1)
		}
		if count := strings.Count(body, e.Old); count != 1 {
			return "", fmt.Errorf("第%d处修改原文在原始正文中匹配%d次，需要唯一匹配；请保留Markdown标记和换行，从原始正文复制，不使用其他修改产生的新文字", index+1, count)
		}
		start := strings.Index(body, e.Old)
		changes = append(changes, change{start, start + len(e.Old), e.New})
	}
	sort.Slice(changes, func(i, j int) bool { return changes[i].start < changes[j].start })
	for i := 1; i < len(changes); i++ {
		if changes[i].start < changes[i-1].end {
			return "", errors.New("局部修改范围不能重叠")
		}
	}
	out := body
	for i := len(changes) - 1; i >= 0; i-- {
		c := changes[i]
		out = out[:c.start] + c.replacement + out[c.end:]
	}
	if strings.TrimSpace(out) == "" {
		return "", errors.New("修改后成果正文不能为空")
	}
	return out, nil
}
