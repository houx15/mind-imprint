package evalbench

import (
	"fmt"
	"strings"
)

// ValidateGoldMarkdown checks the small, stable document contract expected by
// the comparator. Gold is user-provided input: evalbench does not generate,
// transform, or enrich it.
func ValidateGoldMarkdown(source string) error {
	for _, heading := range requiredGoldHeadings {
		if !strings.Contains(source, heading) {
			return fmt.Errorf("evalbench: gold Markdown is missing required heading %q", heading)
		}
	}
	return nil
}

var requiredGoldHeadings = []string{
	"## D1", "## D2", "## D3", "## D4", "## D5", "## D6",
	"## A1", "## A2", "## A3", "## A4", "## A5", "## A6",
	"## 综述", "## 提问透镜", "## 风险", "## 下一步", "## 未归类内容",
}
