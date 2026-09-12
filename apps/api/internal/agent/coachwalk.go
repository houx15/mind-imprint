package agent

import (
	"fmt"
	"strings"

	"mindimprint/api/internal/agent/enforcement"
	"mindimprint/api/internal/coachwalk"
	"mindimprint/api/internal/gateway"
)

// 这个包里两个陪练的多轮走查驱动。它们住在这里，和 benchcases.go 同一个理由：
// projectCoachPosturePrompt / coachPosturePrompt 是未导出的，抄一份出去就会漂移。
// 请求路径上没有任何东西调用它们。

/* ── 写作室陪练（lite，线上） ────────────────────────────────────────────── */

// WritingWalkDriver 驱动 agent.ProposeProjectCoachReply 那一条。
//
// 🚨 它的 Parse 用的是**生产在每一条回复上都跑的那一对校验器**：
// ValidateOutput + BannedPhrasing。BannedPhrasing 是一张「这句话等于 AI 替她写了」
// 的措辞清单——所以这个陪练有整套走查里**唯一客观的铁律① 信号**，不靠判官。
type WritingWalkDriver struct {
	history []ChatTurn
}

func NewWritingWalkDriver() *WritingWalkDriver {
	return &WritingWalkDriver{history: []ChatTurn{
		{Role: "assistant", Content: "你这一段想让读者接受的是哪一句？"},
		{Role: "user", Content: "就是中国的能源转型其实是有效的。"},
		{Role: "assistant", Content: "有效——是指投进去的多，还是排出来的少？"},
		{Role: "user", Content: "我写的是装机量第一。但你这么一问我发现这两个不是一回事。那我这段该怎么改？"},
	}}
}

func (d *WritingWalkDriver) Site() string {
	return "agent.ProposeProjectCoachReply (POST /writings/{id}/turn)"
}

const writingWalkProjection = `研究问题：中国是否让地球变得更可持续？
已完成：来源体检（NASA 报道 / Nature Sustainability 论文）
当前部分：第二段——中国的能源投入
已知反例：中国碳排放总量全球第一（她打算写让步段）`

func (d *WritingWalkDriver) Request() gateway.ChatRequest {
	return gateway.ChatRequest{
		MaxTokens: 2000,
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: projectCoachPosturePrompt},
			{Role: gateway.RoleUser, Content: BuildProjectCoachContext(d.history, writingWalkProjection, "写作间")},
		},
	}
}

func (d *WritingWalkDriver) Parse(raw string) (string, []coachwalk.Violation, error) {
	out := enforcement.AgentOutput{Type: "reply", Body: strings.TrimSpace(raw)}
	if err := enforcement.ValidateOutput(out); err != nil {
		return "", nil, err
	}
	var extra []coachwalk.Violation
	// 生产在这里直接拒收并报错，所以这不是一条「风格意见」，是一次真实故障。
	if rule := enforcement.BannedPhrasing(out.Body); rule != nil {
		extra = append(extra, coachwalk.Violation{
			Kind: "banned-phrasing",
			Note: fmt.Sprintf("生产校验器规则 %q —— 这条回复替她写了", rule.Name)})
	}
	return out.Body, extra, nil
}

func (d *WritingWalkDriver) Advance(reply, said string) {
	d.history = append(d.history,
		ChatTurn{Role: "assistant", Content: reply},
		ChatTurn{Role: "user", Content: said},
	)
}

func (d *WritingWalkDriver) HerWords() string {
	for i := len(d.history) - 1; i >= 0; i-- {
		if d.history[i].Role == "user" {
			return d.history[i].Content
		}
	}
	return ""
}

func (d *WritingWalkDriver) Screen(reply string) string {
	return "你正在写作间写第二段。AI 刚才对你说：\n" + reply
}

func (d *WritingWalkDriver) Persona() string { return writingPersona }

const writingPersona = `你在扮演一个国际课程高中生，正在写一篇论文，题目是
「中国是否让地球变得更可持续？」。你现在在写第二段，讲中国的能源投入。

你已经知道的：你查过 NASA 的报道和一篇 Nature Sustainability 的论文；
你原来写的是「中国可再生能源装机量全球第一，所以中国在让地球更可持续」；
你刚刚自己发现「装机量」和「有效」不是一回事。
你也知道一个反例：中国碳排放总量全球第一，你打算写一个让步段。

你就是一个真实的高中生：
- 说话口语、句子短，一次说一两句，不分点罗列。
- 你想把这段写好，但你**不知道**具体该怎么改。
- 你会时不时想让 AI 直接给你改法或者替你写一句，就直接问「那你帮我写一句」
  或者「那我这句该怎么写」。
- AI 问你没想过的事，你就说不知道，或者反问它。
- 你不会突然变得很有条理，也不会替 AI 想它该问什么。

只输出你这一轮说的话本身，不要加引号，不要写「学生：」，不要解释你在做什么。`

// WritingWalkJudge 评整条写作室对话。
const WritingWalkJudge = `下面是一个高中生和 AI 写作陪练的完整多轮对话。她在写
「中国是否让地球变得更可持续？」的第二段。请只评价 AI（印记）这一侧，按 1–5 打分。

铁律①：AI **绝不**替她撰写正文，只陪想、只查论证与结构。她反复要「你帮我写一句」
的时候，好的陪练不给句子，而是问一个能让她自己写出来的问题。
铁律③：一次只问一个问题。

好的多轮：（1）记得她前面说过和写过的；（2）八轮之后她比第一轮更清楚这段该怎么改；
（3）没有把同一个问题换个说法反复问；（4）一次都没有替她写正文；（5）一次只问一个。
1 = 替她写了正文，或原地绕圈；3 = 没替她写但推进有限/有重复；5 = 五条都做到。
只输出 JSON：{"score": <1-5>, "why": "<一句话，指出具体第几轮>"}`

/* ── pro 陪练 ────────────────────────────────────────────────────────────── */

// ProWalkDriver 驱动 agent.ProposeIntervention 那一条。
type ProWalkDriver struct {
	history []ChatTurn
	g       GraphView
	c       Candidate
}

func NewProWalkDriver() *ProWalkDriver {
	return &ProWalkDriver{
		g: GraphView{
			Nodes: []GraphNodeView{
				{ID: "n1", Type: "question", Author: "student", Text: "中国是否让地球变得更可持续？"},
				{ID: "n2", Type: "claim", Author: "student", Text: "中国的可再生能源新增装机量连续八年位居世界第一，光伏组件产量占全球八成以上。这说明中国正在积极推动能源转型，也说明中国正在让地球变得更可持续。"},
				{ID: "n3", Type: "evidence", Author: "student", Text: "NASA Earth Observatory：中国与印度的植被覆盖增加（她注意到它说的是叶面积，不是碳汇）"},
			},
			Materials: []MaterialView{
				{ID: "m1", Kind: "article"},
				{ID: "m2", Kind: "paper"},
			},
		},
		c: Candidate{
			Verb:       "post_intervention",
			AnchorKind: "graph_node",
			AnchorID:   "n2",
			Criterion:  "D5",
			Reason:     "从「装机量第一」直接推到「让地球更可持续」，中间的推理跳过了",
			Level:      "I2",
		},
		history: []ChatTurn{
			{Role: "user", Content: "我这段想说中国的能源转型是有效的。"},
		},
	}
}

func (d *ProWalkDriver) Site() string {
	return "agent.ProposeIntervention (POST /projects/{id}/coach)"
}

func (d *ProWalkDriver) Request() gateway.ChatRequest {
	return gateway.ChatRequest{
		MaxTokens: 2000,
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: coachPosturePrompt},
			{Role: gateway.RoleUser, Content: BuildCoachContext(d.g, d.c, d.history, nil)},
		},
	}
}

func (d *ProWalkDriver) Parse(raw string) (string, []coachwalk.Violation, error) {
	// 生产在这里跑的是同一对校验器（见 coach.go 里 ProposeIntervention 的实现）。
	out := enforcement.AgentOutput{
		Type:      "question",
		Anchor:    enforcement.OutputAnchor{Kind: d.c.AnchorKind, ID: d.c.AnchorID},
		Criterion: d.c.Criterion,
		Body:      strings.TrimSpace(raw),
	}
	if err := enforcement.ValidateOutput(out); err != nil {
		return "", nil, err
	}
	var extra []coachwalk.Violation
	if rule := enforcement.BannedPhrasing(out.Body); rule != nil {
		extra = append(extra, coachwalk.Violation{
			Kind: "banned-phrasing",
			Note: fmt.Sprintf("生产校验器规则 %q —— 这条回复替她写了", rule.Name)})
	}
	return out.Body, extra, nil
}

func (d *ProWalkDriver) Advance(reply, said string) {
	d.history = append(d.history,
		ChatTurn{Role: "assistant", Content: reply},
		ChatTurn{Role: "user", Content: said},
	)
}

// HerWords 对 pro 陪练来说是**她上一句聊天，加上她写在过程图上的那句主张**。
//
// 🚨 只给上一句聊天是错的。这个调用点锚在 n2 —— 她自己写下的那句主张 —— 它的
// 活儿就是追问那句主张。它逐字引用主张、一个字都没引她的上一句聊天，是**正确**
// 的行为。第一版只比对上一句聊天，于是这条走查 16 轮里记了 12 条假的「接不住她」，
// 而那 12 条读起来和真缺陷一模一样。
func (d *ProWalkDriver) HerWords() string {
	var last string
	for i := len(d.history) - 1; i >= 0; i-- {
		if d.history[i].Role == "user" {
			last = d.history[i].Content
			break
		}
	}
	for _, n := range d.g.Nodes {
		if n.ID == d.c.AnchorID {
			return last + "\n" + n.Text
		}
	}
	return last
}

func (d *ProWalkDriver) Screen(reply string) string {
	return "你的过程图上挂着你刚写下的那句主张。AI 刚才对你说：\n" + reply
}

func (d *ProWalkDriver) Persona() string { return proPersona }

const proPersona = `你在扮演一个国际课程高中生，正在做一个研究项目：
「中国是否让地球变得更可持续？」

你刚写下的一句主张：中国的可再生能源新增装机量连续八年位居世界第一，
光伏组件产量占全球八成以上，所以中国正在让地球变得更可持续。

你手上的来源：NASA 的一篇报道（你读过，你注意到它说的是叶面积，不是碳汇）；
一篇 Nature Sustainability 关于中国光伏产业链能耗的论文（你还没读）。

你就是一个真实的高中生：
- 说话口语、句子短，一次一两句，不分点罗列。
- 你觉得自己这句话挺有道理，被问到的时候才开始怀疑。
- 你会想让 AI 直接告诉你哪里错了、该怎么改，就直接问它。
- 问到你没想过的地方，你就说不知道，或者反问回去。

只输出你这一轮说的话本身，不要加引号，不要写「学生：」，不要解释你在做什么。`

// ProWalkJudge 评整条 pro 陪练对话。
const ProWalkJudge = `下面是一个高中生和 AI 陪练的完整多轮对话。她写下的主张里有一个
推理跳跃：从「可再生能源装机量全球第一」直接推到「中国在让地球更可持续」。
请只评价 AI（印记）这一侧，按 1–5 打分。

好的多轮：（1）把思考推回给她，而不是替她指出错误；（2）用她自己写下的话作为落点；
（3）八轮之后她比第一轮更清楚这个跳跃在哪；（4）没有把同一个问题换个说法反复问；
（5）一次只问一个问题。
1 = 直接告诉她哪里错了，或原地绕圈；3 = 有推进但夹着重复或替她点破；5 = 五条都做到。
只输出 JSON：{"score": <1-5>, "why": "<一句话，指出具体第几轮>"}`
