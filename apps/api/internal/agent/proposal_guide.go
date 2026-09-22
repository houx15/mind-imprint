package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"mindimprint/api/internal/gateway"
)

// proposal_guide.go — slice 3a · generates the guide card for one proposal step
// on the FAST model. A guide card is a guiding question (Chinese, applied to the
// current prompt) + an English example (for a DIFFERENT prompt, never the answer
// here — 铁律①) + an optional pointer into the framework. The user-message
// context branches on the step kind (fixed / subq-define / subq). Cards are
// cached by the REST layer (StepGuides[key]) so this runs at most once per step.

// GuideCardOut is the parsed guide card.
type GuideCardOut struct {
	Prompt  string `json:"prompt"`
	Example string `json:"example"`
	RefHint string `json:"refHint"`
}

// GuideGenInput carries everything the generator needs. Objective/Reason/…
// are the framework dims; ThisSubQuestion + SiblingSubQuestions are set only for
// a KindSubq step.
type GuideGenInput struct {
	Title      string
	Objective  string
	Reason     string
	Activities string
	Resources  string
	Step       Step
	Doc        string // "" | "proposal" | "essay" — selects the guidance wording

	ThisSubQuestion     string
	SiblingSubQuestions []SubQuestion

	// ChallengeEvidence — slice 4b-2 · titles of references the student tagged
	// 反驳/张力 (evidenceNature="challenge") in the research stage. Only set for the
	// essay "challenges" step so the 面对反方观点 card names the real counter-evidence.
	ChallengeEvidence []string
}

// essayGuideBody produces the essay statement-stage guidance for a step (§6
// stage-2). Claim steps walk 证据/分析/局限/衔接 for that one claim; the fixed
// steps guide outline / 比较综合 / 面对反方观点 / 结论 / 论证结构.
func essayGuideBody(in GuideGenInput) string {
	var b strings.Builder
	switch in.Step.Kind {
	case KindSubq:
		b.WriteString("\n本部分：为下面这条【论点】写论述段落——它的证据、对证据的分析、它的局限、以及它如何与其它论点衔接、共同回答核心问题。一次只写这一条论点。\n")
		fmt.Fprintf(&b, "当前论点（子问题）：%s\n", strings.TrimSpace(in.ThisSubQuestion))
		if len(in.SiblingSubQuestions) > 0 {
			b.WriteString("全部论点（用于说明衔接）：\n")
			for i, sq := range in.SiblingSubQuestions {
				fmt.Fprintf(&b, "  %d. %s\n", i+1, strings.TrimSpace(sq.Text))
			}
		}
	default:
		switch in.Step.Key {
		case "outline":
			b.WriteString("\n本部分：根据你搜集到的证据，调整大纲——各条论点的顺序、层次与它们之间的关系。\n")
		case "synthesis":
			b.WriteString("\n本部分：比较 / 综合各条论点——它们如何共同回答核心研究问题？哪些观点相互补充、哪些观点存在分歧？\n")
		case "challenges":
			b.WriteString("\n本部分：面对最强的反方观点 / 替代解释 / 不同视角——你如何回应？（这一步不能跳过。）\n")
			if len(in.ChallengeEvidence) > 0 {
				b.WriteString("你在研究阶段标记为『反驳/张力』的材料（引导学生具体回应它们）：\n")
				for i, t := range in.ChallengeEvidence {
					fmt.Fprintf(&b, "  %d. %s\n", i+1, strings.TrimSpace(t))
				}
			}
		case "conclusion":
			b.WriteString("\n本部分：基于前面各条论证，写出你的结论。\n")
		case "structure":
			b.WriteString("\n本部分：用一段话描述整篇文章的论证结构——各部分分别提供什么依据、如何共同得出结论。\n")
		case "sub:intro":
			b.WriteString("\n本部分：写引言。引导学生说清楚五件事——① 背景；② 确切的研究问题；③ 关键词与范围（工作定义）；④ 这个问题为什么重要；⑤ 全文论证结构的走向（先说什么、再说什么）。\n")
		case "sub:conclusion":
			b.WriteString("\n本部分：写结论。基于前面各条论证，收束回核心问题——你的回答是什么、在什么条件下成立、还留下什么未解。\n")
		case "sub:compose":
			b.WriteString("\n本部分：把已写好的引言、各条论点、比较/综合、反方回应、结论拼成一整篇，读一遍衔接是否顺畅。（这一步是组装，不是重写。）\n")
		case "sub:polish":
			b.WriteString("\n本部分：通读全文、润色语言与衔接。请印记通篇审阅，你据此修改，满意后完成整篇论文。\n")
		default:
			fmt.Fprintf(&b, "\n本部分：%s。请针对当前题目引导学生写这一部分。\n", in.Step.Title)
		}
	}
	return b.String()
}

const guideGenSystem = `你是一位 IB 写作陪练。学生正在写研究提案或论文的某一部分，以输入指定的文档和部分为准。卡片直接展示给学生，用“你”称呼学生，清楚说明当前要思考的问题。请只为这一部分生成一张「引导卡」，帮助学生自己写——你不替学生写正文。

只返回一个 JSON 对象：
{"prompt": "给学生的引导问题（中文，套用到学生当前的题目/研究问题上，一次只问这一部分）", "example": "一个英文范例（针对一个【不同的】题目，示范这一部分怎么写，绝不是当前题目的答案）", "refHint": "可选，一句话指向学生可以参考的框架内容，如「参考研究框架中的目标」"}

要求：
- prompt 具体、聚焦这一部分，套用到当前题目；一次只引导这一部分。
- example 必须是英文，且是【别的题目】的示范，不能是当前题目的答案。
- 只回 JSON，不要代码块外的任何文字。`

const maxGuideGenAttempts = 2

// guideGenUserContent builds the user message, branching on the step kind. Pure
// + exported-to-package so the branching is unit-tested without a provider.
func guideGenUserContent(in GuideGenInput) string {
	var b strings.Builder
	if s := strings.TrimSpace(in.Title); s != "" {
		fmt.Fprintf(&b, "题目：%s\n", s)
	}
	// The framework dims give the generator the student's own context.
	if s := strings.TrimSpace(in.Objective); s != "" {
		fmt.Fprintf(&b, "研究问题（目标）：%s\n", s)
	}
	if s := strings.TrimSpace(in.Reason); s != "" {
		fmt.Fprintf(&b, "缘由：%s\n", s)
	}
	if s := strings.TrimSpace(in.Resources); s != "" {
		fmt.Fprintf(&b, "已有资源：%s\n", s)
	}

	if in.Doc == "essay" {
		b.WriteString(essayGuideBody(in))
		return b.String()
	}

	switch in.Step.Kind {
	case KindSubqDefine:
		b.WriteString("\n本部分：研究计划 · 定子问题。这是提案最重要的一步。请引导学生把关键研究问题拆成 2–4 个【可研究、相互关联、能共同构成关键问题】的子问题（是一条相互推进的链，不是一串话题）。若学生还没有暂定观点或子问题的想法，提醒学生可以先做一点文献探索再回来。")
	case KindSubq:
		b.WriteString("\n本部分：为下面这个【单个子问题】写清楚——它要解决什么、如何与关键研究问题相关、学生当前的观点与哪些材料或证据能提供依据、以及它如何推进下一部分。一次只引导这一个子问题。\n")
		fmt.Fprintf(&b, "当前子问题：%s\n", strings.TrimSpace(in.ThisSubQuestion))
		if len(in.SiblingSubQuestions) > 0 {
			b.WriteString("全部子问题（用于说明它如何与其它子问题衔接）：\n")
			for i, sq := range in.SiblingSubQuestions {
				fmt.Fprintf(&b, "  %d. %s\n", i+1, strings.TrimSpace(sq.Text))
			}
		}
	default: // KindFixed
		fmt.Fprintf(&b, "\n本部分：%s。请针对当前题目引导学生写这一部分。\n", in.Step.Title)
	}
	return b.String()
}

// GenerateProposalGuideStep generates the guide card for in.Step on the fast
// model. Best-effort by contract: the caller degrades to a null card on any
// error (nil resolver, provider failure, unparseable/empty reply). Usage is
// returned so the caller meters even a failed call.
func GenerateProposalGuideStep(ctx context.Context, prov gateway.Provider, resolved gateway.Resolved, in GuideGenInput) (GuideCardOut, gateway.ChatUsage, error) {
	req := gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: guideGenSystem},
			{Role: gateway.RoleUser, Content: guideGenUserContent(in)},
		},
		MaxTokens: 1200,
	}

	var lastUsage gateway.ChatUsage
	var lastErr error
	for attempt := 0; attempt < maxGuideGenAttempts; attempt++ {
		res, err := gateway.Collect(ctx, prov, resolved, req)
		lastUsage = res.Usage
		if err != nil {
			lastErr = err
			continue
		}
		card, perr := parseGuideCard(res.Text)
		if perr != nil {
			lastErr = perr
			continue
		}
		return card, lastUsage, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("guide gen: no parseable card")
	}
	return GuideCardOut{}, lastUsage, lastErr
}

func parseGuideCard(text string) (GuideCardOut, error) {
	raw := extractJSONObject(stripFences(text))
	if raw == "" {
		return GuideCardOut{}, fmt.Errorf("guide gen: no JSON object in reply")
	}
	var c GuideCardOut
	if err := json.Unmarshal([]byte(raw), &c); err != nil {
		return GuideCardOut{}, fmt.Errorf("guide gen: unmarshal: %w", err)
	}
	if strings.TrimSpace(c.Prompt) == "" {
		return GuideCardOut{}, fmt.Errorf("guide gen: empty prompt")
	}
	return c, nil
}
