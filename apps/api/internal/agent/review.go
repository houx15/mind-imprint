package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"mindimprint/api/internal/agent/enforcement"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/skills"
)

// ReviewItem is one work-order row: which criterion, the band the draft sits
// in, the evidence it submits, what is missing, and an OPTIONAL fix — advice
// only, never a rewritten sentence (RL-1).
type ReviewItem struct {
	CriterionCode string `json:"criterion_code"`
	CriterionName string `json:"criterion_name"`
	Band          string `json:"band"`
	Evidence      string `json:"evidence"`
	Missing       string `json:"missing"`
	Fix           string `json:"fix"`
}

// reviewItemWire is the model's per-item JSON contract (name is resolved
// server-side from the criterion code — the model never invents labels).
type reviewItemWire struct {
	CriterionCode string `json:"criterion_code"`
	Band          string `json:"band"`
	Evidence      string `json:"evidence"`
	Missing       string `json:"missing"`
	Fix           string `json:"fix"`
}

const reviewPosturePrompt = `你是 IB/国际课程写作的「整稿体检」考官。学生已提交一版草稿快照。
只做一件事：对照给定的评分表，指出每一张表现在收到了哪些证据、还缺什么。
铁律：绝不替学生改写句子、绝不给示范句、绝不续写。你的「建议」只能是"要补什么/要接什么"的方向，
不能是可直接粘贴的成品句子。一次只输出 JSON 数组，每个评分表一个对象。`

// ProposeReview asks the flagship model for a whole-draft work-order over the
// snapshot's paragraphs, then runs the full enforcement stack on every field
// before returning. A single banned-phrasing / output-check violation rejects
// the WHOLE review (nothing is returned or persisted) — the same all-or-nothing
// discipline as the coach. Usage is populated whenever Collect succeeded (even
// on a later rejection) so the caller can still record 档位+token+成本.
func ProposeReview(ctx context.Context, prov gateway.Provider, r gateway.Resolved, criteria []skills.ReviewCriterion, paragraphs []string, graphSummary string) ([]ReviewItem, gateway.ChatUsage, error) {
	name := map[string]string{}
	codes := make([]string, 0, len(criteria))
	for _, c := range criteria {
		name[c.Code] = c.Name
		codes = append(codes, c.Code+"（"+c.Name+"）")
	}
	user := fmt.Sprintf("评分表：%s\n\n论证摘要：%s\n\n草稿（分段）：\n%s",
		strings.Join(codes, "、"), graphSummary, strings.Join(paragraphs, "\n\n"))

	res, err := gateway.Collect(ctx, prov, r, gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: reviewPosturePrompt},
			{Role: gateway.RoleUser, Content: user},
		},
	})
	if err != nil {
		return nil, gateway.ChatUsage{}, err
	}
	usage := res.Usage

	var wires []reviewItemWire
	if err := json.Unmarshal([]byte(strings.TrimSpace(res.Text)), &wires); err != nil {
		return nil, usage, fmt.Errorf("agent: review output not a JSON array: %w", err)
	}
	items := make([]ReviewItem, 0, len(wires))
	for _, wv := range wires {
		nm, known := name[wv.CriterionCode]
		if !known {
			continue // ignore criteria the skill didn't ask for
		}
		// Enforcement on every free-text field the model produced.
		for _, field := range []string{wv.Evidence, wv.Missing, wv.Fix} {
			if field == "" {
				continue
			}
			if rule := enforcement.BannedPhrasing(field); rule != nil {
				return nil, usage, fmt.Errorf("agent: review output rejected by banned-phrasing rule %q", rule.Name)
			}
		}
		items = append(items, ReviewItem{
			CriterionCode: wv.CriterionCode, CriterionName: nm,
			Band: wv.Band, Evidence: wv.Evidence, Missing: wv.Missing, Fix: wv.Fix,
		})
	}
	if len(items) == 0 {
		return nil, usage, fmt.Errorf("agent: review produced no usable items")
	}
	return items, usage, nil
}
