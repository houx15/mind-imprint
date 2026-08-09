package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"mindimprint/api/internal/gateway"
)

// evidence_saturation.go — slice 4a · the flagship reviewer that judges whether
// ONE sub-question's evidence map is "saturated" enough to write its claim from
// (§6). Advisory only (铁律②) — the student can write or read on regardless. It
// reads the papers gathered under the sub-question (their 支持/反驳 nature +
// argument + finding) and judges three things (the user's own criteria):
//  1. enough strong, reliable material that supports the claim;
//  2. at least one limitation / challenge / substitute explanation / different
//     perspective (支持 alone is never saturated — the 撞反例 ethos);
//  3. the real test: new reading has started repeating what's already gathered.

// SubQuestionVerdictOut is the reviewer's read (server mints subQuestionId).
type SubQuestionVerdictOut struct {
	Saturated bool     `json:"saturated"`
	Why       string   `json:"why"`
	Gaps      []string `json:"gaps"`
}

// EvidencePaper is one gathered paper's evidence facets.
type EvidencePaper struct {
	Title    string
	Nature   string // support | challenge | ""
	Argument string
	Finding  string
}

// EvidenceReviewInput is one sub-question + its gathered evidence.
type EvidenceReviewInput struct {
	SubQuestion string
	Papers      []EvidencePaper
}

const evidenceSaturationSystem = `你是一位严谨而鼓励的 IB 研究导师。学生正在为一个子问题搜集证据，建「证据地图」。请判断这个子问题的证据是否已经"饱和"——足以据此写出这条论点。

饱和的三条标准：
1. 有足够强、足够可靠的材料支持这条论点。
2. 至少有一条局限 / 反例 / 替代解释 / 不同视角（只有支持材料，永远不算饱和）。
3. 真正的饱和信号：再找新材料，基本都在重复已有的内容。

只返回一个 JSON 对象：
{"saturated": false, "why": "一句话总体判断", "gaps": ["还缺什么，具体一点", "……"]}

要求：
- saturated 表示"足以据此写这条论点"，不是"读完了所有文献"。
- gaps 具体、可操作，指向最关键的缺口（比如缺反例、支持材料太弱、只有一种视角）；最多 4 条；若确实饱和给空数组。
- 用中文；只回 JSON，不要任何解释或代码块外的文字。`

const maxEvidenceSaturationAttempts = 2

// ReviewEvidenceSaturation runs the flagship reviewer over one sub-question's
// evidence. Best-effort by contract: any error → the caller degrades (treats it
// as no-verdict / saturated). Usage is returned so the caller meters.
func ReviewEvidenceSaturation(ctx context.Context, prov gateway.Provider, resolved gateway.Resolved, in EvidenceReviewInput) (SubQuestionVerdictOut, gateway.ChatUsage, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "子问题：%s\n\n已搜集的材料：\n", strings.TrimSpace(in.SubQuestion))
	if len(in.Papers) == 0 {
		b.WriteString("（还没有材料）\n")
	}
	for i, p := range in.Papers {
		nature := "未标注"
		switch p.Nature {
		case "support":
			nature = "支持"
		case "challenge":
			nature = "反驳/张力"
		}
		fmt.Fprintf(&b, "%d. 《%s》[%s] 论点：%s 证据：%s\n", i+1, strings.TrimSpace(p.Title), nature,
			strings.TrimSpace(p.Argument), strings.TrimSpace(p.Finding))
	}

	req := gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: evidenceSaturationSystem},
			{Role: gateway.RoleUser, Content: b.String()},
		},
		MaxTokens: 3000,
	}

	var lastUsage gateway.ChatUsage
	var lastErr error
	for attempt := 0; attempt < maxEvidenceSaturationAttempts; attempt++ {
		res, err := gateway.Collect(ctx, prov, resolved, req)
		lastUsage = res.Usage
		if err != nil {
			lastErr = err
			continue
		}
		v, perr := parseSaturationVerdict(res.Text)
		if perr != nil {
			lastErr = perr
			continue
		}
		return v, lastUsage, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("evidence saturation: no parseable verdict")
	}
	return SubQuestionVerdictOut{}, lastUsage, lastErr
}

func parseSaturationVerdict(text string) (SubQuestionVerdictOut, error) {
	raw := extractJSONObject(stripFences(text))
	if raw == "" {
		return SubQuestionVerdictOut{}, fmt.Errorf("evidence saturation: no JSON object in reply")
	}
	var v SubQuestionVerdictOut
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return SubQuestionVerdictOut{}, fmt.Errorf("evidence saturation: unmarshal: %w", err)
	}
	if strings.TrimSpace(v.Why) == "" {
		return SubQuestionVerdictOut{}, fmt.Errorf("evidence saturation: empty verdict")
	}
	cleaned := make([]string, 0, len(v.Gaps))
	for _, g := range v.Gaps {
		if t := strings.TrimSpace(g); t != "" {
			cleaned = append(cleaned, t)
			if len(cleaned) == 4 {
				break
			}
		}
	}
	v.Gaps = cleaned
	return v, nil
}
