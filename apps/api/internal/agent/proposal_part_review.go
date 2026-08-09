package agent

import (
	"context"
	"fmt"
	"strings"

	"mindimprint/api/internal/gateway"
)

// proposal_part_review.go — slice 3a · the flagship reasoning reviewer for ONE
// proposal part (the "我写好了" interim). It reads the student's text for a single
// guided step against that step's guiding question and returns a strong-advisory
// FrameworkVerdict ({ready, why, suggestions}). It NEVER rewrites the text
// (铁律①) — suggestions are directions. Mirrors ReviewFramework's shape; 3b will
// upgrade the surfaced feedback to anchored, colored 批注.

// ProposalPartReviewInput carries the one part under review.
type ProposalPartReviewInput struct {
	Title       string
	StepTitle   string
	StepPrompt  string
	StudentText string
}

const proposalPartReviewSystem = `你是一位严谨而鼓励的 IB 导师。学生刚写完研究提案的某一个部分。请只针对这一部分、对照它的引导问题，判断它是否已经足够扎实、可以推进到下一部分。

只返回一个 JSON 对象：
{"ready": true, "why": "一句话总体判断", "suggestions": ["具体建议一", "具体建议二"]}

要求：
- ready 表示「足以推进到下一部分」，不是「完美」；即使 ready 也可给改进建议。
- suggestions 具体、可操作，指向最弱的一到两处；最多 4 条；确实没有可改进处给空数组。
- 绝不替学生改写正文——只给方向（铁律①）。
- 用中文；只回 JSON，不要任何解释或代码块外的文字。`

const maxProposalPartReviewAttempts = 2

// ReviewProposalPart runs the flagship reasoning reviewer over one part. Same
// best-effort contract as ReviewFramework: any error → the caller degrades to no
// verdict. Usage is returned so the caller meters even a failed call.
func ReviewProposalPart(ctx context.Context, prov gateway.Provider, resolved gateway.Resolved, in ProposalPartReviewInput) (FrameworkVerdict, gateway.ChatUsage, error) {
	var b strings.Builder
	if s := strings.TrimSpace(in.Title); s != "" {
		fmt.Fprintf(&b, "题目：%s\n", s)
	}
	if s := strings.TrimSpace(in.StepTitle); s != "" {
		fmt.Fprintf(&b, "部分：%s\n", s)
	}
	if s := strings.TrimSpace(in.StepPrompt); s != "" {
		fmt.Fprintf(&b, "这一部分的引导问题：%s\n", s)
	}
	fmt.Fprintf(&b, "学生写的内容：\n%s\n", strings.TrimSpace(in.StudentText))

	req := gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: proposalPartReviewSystem},
			{Role: gateway.RoleUser, Content: b.String()},
		},
		MaxTokens: 3000, // reasoning-model headroom
	}

	var lastUsage gateway.ChatUsage
	var lastErr error
	for attempt := 0; attempt < maxProposalPartReviewAttempts; attempt++ {
		res, err := gateway.Collect(ctx, prov, resolved, req)
		lastUsage = res.Usage
		if err != nil {
			lastErr = err
			continue
		}
		v, perr := parseFrameworkVerdict(res.Text)
		if perr != nil {
			lastErr = perr
			continue
		}
		return v, lastUsage, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("proposal part review: no parseable verdict")
	}
	return FrameworkVerdict{}, lastUsage, lastErr
}
