package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"mindimprint/api/internal/gateway"
)

// ParentAdvice is one 在家可以怎么帮 item.
type ParentAdvice struct {
	Title string `json:"title"`
	Text  string `json:"text"`
}

// ParentProse is the ONLY thing the model contributes to the parent report:
// gentled wording over the finished canonical object. It cannot change any
// level, state, badge, or which dims/signals appear — those are deterministic.
// dReadings is keyed by depth code (only dims with a real level); aReadings by
// autonomy code (all six). Stored as the parent_report_prose.prose bundle.
type ParentProse struct {
	Glance      string            `json:"glance"`
	DOverview   string            `json:"dOverview"`
	AOverview   string            `json:"aOverview"`
	Opportunity string            `json:"opportunity"`
	WarmLine    string            `json:"warmLine"`
	DReadings   map[string]string `json:"dReadings"`
	AReadings   map[string]string `json:"aReadings"`
	Advice      []ParentAdvice    `json:"advice"`
}

// Rune-count caps (Chinese-first; a byte cap would let CJK blow past it).
const (
	parentGlanceMax   = 120
	parentOverviewMax = 120
	parentReadingMax  = 220
	parentOppMax      = 300
	parentWarmMax     = 160
	parentAdviceMax   = 220
)

// parentBareCode rejects internal register leaking into parent prose: bare axis
// codes (D3 / a 4), the opportunity enum, SOLO, and P0–P3. Case-insensitive and
// space-tolerant — the fact sheet is Chinese-only, so a real alphanumeric
// collision is rare and a false reject only fails this one compose call.
var parentBareCode = regexp.MustCompile(`(?i)(\b[da]\s*[1-6]\b|given_taken|given_not_taken|not_supplied|solo|\bp\s*[0-3]\b)`)

func parentSystemPrompt() string {
	return strings.Join([]string{
		"你在为一位学生的家长写一份「能力成长报告」的措辞。读者是家长，不是老师，也不是学生本人。",
		"你只负责措辞。每一维的等级、每个信号的状态，都已经由系统判定完毕，你不得改动，也不得新增或删减维度/信号。",
		"规则：",
		"1. 只使用给你的事实（每一维/信号的证据）。不得引入任何未给出的行为、数字或结论。",
		"2. 说人话、温和。绝不出现 D1–D6 / A1–A6 这类内部代码，也不出现 given_taken / SOLO / P0–P3 之类术语。",
		"3. A 轴（智识自主）只描述信号，绝不写任何数字或等级。",
		"4. 「机会供给先于判定」：平台没给到锻炼机会 = 平台欠账，不算孩子短板；只有「机会已给、没接住」才算孩子信号。opportunity 段要体现这一点，并如实描述真实性（哪些由 AI 代写需当面核对）。",
		"5. 不贴标签、不排名、不预测考分。只描述这一次作品里的行为。",
		"6. 只输出 JSON：{\"glance\":\"\",\"dOverview\":\"\",\"aOverview\":\"\",\"opportunity\":\"\",\"warmLine\":\"\",\"dReadings\":{\"D1\":\"\"},\"aReadings\":{\"A1\":\"\"},\"advice\":[{\"title\":\"\",\"text\":\"\"}]}",
		"dReadings 只为「给了读数的维度」写；aReadings 为全部六个信号写；advice 写 2–3 条家长在家可以怎么帮。",
	}, "\n")
}

// wantedDepth returns the depth codes that carry a real level — the only ones
// offered to the composer. NA/"" dims render 敢于空白 deterministically.
func wantedDepth(rep Report) []DepthDim {
	out := make([]DepthDim, 0, len(rep.DepthAxis))
	for _, d := range rep.DepthAxis {
		if d.Level == "L1" || d.Level == "L2" || d.Level == "L3" || d.Level == "L4" {
			out = append(out, d)
		}
	}
	return out
}

// ParentFactsPrompt renders the canonical evidence the composer may see —
// deliberately excludes promptLens, interactionEvidence, officialProjection and
// workAndProcess (家长端与教师端隔离).
func ParentFactsPrompt(rep Report, name, subject string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "学生：%s\n研究主题：%s\n", name, subject)
	b.WriteString("认知深度（给了读数的维度，需写 dReadings）：\n")
	for _, d := range wantedDepth(rep) {
		fmt.Fprintf(&b, "- %s %s：等级 %s，证据=%s\n", d.Code, d.Name, d.Level, d.Evidence)
	}
	b.WriteString("智识自主（全部六个信号，需写 aReadings，措辞里不得出现数字/等级）：\n")
	for _, a := range rep.AutonomyAxis {
		opp := map[string]string{"given_taken": "机会已给·已接住", "given_not_taken": "机会已给·没接住", "not_supplied": "平台还没提供机会"}[a.Opportunity]
		fmt.Fprintf(&b, "- %s %s：内部级别 %d（不得写出），机会=%s，证据=%s\n", a.Code, a.Name, a.Level, opp, a.Evidence)
	}
	return b.String()
}

// ComposeParent makes ONE flagship call gentling the canonical object into
// parent wording. Usage is returned even when the output is rejected, so the
// caller records the spend either way (mirrors ComposeWeekly / AssessReport).
func ComposeParent(ctx context.Context, prov gateway.Provider, r gateway.Resolved, rep Report, name, subject string) (ParentProse, gateway.ChatUsage, error) {
	res, err := gateway.Collect(ctx, prov, r, gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: parentSystemPrompt()},
			{Role: gateway.RoleUser, Content: ParentFactsPrompt(rep, name, subject)},
		},
	})
	if err != nil {
		return ParentProse{}, gateway.ChatUsage{}, err
	}
	usage := res.Usage
	var out ParentProse
	if err := json.Unmarshal([]byte(strings.TrimSpace(res.Text)), &out); err != nil {
		return ParentProse{}, usage, fmt.Errorf("agent: parent prose not JSON: %w", err)
	}
	if err := validateParentProse(out, rep); err != nil {
		return ParentProse{}, usage, err
	}
	return out, usage, nil
}

// validateParentProse enforces wording-only: every wanted depth dim and all six
// autonomy signals get exactly one non-empty reading keyed to a real code,
// overviews/glance/opportunity/warmLine/advice are non-empty and within caps,
// and no internal code/term leaks (说人话).
func validateParentProse(p ParentProse, rep Report) error {
	// Depth: readings for exactly the real-level dims.
	wantD := map[string]bool{}
	for _, d := range wantedDepth(rep) {
		wantD[d.Code] = false
	}
	for code, txt := range p.DReadings {
		seen, known := wantD[code]
		if !known {
			return fmt.Errorf("agent: parent prose has an unwanted depth reading %q", code)
		}
		if seen {
			return fmt.Errorf("agent: parent prose repeats depth reading %q", code)
		}
		wantD[code] = true
		if strings.TrimSpace(txt) == "" {
			return fmt.Errorf("agent: parent prose leaves depth %q empty", code)
		}
		if utf8.RuneCountInString(txt) > parentReadingMax {
			return fmt.Errorf("agent: parent depth reading %q too long", code)
		}
	}
	for code, seen := range wantD {
		if !seen {
			return fmt.Errorf("agent: parent prose missing depth reading %q", code)
		}
	}
	// Autonomy: all six signals.
	wantA := map[string]bool{}
	for _, a := range rep.AutonomyAxis {
		wantA[a.Code] = false
	}
	for code, txt := range p.AReadings {
		seen, known := wantA[code]
		if !known {
			return fmt.Errorf("agent: parent prose has an unknown autonomy reading %q", code)
		}
		if seen {
			return fmt.Errorf("agent: parent prose repeats autonomy reading %q", code)
		}
		wantA[code] = true
		if strings.TrimSpace(txt) == "" {
			return fmt.Errorf("agent: parent prose leaves autonomy %q empty", code)
		}
		if utf8.RuneCountInString(txt) > parentReadingMax {
			return fmt.Errorf("agent: parent autonomy reading %q too long", code)
		}
	}
	for code, seen := range wantA {
		if !seen {
			return fmt.Errorf("agent: parent prose missing autonomy reading %q", code)
		}
	}
	// Scalars non-empty + capped.
	scalars := []struct {
		name, val string
		max       int
	}{
		{"glance", p.Glance, parentGlanceMax}, {"dOverview", p.DOverview, parentOverviewMax},
		{"aOverview", p.AOverview, parentOverviewMax}, {"opportunity", p.Opportunity, parentOppMax},
		{"warmLine", p.WarmLine, parentWarmMax},
	}
	for _, s := range scalars {
		if strings.TrimSpace(s.val) == "" {
			return fmt.Errorf("agent: parent prose leaves %s empty", s.name)
		}
		if utf8.RuneCountInString(s.val) > s.max {
			return fmt.Errorf("agent: parent prose %s too long", s.name)
		}
	}
	if len(p.Advice) == 0 {
		return fmt.Errorf("agent: parent prose has no advice")
	}
	for _, ad := range p.Advice {
		if strings.TrimSpace(ad.Title) == "" || strings.TrimSpace(ad.Text) == "" {
			return fmt.Errorf("agent: parent advice item empty")
		}
		if utf8.RuneCountInString(ad.Title) > parentAdviceMax {
			return fmt.Errorf("agent: parent advice title too long")
		}
		if utf8.RuneCountInString(ad.Text) > parentAdviceMax {
			return fmt.Errorf("agent: parent advice too long")
		}
	}
	// 说人话: no internal register anywhere.
	texts := []string{p.Glance, p.DOverview, p.AOverview, p.Opportunity, p.WarmLine}
	for _, t := range p.DReadings {
		texts = append(texts, t)
	}
	for _, t := range p.AReadings {
		texts = append(texts, t)
	}
	for _, ad := range p.Advice {
		texts = append(texts, ad.Title, ad.Text)
	}
	for _, t := range texts {
		if parentBareCode.MatchString(t) {
			return fmt.Errorf("agent: parent prose contains an internal code/term")
		}
	}
	return nil
}
