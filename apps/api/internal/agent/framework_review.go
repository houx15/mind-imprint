package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"mindimprint/api/internal/gateway"
)

// framework_review.go — slice 2 · the reasoning-model reviewer at the
// framework-readiness gate. When the立项 framework's dimensions are filled, the
// flagship reasoning model reads the WHOLE framework and returns a strong-
// advisory verdict (all-statuses.md §2: "read the whole framework, give some
// suggestions"). It never blocks — the plan generates regardless; the verdict is
// surfaced to the student so 印记 can point at the weakest spots.

// FrameworkVerdict is the reviewer's read. Ready means "solid enough to build a
// plan on" (not "perfect"); Suggestions are concrete, pointed at the 1-2 weakest
// dimensions.
type FrameworkVerdict struct {
	Ready       bool     `json:"ready"`
	Why         string   `json:"why"`
	Suggestions []string `json:"suggestions"`
}

// FrameworkReviewInput carries the five立项 dimensions (+ the task title) the
// reviewer reads. Counterpoints is optional (may be empty).
type FrameworkReviewInput struct {
	Title         string
	Objective     string
	Reason        string
	Activities    string
	Resources     string
	Counterpoints string
}

const frameworkReviewSystem = `你是一位严谨而鼓励的 IB 导师。学生刚把研究框架的几件事聊清楚：目标、缘由、活动与时间、资源，以及可选的「可能的反例/张力」。请通读整份框架，判断它是否已经足够扎实、可以据此生成一份研究计划。

只返回一个 JSON 对象，形如：
{"ready": true, "why": "一句话总体判断", "suggestions": ["具体建议一", "具体建议二"]}

要求：
- ready 表示「足以据此推进到计划」，不是「完美」；即使 ready 也可以给改进建议。
- suggestions 要具体、可操作，指向最弱的一到两处（比如目标太泛、资源不落地、缺反例）；最多 4 条；若确实没有可改进处，给空数组。
- 用中文；不要复述学生原话，直接给判断与建议。
- 只回 JSON，不要任何解释或代码块外的文字。`

const maxFrameworkReviewAttempts = 2

// ReviewFramework runs the flagship reasoning reviewer over the framework and
// returns its verdict. Best-effort by contract: the caller degrades to "no
// verdict" on any error (nil resolver, provider failure, unparseable reply) and
// the turn/plan-gen proceed unchanged. Usage is returned so the caller can meter
// even a failed call (a rejected call still cost money).
func ReviewFramework(ctx context.Context, prov gateway.Provider, resolved gateway.Resolved, in FrameworkReviewInput) (FrameworkVerdict, gateway.ChatUsage, error) {
	var b strings.Builder
	if s := strings.TrimSpace(in.Title); s != "" {
		fmt.Fprintf(&b, "题目：%s\n", s)
	}
	fmt.Fprintf(&b, "目标：%s\n缘由：%s\n活动与时间：%s\n资源：%s\n",
		strings.TrimSpace(in.Objective), strings.TrimSpace(in.Reason),
		strings.TrimSpace(in.Activities), strings.TrimSpace(in.Resources))
	if s := strings.TrimSpace(in.Counterpoints); s != "" {
		fmt.Fprintf(&b, "可能的反例/张力：%s\n", s)
	}
	req := gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: frameworkReviewSystem},
			{Role: gateway.RoleUser, Content: b.String()},
		},
		MaxTokens: 3000, // reasoning-model headroom (deepseek-v4-pro flagship)
	}

	var lastUsage gateway.ChatUsage
	var lastErr error
	for attempt := 0; attempt < maxFrameworkReviewAttempts; attempt++ {
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
		lastErr = fmt.Errorf("framework review: no parseable verdict")
	}
	return FrameworkVerdict{}, lastUsage, lastErr
}

// parseFrameworkVerdict defensively decodes the model's JSON object (strip
// fences → clamp to the first '{'..last '}' → unmarshal), clamps suggestions to
// 4 non-empty entries, and requires a non-empty `why` so an empty object is
// treated as a parse failure (drives the retry).
func parseFrameworkVerdict(text string) (FrameworkVerdict, error) {
	raw := extractJSONObject(stripFences(text))
	if raw == "" {
		return FrameworkVerdict{}, fmt.Errorf("framework review: no JSON object in reply")
	}
	var v FrameworkVerdict
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return FrameworkVerdict{}, fmt.Errorf("framework review: unmarshal: %w", err)
	}
	if strings.TrimSpace(v.Why) == "" {
		return FrameworkVerdict{}, fmt.Errorf("framework review: empty verdict")
	}
	cleaned := make([]string, 0, len(v.Suggestions))
	for _, s := range v.Suggestions {
		if t := strings.TrimSpace(s); t != "" {
			cleaned = append(cleaned, t)
			if len(cleaned) == 4 {
				break
			}
		}
	}
	v.Suggestions = cleaned
	return v, nil
}
