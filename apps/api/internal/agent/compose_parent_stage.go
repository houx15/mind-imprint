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

// ParentStageProse is the ONLY thing the model contributes to a stage report:
// gentled family-facing wording over deterministic usage stats + the
// cross-session ability standing. It cannot change any number — those are
// computed and rendered deterministically. Stored as parent_report_prose.prose
// under surface='stage'.
type ParentStageProse struct {
	WarmLine       string         `json:"warmLine"`
	StageGrowth    string         `json:"stageGrowth"`
	StageHighlight string         `json:"stageHighlight"` // may be "" (敢于空白)
	StageForward   string         `json:"stageForward"`
	Advice         []ParentAdvice `json:"advice"`
}

// AbilityDepthFact / AbilitySummary are a plain, import-cycle-safe projection of
// ability.Model (which imports agent). The internal/api handler builds this from
// ability.Aggregate and hands it to the composer as reference substance — its
// numbers/labels are input only and must never surface in the OUTPUT.
type AbilityDepthFact struct {
	Name          string
	LevelLabel    string // "" ⇒ 证据不足·需更多任务
	EvidenceCount int
}

type AbilitySummary struct {
	TotalSessions       int
	BoundarySettings    int
	AdversaryInvites    int
	OpportunitiesTaken  int
	OpportunitiesMissed int
	Depth               []AbilityDepthFact
}

// ParentStageFacts is the composer input for one (student, week).
type ParentStageFacts struct {
	Name    string
	Subject string // week label, e.g. 第 30 周（7.20–7.26）
	Klass   string

	ActiveDays  int
	Turns       int
	Reports     int
	CourseSteps int

	Ability AbilitySummary
}

// parentStageBareCode extends E1's leak guard with bare LEVEL codes (L1–L4):
// the composer is fed level labels, but the parent surface shows only the 台阶
// words 起步/发展/熟练/优秀, never L3. Case-insensitive, space-tolerant.
var parentStageBareCode = regexp.MustCompile(`(?i)(\b[da]\s*[1-6]\b|\bl\s*[1-4]\b|given_taken|given_not_taken|not_supplied|solo|\bp\s*[0-3]\b)`)

func parentStageSystemPrompt() string {
	return strings.Join([]string{
		"你在为一位学生的家长写一份「阶段成长报告」的措辞。读者是家长，不是老师，也不是学生本人。",
		"你只负责措辞。所有数字（使用天数、对话轮次等）都已算好并会另行展示，你不得改动，也不得编造未给出的数字或结论。",
		"规则：",
		"1. 只使用给你的事实（本阶段使用数据 + 跨会话能力概况）。不得引入任何未给出的行为、数字或结论。",
		"2. 说人话、温和。绝不出现 D1–D6 / A1–A6 / L1–L4 这类内部代码或字母数字等级，也不出现 given_taken / SOLO / P0–P3 之类术语。",
		"3. 描述「这段时间的变化」时，只用台阶词（起步/发展/熟练/优秀）或大白话，绝不写出等级数字/字母。",
		"4. 「机会供给先于判定」+「敢于空白」：使用很少或刚起步时，如实说「刚起步、暂未见到明显提升」，不硬凑亮点；本阶段没有明显亮点时，stageHighlight 留空字符串。",
		"5. 不贴标签、不排名、不预测考分。只描述这一阶段观察到的行为与变化。",
		"6. 只输出 JSON：{\"warmLine\":\"\",\"stageGrowth\":\"\",\"stageHighlight\":\"\",\"stageForward\":\"\",\"advice\":[{\"title\":\"\",\"text\":\"\"}]}",
		"advice 写恰好 3 条家长在家可以怎么帮；stageHighlight 可为空字符串。",
	}, "\n")
}

// ParentStageFactsPrompt renders the reference substance the composer may see.
// Deliberately excludes any project-scoped canonical readings — a stage report
// is not scoped to one project.
func ParentStageFactsPrompt(f ParentStageFacts) string {
	var b strings.Builder
	fmt.Fprintf(&b, "学生：%s\n班级：%s\n时间范围：%s\n", f.Name, f.Klass, f.Subject)
	fmt.Fprintf(&b, "本阶段使用：活跃 %d 天，AI 对话 %d 轮，生成能力报告 %d 份，完成课程 %d 节。\n",
		f.ActiveDays, f.Turns, f.Reports, f.CourseSteps)
	b.WriteString("跨会话能力当前概况（仅供你参考，措辞里不得写出等级代码或字母数字等级）：\n")
	for _, d := range f.Ability.Depth {
		label := d.LevelLabel
		if label == "" {
			label = "证据不足·需更多任务"
		}
		fmt.Fprintf(&b, "- %s：%s（累计证据 %d 次）\n", d.Name, label, d.EvidenceCount)
	}
	fmt.Fprintf(&b, "自主观察：累计参与 %d 次会话；主动设界 %d 次，主动请对手检验 %d 次；机会已给并接住 %d 次，机会已给但未接住 %d 次。\n",
		f.Ability.TotalSessions, f.Ability.BoundarySettings, f.Ability.AdversaryInvites,
		f.Ability.OpportunitiesTaken, f.Ability.OpportunitiesMissed)
	return b.String()
}

// ComposeParentStage makes ONE flagship call. Usage is returned even when the
// output is rejected, so the caller records the spend either way.
func ComposeParentStage(ctx context.Context, prov gateway.Provider, r gateway.Resolved, f ParentStageFacts) (ParentStageProse, gateway.ChatUsage, error) {
	res, err := gateway.Collect(ctx, prov, r, gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: parentStageSystemPrompt()},
			{Role: gateway.RoleUser, Content: ParentStageFactsPrompt(f)},
		},
	})
	if err != nil {
		return ParentStageProse{}, gateway.ChatUsage{}, err
	}
	usage := res.Usage
	var out ParentStageProse
	if err := json.Unmarshal([]byte(strings.TrimSpace(res.Text)), &out); err != nil {
		return ParentStageProse{}, usage, fmt.Errorf("agent: parent stage prose not JSON: %w", err)
	}
	if err := validateParentStageProse(out); err != nil {
		return ParentStageProse{}, usage, err
	}
	return out, usage, nil
}

// validateParentStageProse enforces wording-only: warmLine/stageGrowth/
// stageForward non-empty + capped; stageHighlight optional but capped; exactly
// 3 advice items each non-empty + capped; no internal register leaks anywhere.
func validateParentStageProse(p ParentStageProse) error {
	req := []struct {
		name, val string
		max       int
	}{
		{"warmLine", p.WarmLine, parentWarmMax},
		{"stageGrowth", p.StageGrowth, parentOppMax},
		{"stageForward", p.StageForward, parentWarmMax},
	}
	for _, s := range req {
		if strings.TrimSpace(s.val) == "" {
			return fmt.Errorf("agent: parent stage prose leaves %s empty", s.name)
		}
		if utf8.RuneCountInString(s.val) > s.max {
			return fmt.Errorf("agent: parent stage prose %s too long", s.name)
		}
	}
	if utf8.RuneCountInString(p.StageHighlight) > parentOppMax {
		return fmt.Errorf("agent: parent stage highlight too long")
	}
	if len(p.Advice) != 3 {
		return fmt.Errorf("agent: parent stage prose needs exactly 3 advice items, got %d", len(p.Advice))
	}
	for _, ad := range p.Advice {
		if strings.TrimSpace(ad.Title) == "" || strings.TrimSpace(ad.Text) == "" {
			return fmt.Errorf("agent: parent stage advice item empty")
		}
		if utf8.RuneCountInString(ad.Title) > parentAdviceMax || utf8.RuneCountInString(ad.Text) > parentAdviceMax {
			return fmt.Errorf("agent: parent stage advice too long")
		}
	}
	texts := []string{p.WarmLine, p.StageGrowth, p.StageHighlight, p.StageForward}
	for _, ad := range p.Advice {
		texts = append(texts, ad.Title, ad.Text)
	}
	for _, t := range texts {
		if parentStageBareCode.MatchString(t) {
			return fmt.Errorf("agent: parent stage prose contains an internal code/term")
		}
	}
	return nil
}
