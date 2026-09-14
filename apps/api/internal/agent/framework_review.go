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

const frameworkReviewSystem = `你是一位严谨的 IB 研究导师。请审阅学生填写的研究框架：目标、缘由、活动与时间、资源，以及可选的反例/张力。

ready 表示现有输入足以派生初步研究计划，AI 无需替学生发明研究范围、主要证据方向或核心分析方法。ready 也不表示正式提案已经达标；本审阅只给建议，不阻止后续流程。

这是 AND 门槛，不是综合评分。先得到五个布尔值，再严格执行：
ready = objective_ok AND activities_ok AND resources_ok AND coherence_ok AND reason_ok。
任一项为 false，ready 必须为 false；其他项不能抵消。

1. objective_ok 只根据目标字段判断。它须说明研究对象、必要范围，以及要比较、解释或判断什么。若目标与题目同义，只是「研究/探讨 + 原题复述」，objective_ok=false。活动、资源或缘由不得补全目标未表达的范围、指标或判断任务。
2. activities_ok：活动须说明要收集、比较、分析或检验什么；只有「找资料、做图、写作」或日程标签则为 false。
3. resources_ok：资源须给出证据类型、机构、数据库、材料或获取方向。可靠来源类型已经足够，不要求具体论文；只有「上网查、去图书馆」则为 false。
4. coherence_ok：活动和资源须共同服务同一目标，否则为 false。
5. reason_ok：缘由须表达与目标相关的真实疑问、经验或矛盾；仅为占位语或无关则为 false。

反例不进入上述 AND 公式。它缺失、跳过、较弱或尚未说明处理方式，均不能单独改变 ready。

只依据已填写内容，不假设之后会补充，不做外部事实核查，不补写缺失选择。输出前检查一致性：若 why 或 suggestions 指出仍须补充目标范围、判断任务、核心活动或证据方向，ready 必须为 false；ready 为 true 时建议只能是非必要优化。

只返回中文 JSON，不要代码块或其他文字：
{"ready": true, "why": "一句话总体判断", "suggestions": ["具体建议"]}
最多 4 条建议；false 时优先处理决定性缺口，不要大段复述原文。`

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
