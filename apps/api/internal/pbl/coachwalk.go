package pbl

import (
	"context"
	"fmt"
	"strings"
	"unicode"

	"mindimprint/api/internal/gateway"
)

// ── 多轮陪练走查 ────────────────────────────────────────────────────────────
//
// routebench 测的是**一轮**：给定一个上文，这个模型这一轮答得好不好。
// 但「换了模型，陪练还能用吗」不是一轮的性质。它问的是八轮之后印记还记不记得
// 她说过的话、有没有把同一个问题换个说法再问一遍、她跳过一步的时候有没有放她
// 过去。一轮的分数看不见这三件事——一个模型可以每一轮都单独好看，连起来读却是
// 在原地绕。
//
// 所以这里再跑一条：让另一家的模型演学生，和真的 coachPrompt / parseCoachOutput
// 来回八轮，然后**按可验的判据**数它犯了几次规，而不是请判官给个印象分。
// 「提示词里的规矩要能在代码里验」这条，2026-09-03 和 2026-09-12 各付过一次学费。
//
// 它和 benchcases.go 一样住在 pbl 包里，理由也一样：coachPrompt 和
// parseCoachOutput 是未导出的，抄一份出去就会漂移，而拿漂移的 prompt 测出来的
// 结论读起来像证据，其实一文不值。请求路径上没有任何东西调用这里。

// WalkCall 打一次模型并把正文交回来。延迟和 token 由调用方在闭包里量——
// 这样 pbl 包不需要认识 provider、catalog 或任何一件工具侧的东西。
type WalkCall func(ctx context.Context, req gateway.ChatRequest) (string, error)

// WalkTurn 是一轮来回，以及这一轮能当场验出来的几件事。
type WalkTurn struct {
	N int
	// CoachRaw 是模型回的原文，解析之前。解析失败时唯一能看的就是它。
	CoachRaw string
	Reply    string
	Hook     string
	Tool     string
	ParseErr string

	// StudentSaid 是演学生的那个模型这一轮说的话。
	StudentSaid string

	// QuestionsInReply 是 reply 里的问号个数。铁律③：一次只问一个问题。
	// 两个以上就是犯规，这一条不需要判官。
	QuestionsInReply int
	// HasHook 记一句挂出去的追问。reply 里已经问了一个、又挂一个钩子，
	// 是「三个问题穿了件外套」那件事的轻量版（见 CoachOutput.Hook 的注释）。
	HasHook bool

	// RepeatOf 不为 0 时，这一轮的 reply 和第 RepeatOf 轮高度重合——
	// 印记在换个说法重复自己。
	RepeatOf int
	// EchoesHer 为 false 时，这一轮的 reply 里找不到她上一句话的任何一段原文。
	// 这是「接住她已经说过的具体内容」的可验版本：不是要求逐字引用，而是
	// 八轮里一次都接不上，就说明它在自己说自己的。
	EchoesHer bool
}

// WalkLog 是一整条走查。
type WalkLog struct {
	Turns []WalkTurn
}

// Violation 是一条数得出来的犯规。
type Violation struct {
	Turn int
	Kind string
	Note string
}

// Violations 把整条走查里数得出来的问题列出来，按轮次排。
//
// 这里**只放不需要判断的东西**。「这个问题问得好不好」不在里面，那个确实要人
// 或判官去看；「一轮问了三个问题」「第六轮把第三轮的话又说了一遍」不需要。
func (w *WalkLog) Violations() []Violation {
	var vs []Violation
	for _, t := range w.Turns {
		if t.ParseErr != "" {
			vs = append(vs, Violation{t.N, "parse", t.ParseErr})
			continue
		}
		if t.QuestionsInReply >= 2 {
			vs = append(vs, Violation{t.N, "multi-question",
				fmt.Sprintf("reply 里有 %d 个问号，铁律③ 是一次只问一个", t.QuestionsInReply)})
		}
		if t.QuestionsInReply >= 1 && t.HasHook {
			vs = append(vs, Violation{t.N, "question+hook",
				"reply 里问了一个，又另外挂了一个钩子"})
		}
		if t.RepeatOf != 0 {
			vs = append(vs, Violation{t.N, "repeat",
				fmt.Sprintf("和第 %d 轮高度重合", t.RepeatOf)})
		}
		if !t.EchoesHer {
			vs = append(vs, Violation{t.N, "ungrounded",
				"reply 里找不到她上一句的任何一段原文"})
		}
	}
	return vs
}

// Transcript 把整条对话渲染成人和判官都读得懂的样子。
func (w *WalkLog) Transcript() string {
	var b strings.Builder
	for _, t := range w.Turns {
		fmt.Fprintf(&b, "── 第 %d 轮 ──\n", t.N)
		if t.ParseErr != "" {
			fmt.Fprintf(&b, "印记（解析失败：%s）：%s\n\n", t.ParseErr, truncateRunes(t.CoachRaw, 300))
			continue
		}
		fmt.Fprintf(&b, "印记：%s\n", t.Reply)
		if t.Hook != "" {
			fmt.Fprintf(&b, "　　［追问钩子］%s\n", t.Hook)
		}
		if t.Tool != "" {
			fmt.Fprintf(&b, "　　［递出工具］%s\n", t.Tool)
		}
		fmt.Fprintf(&b, "学生：%s\n\n", t.StudentSaid)
	}
	return b.String()
}

// studentSystem 是演学生的那个模型拿到的全部人设。
//
// 🚨 她只能看到学生看得见的东西：印记说的那句话、挂出来的钩子、递过来的工具名。
// 不给她 JSON、不给她 coachPrompt、不告诉她这个项目该走几步。她看不懂就说看不懂，
// 那句「看不懂」就是我们要的结果——这条和 e2e/camp 的 brain.ts 是同一个道理。
const studentSystem = `你在扮演一个中国初中二年级的学生，正在做一个学校里的项目式学习任务。

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

// runStudent 让演学生的模型看一眼屏幕，然后说一句话。
func runStudent(ctx context.Context, call WalkCall, t WalkTurn, history string) (string, error) {
	var screen strings.Builder
	screen.WriteString("AI 刚才对你说：\n")
	screen.WriteString(t.Reply)
	if t.Hook != "" {
		screen.WriteString("\n\n屏幕上还有一个可以点的追问：「" + t.Hook + "」")
	}
	if t.Tool != "" {
		screen.WriteString("\n\n屏幕上还出现了一个工具按钮：「" + t.Tool + "」")
	}
	if history != "" {
		screen.WriteString("\n\n你们之前聊过的（供你记得住，别复述）：\n" + history)
	}
	screen.WriteString("\n\n你这一轮说什么？")

	return call(ctx, gateway.ChatRequest{
		MaxTokens: 800,
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: studentSystem},
			{Role: gateway.RoleUser, Content: screen.String()},
		},
	})
}

// RunCoachWalk 用真的 coachPrompt 和真的 parseCoachOutput 跑 turns 轮。
//
// 起点是 benchTurnInput()——和 routebench 那一格同一个上文，这样一轮的分数和多轮
// 的走查说的是同一个学生、同一个项目。
func RunCoachWalk(ctx context.Context, coach, student WalkCall, turns int) (*WalkLog, error) {
	in := benchTurnInput()
	log := &WalkLog{}

	for n := 1; n <= turns; n++ {
		raw, err := coach(ctx, gateway.ChatRequest{
			MaxTokens: 16384,
			Messages: []gateway.ChatMessage{
				{Role: gateway.RoleSystem, Content: coachPrompt(in.Kind)},
				{Role: gateway.RoleUser, Content: buildCoachContext(in)},
			},
		})
		if err != nil {
			return log, fmt.Errorf("第 %d 轮陪练调用失败: %w", n, err)
		}

		t := WalkTurn{N: n, CoachRaw: raw}
		out, perr := parseCoachOutput(raw)
		if perr != nil {
			// 解析失败在生产里就是给学生弹一个错。记下来，停在这里——
			// 后面几轮拿不到 reply，续下去只是在编一条不存在的对话。
			t.ParseErr = perr.Error()
			log.Turns = append(log.Turns, t)
			return log, nil
		}
		t.Reply, t.Hook, t.Tool = out.Reply, out.Hook, out.Tool
		t.QuestionsInReply = countQuestions(out.Reply)
		t.HasHook = out.Hook != ""
		t.EchoesHer = echoesLastStudent(out.Reply, in.Recent)
		t.RepeatOf = repeatOf(out.Reply, log.Turns)

		// 学生看一眼屏幕，说一句。
		said, serr := runStudent(ctx, student, t, transcriptTail(in.Recent))
		if serr != nil {
			return log, fmt.Errorf("第 %d 轮学生调用失败: %w", n, serr)
		}
		t.StudentSaid = strings.TrimSpace(said)
		log.Turns = append(log.Turns, t)

		// 把这一轮接到上文里，下一轮印记看到的就是真的往前走了一步的对话。
		in.Recent = append(in.Recent,
			Turn{Role: "ai", Content: out.Reply},
			Turn{Role: "student", Content: t.StudentSaid},
		)
	}
	return log, nil
}

/* ── 可验的判据 ──────────────────────────────────────────────────────────── */

// countQuestions 数问号，中英文都算。
func countQuestions(s string) int {
	return strings.Count(s, "？") + strings.Count(s, "?")
}

// echoesLastStudent 检查 reply 里有没有她上一句话的任何一段原文。
//
// 判据故意宽：连续四个字重合就算接住了。要抓的不是「没有逐字引用」，是
// 「八轮下来一次都接不上她说的话」那种自说自话。四个字以下不判——
// 「好的」「嗯」这种太短的话，任何回复都会偶然命中。
func echoesLastStudent(reply string, recent []Turn) bool {
	var last string
	for i := len(recent) - 1; i >= 0; i-- {
		if recent[i].Role == "student" {
			last = recent[i].Content
			break
		}
	}
	hers := []rune(foldWalkSpace(last))
	flat := foldWalkSpace(reply)
	const win = 4
	if len(hers) < win {
		return true
	}
	for i := 0; i+win <= len(hers); i++ {
		if strings.Contains(flat, string(hers[i:i+win])) {
			return true
		}
	}
	return false
}

// repeatOf 找出这一句和前面哪一轮高度重合，没有就返回 0。
//
// 用二元组的 Jaccard，不是字符串相等：印记重复自己的时候会换个说法，
// 而换了说法的同一个问题，对学生来说仍然是同一个问题又被问了一遍。
//
// 阈值是量出来的，不是定出来的。同一个问题换句话说落在 0.58，
// 一个真正的新问题落在 0.00——中间这段空得很宽，所以取 0.45：
// 比真重复低一截，比任何一个新问题高得多。
// 第一版取 0.6，正好卡在真重复的上面一点，于是一次重复都抓不到。
const repeatThreshold = 0.45

func repeatOf(reply string, prev []WalkTurn) int {
	cur := bigrams(reply)
	if len(cur) == 0 {
		return 0
	}
	for _, p := range prev {
		if p.ParseErr != "" {
			continue
		}
		if jaccard(cur, bigrams(p.Reply)) >= repeatThreshold {
			return p.N
		}
	}
	return 0
}

func bigrams(s string) map[string]struct{} {
	r := []rune(foldWalkSpace(s))
	out := make(map[string]struct{}, len(r))
	for i := 0; i+2 <= len(r); i++ {
		out[string(r[i:i+2])] = struct{}{}
	}
	return out
}

func jaccard(a, b map[string]struct{}) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	inter := 0
	for k := range a {
		if _, ok := b[k]; ok {
			inter++
		}
	}
	union := len(a) + len(b) - inter
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}

// foldWalkSpace 只留下字、数字和字母，空白和标点全部丢掉。
//
// 🚨 丢标点不是顺手做的。第一版只折空白，于是这一轮被判成「接不住她」：
//
//	她说：…他们是因为打多了还是不好吃才剩的？
//	印记：先别急着分「打多了」还是「不好吃」——…
//
// 印记接住的是她的原话，一字不差，可它加了一对引号，四字窗口
// 「打多了还」就再也对不上了。**判据错了就会把一个走通了的陪练记成走不通的**，
// 而那条假缺陷读起来和真的一模一样（2026-09-04、09-12 各栽过一次）。
func foldWalkSpace(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// transcriptTail 给演学生的模型一段简短的记忆，避免她每轮都像刚进门。
func transcriptTail(recent []Turn) string {
	if len(recent) > 6 {
		recent = recent[len(recent)-6:]
	}
	var b strings.Builder
	for _, t := range recent {
		who := "AI"
		if t.Role == "student" {
			who = "你"
		}
		fmt.Fprintf(&b, "%s：%s\n", who, truncateRunes(t.Content, 120))
	}
	return b.String()
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

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
