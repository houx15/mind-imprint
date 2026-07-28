package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"mindimprint/api/internal/gateway"
)

// MirrorSection is one titled paragraph of the 你的思维印记 narrative.
type MirrorSection struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

// Mirror is the whole 你的思维印记 reflection mirror: a short restrained
// narrative (a few titled sections) plus two forward-looking carry-forwards.
// It is "for reference, not a grade" — nothing here is a score. Stored as the
// project_mirror_prose row (sections + carry_forwards jsonb).
type Mirror struct {
	Sections      []MirrorSection `json:"sections"`
	CarryForwards []string        `json:"carryForwards"`
}

// MirrorInput is the honest process record the composer may draw on. Every
// field is optional; the composer must only speak to what is actually present
// (never fabricate a card that was not summoned or a source that was not read).
type MirrorInput struct {
	StudentName   string
	Objective     string   // 开题四问 · 目标
	Reason        string   // 开题四问 · 为什么
	Activities    string   // 开题四问 · 怎么做
	Resources     string   // 开题四问 · 需要什么
	Reflection    []string // the student's own five-dimension answers
	Outline       string   // the writing outline, indented
	Draft         string   // the current draft (edit buffer)
	ProcessDigest string   // a digest of events + summoned cards
}

// Rune-count caps (Chinese-first; a byte cap would let CJK blow past it).
const (
	mirrorTitleMax   = 40
	mirrorBodyMax    = 400
	mirrorCarryMax   = 200
	mirrorMaxSecs    = 6
	mirrorMaxCarries = 4
)

func mirrorSystemPrompt() string {
	return strings.Join([]string{
		"你在为一名学生写一份「你的思维印记」——这一次研究项目的过程镜像。读者是学生本人。",
		"这不是评分、不是评语、不是打分表。你只是把「过程」如实地照回给学生看，让 TA 看清自己这一路是怎么想的。",
		"用第二人称「你」，克制、诚恳、具体，不夸奖也不说教。只写这一次作品里真实发生过的事。",
		"可以围绕这些线索来写（只写有材料支撑的）：",
		"- 你的论点（thesis）是怎么长出来、又怎么被修正的；",
		"- 阅读是怎么喂给写作的（哪段材料真的进了你的论证）；",
		"- 哪些地方是你自己想明白的，哪些地方你靠了「印记」；",
		"- 你召唤了哪些思维工具卡，它们帮你卡住了什么。",
		"规则：",
		"1. 只使用给你的事实。绝不引入未给出的行为、数字、来源或结论。材料稀薄时就写得短，宁可留白，也不要编。",
		"2. 不贴标签、不排名、不预测考分、不给等级。",
		"3. carryForwards 写两条「带走的下一步」——面向未来、可操作、温和。",
		"4. 只输出 JSON：{\"sections\":[{\"title\":\"\",\"body\":\"\"}],\"carryForwards\":[\"\",\"\"]}",
		"   sections 写 2–4 段，每段一个小标题；body 是一小段话。carryForwards 写两条。",
	}, "\n")
}

// mirrorFactsPrompt renders only the process facts that actually exist — an
// absent field is simply omitted, never padded with a placeholder.
func mirrorFactsPrompt(in MirrorInput) string {
	var b strings.Builder
	if strings.TrimSpace(in.StudentName) != "" {
		fmt.Fprintf(&b, "学生：%s\n", in.StudentName)
	}
	kickoff := []struct{ label, val string }{
		{"目标", in.Objective}, {"为什么做", in.Reason},
		{"打算怎么做", in.Activities}, {"需要什么", in.Resources},
	}
	kickoffWritten := false
	for _, k := range kickoff {
		if strings.TrimSpace(k.val) != "" {
			if !kickoffWritten {
				b.WriteString("【开题四问】\n")
				kickoffWritten = true
			}
			fmt.Fprintf(&b, "- %s：%s\n", k.label, strings.TrimSpace(k.val))
		}
	}
	reflWritten := false
	for i, ans := range in.Reflection {
		if strings.TrimSpace(ans) != "" {
			if !reflWritten {
				b.WriteString("【学生自己的回顾】\n")
				reflWritten = true
			}
			fmt.Fprintf(&b, "%d. %s\n", i+1, strings.TrimSpace(ans))
		}
	}
	if strings.TrimSpace(in.Outline) != "" {
		b.WriteString("【写作提纲】\n")
		b.WriteString(strings.TrimSpace(in.Outline) + "\n")
	}
	if strings.TrimSpace(in.ProcessDigest) != "" {
		b.WriteString("【过程与工具卡】\n")
		b.WriteString(strings.TrimSpace(in.ProcessDigest) + "\n")
	}
	if strings.TrimSpace(in.Draft) != "" {
		b.WriteString("【当前草稿】\n")
		b.WriteString(strings.TrimSpace(in.Draft) + "\n")
	}
	if b.Len() == 0 {
		return "（这次项目留下的过程材料很少，请只写一两句克制的话，并留出空白。）"
	}
	return b.String()
}

// MinimalMirror is the graceful fallback stored when no provider is available
// or the composition fails — the room still shows a mirror (never a 500, never
// a blank pane), just a restrained one that invites the student to look back.
func MinimalMirror() Mirror {
	return Mirror{
		Sections: []MirrorSection{{
			Title: "这一次的过程",
			Body:  "这次的过程镜像还没能生成完整的叙述。不过你留下的每一步——开题、阅读、写作、回顾——都已经记在你的成长报告里了。回到项目里翻一翻，你会看到自己是怎么一路想过来的。",
		}},
		CarryForwards: []string{
			"下一次，试着在动笔前先把「我到底想论证什么」写成一句话。",
			"读到关键材料时，随手记下「这一条能接上我论证的哪一步」。",
		},
	}
}

// ComposeMirror makes ONE flagship call turning the process record into the
// 你的思维印记 narrative. Usage is returned even when the output is rejected,
// so the caller records the spend either way (mirrors ComposeParent /
// AssessReport). A rejected/malformed output is an error; the caller then
// stores MinimalMirror instead.
func ComposeMirror(ctx context.Context, prov gateway.Provider, r gateway.Resolved, in MirrorInput) (Mirror, gateway.ChatUsage, error) {
	res, err := gateway.Collect(ctx, prov, r, gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: mirrorSystemPrompt()},
			{Role: gateway.RoleUser, Content: mirrorFactsPrompt(in)},
		},
	})
	if err != nil {
		return Mirror{}, gateway.ChatUsage{}, err
	}
	usage := res.Usage
	var out Mirror
	if err := json.Unmarshal([]byte(strings.TrimSpace(res.Text)), &out); err != nil {
		return Mirror{}, usage, fmt.Errorf("agent: mirror not JSON: %w", err)
	}
	cleaned, err := sanitizeMirror(out)
	if err != nil {
		return Mirror{}, usage, err
	}
	return cleaned, usage, nil
}

// sanitizeMirror keeps only non-empty, in-cap sections/carry-forwards and
// requires at least one real section and one real carry-forward. It trims
// (never expands) the model output — nothing is invented here.
func sanitizeMirror(m Mirror) (Mirror, error) {
	out := Mirror{}
	for _, s := range m.Sections {
		title, body := strings.TrimSpace(s.Title), strings.TrimSpace(s.Body)
		if title == "" || body == "" {
			continue
		}
		if utf8.RuneCountInString(title) > mirrorTitleMax || utf8.RuneCountInString(body) > mirrorBodyMax {
			return Mirror{}, fmt.Errorf("agent: mirror section too long")
		}
		out.Sections = append(out.Sections, MirrorSection{Title: title, Body: body})
		if len(out.Sections) >= mirrorMaxSecs {
			break
		}
	}
	for _, c := range m.CarryForwards {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		if utf8.RuneCountInString(c) > mirrorCarryMax {
			return Mirror{}, fmt.Errorf("agent: mirror carry-forward too long")
		}
		out.CarryForwards = append(out.CarryForwards, c)
		if len(out.CarryForwards) >= mirrorMaxCarries {
			break
		}
	}
	if len(out.Sections) == 0 {
		return Mirror{}, fmt.Errorf("agent: mirror has no usable section")
	}
	if len(out.CarryForwards) == 0 {
		return Mirror{}, fmt.Errorf("agent: mirror has no carry-forward")
	}
	return out, nil
}
