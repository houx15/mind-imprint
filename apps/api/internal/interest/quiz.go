package interest

import (
	"fmt"
	"strings"

	"mindimprint/api/internal/disciplines"
	"mindimprint/api/internal/interests"
)

// quiz.go —— 觉醒协议（兴趣测试）的纯逻辑。
//
// # 它为什么存在
//
// 采集器只能从她**做完的事**里长词（internal/api/interest_harvest.go）。一个刚
// 注册的学生什么都还没做完，所以她的树是空的 —— 而一棵空树说不出「这就是你的
// 模型」，它只能说「你还没开始」。测试是冷启动：用五分钟换第一批词。
//
// # 它写回的是同一条路
//
// 测试**不另建一套关键词**。它的产物走 plantKeywords 的同一条路（kind="quiz"），
// 用同一个采集 prompt、同一个解析器、同一个三档路由。这不是省事，是正确：如果
// 测试有自己的一套词，树上就会有两种来路不同、可信度不同的词，而学生看不出区别。
//
// # 重做是「再长几个词」
//
// 每一次作答是一行 interest_quiz，ref_id 用那一行的 id，于是重做会给同一个词
// **再添一条来源**（强度上升），而不是清空重来。半年后重做一次，看到的是树长
// 高了，不是树被换掉了。
//
// # 它绝不编词
//
// 她如果只写了「很帅」，那就是没有可摘的原话，这次就该长出**零个词** ——
// 解析器会把 evidence 太短的词全部丢掉，而结果页必须诚实地说这件事，然后请她
// 多写一句。编一个「你关心角色的成长弧」挂上去，是她无从反驳的假话
// （memory: ai-errors-must-surface-never-fake）。

/* ── 兴趣钩子 ───────────────────────────────────────────────────────────── */

// Hook 是她在第二关挑的那个问题 —— 「哪一种问题会让你愿意继续追下去」。
//
// 三个而不是七个：这一屏要在十秒内选完，而七根主枝的完整清单是**结果**该给她
// 看的东西，不是入口该让她面对的东西。
type Hook string

const (
	// HookCharacter 为什么这个人会变成现在这样？（创伤 · 选择 · 关系 · 命运）
	HookCharacter Hook = "character"
	// HookCraft 这么厉害的画面或机制是怎么做出来的？（运动 · 镜头 · 结构 · 技术）
	HookCraft Hook = "craft"
	// HookSociety 为什么大家会这样评价它？（流行 · 舆论 · 身份 · 偏见）
	HookSociety Hook = "society"
)

// hookLenses 把一个钩子连到**真的学科**上。
//
// 🚨 这里写的是 disciplines.json 的 id，不是原型里那三串手写的字符串
// （「心理学 / 文学」）。差别是实打实的：真 id 带着 asks / method / exemplar 和
// **考纲投影**，所以结果页能对她说「这在 IB 心理学 HL 里叫发展心理学」，而不是
// 只丢一个学科名字。`TestHookLensesAreRealDisciplines` 守着这条线。
//
// 每个钩子给两到三片透镜，跨主枝：一个钩子只连一根枝，等于在她刚说完自己喜欢
// 什么的那一刻就把她框住了。
var hookLenses = map[Hook][]string{
	HookCharacter: {"developmental-psychology", "literary-criticism", "film-narrative"},
	HookCraft:     {"engineering-design", "film-narrative", "matter-energy"},
	HookSociety:   {"sociology", "media-literacy", "anthropology"},
}

// IsHook 说这个字符串是不是三个钩子之一。
func IsHook(s string) bool {
	_, ok := hookLenses[Hook(s)]
	return ok
}

// HookLenses 返回这个钩子对应的学科（按 hookLenses 的顺序）。
//
// 未知钩子返回 nil，而不是一个默认列表 —— 给一个没选钩子的作答配上三片学科，
// 就是替她做了她没做的选择。
func HookLenses(h Hook) []disciplines.Discipline {
	ids := hookLenses[h]
	if len(ids) == 0 {
		return nil
	}
	out := make([]disciplines.Discipline, 0, len(ids))
	for _, id := range ids {
		if d, ok := disciplines.ByID(id); ok {
			out = append(out, d)
		}
	}
	return out
}

/* ── 一次作答 ───────────────────────────────────────────────────────────── */

// minReasonRunes 是「值得为它花一次模型调用」的最短原话。
//
// 八个字符。低于这个长度（「很帅」「好看」「喜欢」），里面没有可摘的原话，
// 采集器要么空手而归、要么开始编。**与其花掉那次调用再把结果丢掉，不如根本
// 不发** —— 结果页会请她多写一句，那比一个空结果诚实，也比一个编出来的结果
// 诚实得多。
const minReasonRunes = 8

// maxWorkRunes / maxReasonRunes 与前端的 maxlength 对齐。服务端仍然自己截断：
// 前端的长度限制是体验，不是约束。
const (
	maxWorkRunes   = 40
	maxReasonRunes = 180
)

// Attempt 是她提交上来的一次作答。
type Attempt struct {
	Navigator string
	// Work 是那部作品或那个角色。
	Work string
	// Reason 是**她自己写的那段**：它最吸引她的地方。树上的 evidence 摘自这里。
	Reason string
	Hook   Hook
	// ChallengeChoice 是她最后选的那个回应；ChallengeAttempts 是在此之前错了
	// 几次。两个都记下来，因为**摩擦是信号不是噪音**（铁律④）。
	ChallengeChoice   string
	ChallengeAttempts int
}

// Clean 把一次作答收拾成可以落库的样子：去空白、按 rune 截断、丢掉不认识的钩子。
//
// 它**不拒绝**一次作答。一个中途乱填的作答仍然是数据（她走到哪一屏、她填了
// 什么），拒绝它只会让我们对她少知道一点。真正的门槛在 ShouldHarvest 那里，
// 而那道门槛只决定「要不要花那次模型调用」。
func (a Attempt) Clean() Attempt {
	a.Navigator = truncRunes(strings.TrimSpace(a.Navigator), 40)
	a.Work = truncRunes(strings.TrimSpace(a.Work), maxWorkRunes)
	a.Reason = truncRunes(strings.TrimSpace(a.Reason), maxReasonRunes)
	if !IsHook(string(a.Hook)) {
		a.Hook = ""
	}
	a.ChallengeChoice = truncRunes(strings.TrimSpace(a.ChallengeChoice), 40)
	if a.ChallengeAttempts < 0 {
		a.ChallengeAttempts = 0
	}
	return a
}

// ShouldHarvest 说这次作答值不值得为它发一次采集调用。
//
// 只看 Reason 的长度，不看别的：作品名（「进击的巨人」）本身不是兴趣信号 ——
// 千万人喜欢同一部作品，理由各不相同，而**理由才是那个人**。
func (a Attempt) ShouldHarvest() bool {
	return len([]rune(a.Reason)) >= minReasonRunes
}

// BuildQuizPrompt 拼出这次作答的采集 prompt。
//
// 复用采集的 system prompt：同一张候选词表、同一套字段要求、同一个解析器
// （ParseHarvestReply）。测试如果有自己的一套 prompt，两边对「什么算一个好
// 关键词」的看法迟早会分叉，而分叉的结果是同一棵树上挂着两种质量的词。
func (a Attempt) BuildQuizPrompt() (system, user string) {
	// 🚨 把「她写的」和「我写的」分得死死的。实测过一次失败：脚手架那一行
	// （原来写作「她说她喜欢的是：X」）被模型当成她的原话摘了回来。围栏 + 明确
	// 指令降低这件事发生的概率，而 KeepGrounded 是真正兜住它的那道闸。
	var b strings.Builder
	b.WriteString("以下围栏内是她**自己输入**的全部文字，evidence 只能从围栏内逐字摘录：\n")
	b.WriteString("<<<她写的\n")
	fmt.Fprintf(&b, "%s\n", a.Work)
	fmt.Fprintf(&b, "%s\n", a.Reason)
	b.WriteString("她写的>>>\n")
	if lenses := HookLenses(a.Hook); len(lenses) > 0 {
		names := make([]string, 0, len(lenses))
		for _, d := range lenses {
			names = append(names, d.Zh)
		}
		// 钩子作为**线索**给模型，不是指令：她挑的是「哪种问题想追下去」，
		// 这能帮模型判断同一段话里哪一层才是她真正的兴趣。但词仍然必须从她
		// 的原话里长出来，所以这里写「倾向」而不是「必须」。
		fmt.Fprintf(&b, "\n她挑的追问方向偏向：%s。\n", strings.Join(names, " · "))
	}
	return harvestSystemPromptHead + interests.PromptList() + harvestSystemPromptTail, b.String()
}

// QuizSourceLabel 是这条来源在抽屉里显示的名字。
const QuizSourceLabel = "觉醒协议 · 兴趣测试"

// OwnWords 是她在这次作答里**自己敲进去的全部文字**，用作 KeepGrounded 的
// 比对语料。选项不算：那些是我写的句子，她只是点了一下。
func (a Attempt) OwnWords() string {
	return a.Work + "\n" + a.Reason
}

func truncRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}
