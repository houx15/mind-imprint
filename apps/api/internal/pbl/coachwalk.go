package pbl

import (
	"mindimprint/api/internal/coachwalk"
	"mindimprint/api/internal/gateway"
)

// WalkDriver 是 PBL 陪练在多轮走查里的那一侧。
//
// 它住在这个包里，和 benchcases.go 同一个理由：coachPrompt 和 parseCoachOutput
// 是未导出的，抄一份出去就会漂移。请求路径上没有任何东西调用它。
type WalkDriver struct {
	in                 CoachInput
	lastHook, lastTool string
}

// NewWalkDriver 从 benchTurnInput() 起步——和 routebench 那一格同一个上文，
// 这样一轮的分数和多轮的走查说的是同一个学生、同一个项目。
func NewWalkDriver() *WalkDriver { return &WalkDriver{in: benchTurnInput()} }

func (d *WalkDriver) Site() string { return "internal/pbl.Coach (POST /pbl/projects/{id}/turn)" }

func (d *WalkDriver) Request() gateway.ChatRequest {
	return gateway.ChatRequest{
		MaxTokens: 16384,
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: coachPrompt(d.in.Kind)},
			{Role: gateway.RoleUser, Content: buildCoachContext(d.in)},
		},
	}
}

func (d *WalkDriver) Parse(raw string) (string, []coachwalk.Violation, error) {
	out, err := parseCoachOutput(raw)
	if err != nil {
		return "", nil, err
	}
	var extra []coachwalk.Violation
	// reply 里已经问了一个、又挂一个钩子，是「三个问题穿了件外套」那件事的
	// 轻量版（见 CoachOutput.Hook 的注释）。
	if out.Hook != "" && coachwalk.CountQuestions(out.Reply) >= 1 {
		extra = append(extra, coachwalk.Violation{
			Kind: "question+hook", Note: "reply 里问了一个，又另外挂了一个钩子"})
	}
	d.lastHook, d.lastTool = out.Hook, out.Tool
	return out.Reply, extra, nil
}

func (d *WalkDriver) Advance(reply, said string) {
	d.in.Recent = append(d.in.Recent,
		Turn{Role: "ai", Content: reply},
		Turn{Role: "student", Content: said},
	)
}

func (d *WalkDriver) HerWords() string {
	for i := len(d.in.Recent) - 1; i >= 0; i-- {
		if d.in.Recent[i].Role == "student" {
			return d.in.Recent[i].Content
		}
	}
	return ""
}

func (d *WalkDriver) Screen(reply string) string {
	s := "AI 刚才对你说：\n" + reply
	if d.lastHook != "" {
		s += "\n\n屏幕上还有一个可以点的追问：「" + d.lastHook + "」"
	}
	if d.lastTool != "" {
		s += "\n\n屏幕上还出现了一个工具按钮：「" + d.lastTool + "」"
	}
	return s
}

func (d *WalkDriver) Persona() string { return pblPersona }

const pblPersona = `你在扮演一个中国初中二年级的学生，正在做一个学校里的项目式学习任务。

你的项目：搞清楚学校食堂每天浪费多少饭菜，然后想办法让它少一点。
你已经做过的事：连着三天午饭后称了剩饭，平均每天大概 47 公斤；
在便签板上把「浪费」分成了三类——没打完的、打多了的、不好吃剩下的。

你就是一个真实的初二学生：
- 说话短，口语，不用书面词，不分点罗列。一次只说一两句。
- 你**没有**想清楚接下来该干什么，这正是你在跟 AI 聊的原因。
- 别人问得含糊，你就答得含糊；问到你没想过的地方，你就说「不知道」或者反问回去。
- 有时候你会偷懒，想让 AI 直接告诉你答案，就直接问它「那我该怎么做」。
- 你不会替 AI 想它该问什么，也不会突然变得很有条理。

只输出你这一轮说的话本身，不要加引号，不要写「学生：」，不要解释你在做什么。`

// WalkJudge 是整条对话的评分标准。它问的全是**多轮**才看得见的事——
// 一轮一轮单独打分的那张表回答不了这些。
const WalkJudge = `下面是一个初中生和 AI 陪练关于「校园食堂剩饭」项目的完整多轮对话。
请只评价 AI（印记）这一侧，按 1–5 打分。好的陪练应当：
（1）记得她前面说过的话，后面的问题建立在前面的回答上，而不是每轮重新开始；
（2）真的把项目往前推了一步——八轮之后她比第一轮更清楚下一步做什么；
（3）没有把同一个问题换个说法反复问；
（4）她想偷懒、直接要答案的时候，没有直接把答案给她，也没有就这么放她过去；
（5）一次只问一个问题。
1 = 原地绕圈或直接代她做完；3 = 有推进但夹着重复或泛泛的问题；5 = 五条都做到。
只输出 JSON：{"score": <1-5>, "why": "<一句话，指出具体第几轮>"}`
