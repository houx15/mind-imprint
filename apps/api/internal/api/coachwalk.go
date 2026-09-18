package api

import (
	"fmt"
	"strings"

	"github.com/google/uuid"

	"mindimprint/api/internal/coachwalk"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/store/sqlc"
)

// fixtureTaskID 给走查和 routebench 的阅读用例一个稳定、互不相同的步骤 id。
//
// 🚨 这两份用例原来把步骤状态写成 "active"/"todo"，而且没有 id。生产的
// currentReadingTask 只认 "pending"，于是 prompt 里**没有当前步**，模型读到的是
// 「所有步骤都走完了。跟她说一句收尾的话，别再领新的一步。」—— 它第一轮就收尾，
// 把答案讲完、说「今天带到这儿」。routebench 上五个候选在这一格全判 1–2 分、
// coachwalk 上两个模型六条走查全判 1 分，测的都是这份坏掉的用例，不是陪练。
// 状态一改成 "pending"，所有 id 又都是零值，「她现在在这一步」会标在每一行上，
// 所以 id 也必须互不相同。TestReadingFixturesHaveACurrentStep 守着这两件事。
func fixtureTaskID(n byte) uuid.UUID { return uuid.UUID{15: n} }

// ReadingWalkDriver 驱动阅读室陪练那一条（postReadingCoachTurn）。
//
// 它住在这个包里，和 benchcases.go 同一个理由：buildReadingCoachSystem /
// buildReadingCoachPrompt / parseReadingCoachReply 都是未导出的。
// 请求路径上没有任何东西调用它。
//
// 🚨 这一条走查有一件别的陪练没有的可验判据：`advance`。
// 当前这一步要她「找出最关键的那个数字，说说它衡量的是什么」。她说对了数字
// （装机量第一），但**没有回答「它衡量的是什么」**——装机容量衡量的是发电能力，
// 不等于发电量，更不等于被替代掉的化石燃料。所以这一步还没做完，
// 而模型在她真的答上来之前把 advance 设成 done，就是**放她过去**。
// 那是这个调用点存在的理由本身，也是一条数得出来的判断错误，不需要判官。
type ReadingWalkDriver struct {
	blocks []Block
	lang   string
	tasks  []sqlc.ReadingTask
	msgs   []sqlc.AtomMessage
	picks  []readingPick
	// student 是这一轮交给 prompt 的「她刚说的话」。
	student string
	// answered 记她到底有没有回答过「它衡量的是什么」。走查开始时是 false，
	// 只有她自己说出那个区别才会翻成 true。
	answered bool
	seq      int32
}

func NewReadingWalkDriver() *ReadingWalkDriver {
	blocks := SplitBlocks(benchReadingArticle)
	return &ReadingWalkDriver{
		blocks: blocks,
		lang:   readingLangOf(benchReadingArticle),
		tasks: []sqlc.ReadingTask{
			{ID: fixtureTaskID(1), Position: 1, Kind: "read", Label: "通读全文，说说作者到底在主张什么", BlockID: "", Status: "done"},
			// 🚨 kind 只能从生产的那张闭表里抄（迁移 0180 的 CHECK）。
			// 这两行原来写的是 "locate" 和 "question" —— 两个**不存在的** kind，
			// 于是 readingCurrentStepInstruction 落回默认那条判据，这条走查量的
			// 是一种线上根本不会出现的步骤。和 2026-09-14 那份把状态写成
			// "active"/"todo" 的阅读用例是同一个毛病（见
			// [[fixture-told-coach-session-over-2026-09-14]]）。
			{ID: fixtureTaskID(2), Position: 2, Kind: string(taskFocusBlock), Label: "精读重点段落第3段：找出最关键的那个数字，说说它衡量的是什么", BlockID: "b3", Status: "pending"},
			{ID: fixtureTaskID(3), Position: 3, Kind: string(taskCritique), Label: "你怎么看", BlockID: "", Status: "pending"},
		},
		msgs: []sqlc.AtomMessage{
			{Seq: 1, Role: "assistant", Content: "先通读一遍。读完告诉我，作者到底想让你接受什么？"},
			{Seq: 2, Role: "user", Content: "他想说中国在可再生能源上投了很多钱。"},
			{Seq: 3, Role: "assistant", Content: "投了很多钱——这是他想让你接受的结论，还是他用来支持结论的证据？"},
			{Seq: 4, Role: "user", Content: "……是证据吧。结论应该是中国在让地球变得更可持续。"},
		},
		picks:   []readingPick{{BlockID: "b3", Quote: "中国的可再生能源新增装机量连续八年位居世界第一。"}},
		student: "我觉得这句最关键，装机量连续八年第一，说明投入是真的很大。",
		seq:     4,
	}
}

// ReadingWalkScenarios is the lite-reading-coach multi-turn half of the same
// suite whose single turns live in benchcases.go. Keeping the registration by
// the unexported prompt builders prevents fixtures from drifting into copies.
func ReadingWalkScenarios() []coachwalk.Scenario {
	return []coachwalk.Scenario{
		{
			Suite: liteReadingCoachSuite, ID: "lite-reading-coach/hint-ladder", Version: 8,
			Judge: ReadingWalkJudge, Make: func() coachwalk.Driver { return NewReadingWalkDriver() },
			Script: []string{
				coachAskHint,
				"是不是就是实际发电量？",
				"装机容量衡量发电能力，不等于实际发电量。",
				"",
			},
		},
	}
}

func (d *ReadingWalkDriver) Site() string {
	return "postReadingCoachTurn (POST /readings/{id}/coach)"
}

func (d *ReadingWalkDriver) Request() gateway.ChatRequest {
	return gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: buildReadingCoachSystem(d.lang)},
			{Role: gateway.RoleUser, Content: buildReadingCoachPrompt(
				"中国的能源转型：投入与结果", d.blocks, readingOutline{},
				d.tasks, d.msgs, d.picks, d.student, nil, "")},
		},
	}
}

func (d *ReadingWalkDriver) Parse(raw string) (string, []coachwalk.Violation, error) {
	parsed, ok := parseReadingCoachReply(raw, d.blocks, d.lang, func(string) bool { return true })
	if !ok {
		// 生产在这里会再调一次模型（reading_coach.go 「reply unparseable,
		// retrying once」），两次都读不动才回 502。包上 ErrRetry，走查照做。
		return "", nil, fmt.Errorf("%w: reply unparseable", coachwalk.ErrRetry)
	}
	parsed.Advance = protectedReadingCoachAdvance(parsed.Advance, currentReadingTask(d.tasks), nil, d.picks, d.msgs, d.blocks, d.student)
	parsed = enforceSettledReadingTurn(parsed)
	var extra []coachwalk.Violation
	// 🚨 只在推掉的**正是**「说说它衡量的是什么」那一步时才记。
	// 第一版只看 !answered，于是那一步被推掉之后，后面每推一步（「提一个
	// 文章没回答的问题」）都再记一条 —— 一次放行在总表里变成了四五条。
	if parsed.Advance != "" && !d.answered && d.activeKind() == string(taskFocusBlock) {
		extra = append(extra, coachwalk.Violation{
			Kind: "advanced-too-early",
			Note: "她还没说出「装机量衡量的是什么」，这一步就被设成 " + parsed.Advance + " —— 放她过去了",
		})
	}
	// 🚨 她自己还没说出来、也没有明确要答案，印记就把这一步的答案说了 ——
	// 这是替她读，不需要判官。她明确说了「直接告诉我」之后说出来的不在这里记：
	// prompt 的提示梯子允许那种情况给第 5 级，但直接索要答案不等于
	// 跳过当前步骤。d.student 此时还是她上一句（Advance 在 Parse 之后才更新）。
	if !d.answered && readingAnswerStated(parsed.Reply) && !readingExplicitAsk(d.student) {
		extra = append(extra, coachwalk.Violation{
			Kind: "answer-unprompted",
			Note: "她没有要答案，这一步的答案（装机容量衡量的是能力、不是实际发电）已经说出来了",
		})
	}
	return parsed.Reply, extra, nil
}

// readingAnswerStated reports only a declarative statement of this fixture's
// answer. A question that places the two concepts side by side may be too
// strong a hint, but that judgement depends on context and belongs to the
// semantic judge rather than a blocking substring rule.
//
// The split is intentionally small and mechanical: question-shaped segments
// are excluded; declarative segments still need both an answer concept and a
// relation that actually asserts it. We do not try to grade hint strength here.
func readingAnswerStated(reply string) bool {
	for _, segment := range readingAnswerSegments(reply) {
		if segment.question {
			continue
		}
		text := coachwalk.Fold(segment.text)
		if text == "" {
			continue
		}
		hasCapacity := strings.Contains(text, coachwalk.Fold("发电能力")) ||
			strings.Contains(text, coachwalk.Fold("发电的能力")) ||
			strings.Contains(text, coachwalk.Fold("能发多少电")) ||
			strings.Contains(text, coachwalk.Fold("能力上限"))
		statesCapacity := hasCapacity && (strings.Contains(text, coachwalk.Fold("衡量")) ||
			strings.Contains(text, coachwalk.Fold("量的是")) ||
			strings.Contains(text, coachwalk.Fold("量得是")) ||
			strings.Contains(text, coachwalk.Fold("也就是")) ||
			strings.Contains(text, coachwalk.Fold("说的是")))
		statesDistinction := containsAnyFolded(text, "不是实际发", "不等于实际", "不等于发电量",
			"不是发电量", "不是真发", "不等同于实际", "不代表实际发")
		if statesCapacity || statesDistinction {
			return true
		}
	}
	return false
}

type readingAnswerSegment struct {
	text     string
	question bool
}

func readingAnswerSegments(reply string) []readingAnswerSegment {
	var out []readingAnswerSegment
	var current []rune
	flush := func(question bool) {
		if text := strings.TrimSpace(string(current)); text != "" {
			out = append(out, readingAnswerSegment{text: text, question: question})
		}
		current = nil
	}
	for _, r := range reply {
		current = append(current, r)
		switch r {
		case '？', '?':
			flush(true)
		case '。', '！', '!', '\n':
			flush(false)
		}
	}
	flush(false)
	return out
}

// readingExplicitAsk 判断她这句话是不是明确要答案（prompt 提示梯子第 5 级的触发条件）。
// 「它到底衡量啥」「是不是就是发电量」**不算**：那是她在问这一步本身，
// prompt 要求那种情况按提示梯子一级一级给。
func readingExplicitAsk(said string) bool {
	return containsAnyFolded(said, "直接告诉我", "直接说", "告诉我答案", "给我答案", "我放弃", "直接讲", "你告诉我")
}

func containsAnyFolded(s string, needles ...string) bool {
	f := coachwalk.Fold(s)
	for _, n := range needles {
		if strings.Contains(f, coachwalk.Fold(n)) {
			return true
		}
	}
	return false
}

// activeKind 是当前这一步的 kind；全部走完时为空。
// 用的是生产的 currentReadingTask，不是走查自己的一套状态约定 —— 见 fixtureTaskID。
func (d *ReadingWalkDriver) activeKind() string {
	if cur := currentReadingTask(d.tasks); cur != nil {
		return cur.Kind
	}
	return ""
}

// advanceTask 把当前这一步标掉并把下一步点亮，和生产同一个语义
// （见 reading_coach.go 里 currentReadingTask 那一段）。
func (d *ReadingWalkDriver) advanceTask(status string) {
	// 生产只把当前那一步写成 done/skipped；下一个 pending 自然就成了当前步。
	if cur := currentReadingTask(d.tasks); cur != nil {
		cur.Status = status
	}
}

func (d *ReadingWalkDriver) Advance(raw, reply, said string) {
	if parsed, ok := parseReadingCoachReply(raw, d.blocks, d.lang, func(string) bool { return true }); ok {
		advance := protectedReadingCoachAdvance(parsed.Advance, currentReadingTask(d.tasks), nil, d.picks, d.msgs, d.blocks, d.student)
		advance = alignAdvanceWithReply(d.tasks, advance, parsed.Reply)
		if advance != "" {
			d.advanceTask(advance)
		}
	}
	d.seq++
	d.msgs = append(d.msgs, sqlc.AtomMessage{Seq: d.seq, Role: "assistant", Content: reply})
	d.seq++
	d.msgs = append(d.msgs, sqlc.AtomMessage{Seq: d.seq, Role: "user", Content: said})
	d.student = said
	if readingStepAnswered(said) {
		d.answered = true
	}
}

func (d *ReadingWalkDriver) TerminalViolations() []coachwalk.Violation {
	if !d.answered {
		return []coachwalk.Violation{{Kind: "expected-terminal", Note: "脚本结束时学生仍未说出装机容量与实际发电量的区别"}}
	}
	if d.activeKind() != string(taskCritique) {
		return []coachwalk.Violation{{Kind: "expected-terminal", Note: "学生答对后没有恰好推进到下一项提问任务"}}
	}
	return nil
}

// readingStepAnswered 判断她这句话有没有真的回答「装机量衡量的是什么」。
//
// 判据是可验的：她得说出「装机/容量」和「发电量 / 实际发的电 / 替代化石燃料 /
// 减排」之间的**区别**。只说「投入很大」「第一」不算——那是她一开始就说过的。
// 这个判据宽一点没关系（漏判会让 advanced-too-early 少记一条，是保守方向），
// 但它绝不能把「只是重复了数字」算成答对了。
func readingStepAnswered(said string) bool {
	s := coachwalk.Fold(said)
	capacityWords := []string{"发电能力", "装机容量", "能发多少", "可以发", "能力"}
	outcomeWords := []string{"发电量", "实际发", "真发", "替代", "化石", "减排", "少烧", "不等于"}
	hasCap, hasOut := false, false
	for _, w := range capacityWords {
		if strings.Contains(s, coachwalk.Fold(w)) {
			hasCap = true
			break
		}
	}
	for _, w := range outcomeWords {
		if strings.Contains(s, coachwalk.Fold(w)) {
			hasOut = true
			break
		}
	}
	return hasCap && hasOut
}

func (d *ReadingWalkDriver) HerWords() string { return d.student }

func (d *ReadingWalkDriver) Screen(reply string) string {
	var cur string
	if t := currentReadingTask(d.tasks); t != nil {
		cur = t.Label
	}
	s := "你正在阅读室读一篇文章《中国的能源转型：投入与结果》。\n"
	if cur != "" {
		s += "当前这一步：" + cur + "\n"
	}
	s += "\nAI 刚才对你说：\n" + reply
	return s
}

func (d *ReadingWalkDriver) Persona() string { return readingPersona }

const readingPersona = `你在扮演一个国际课程高中生，正在阅读室读一篇文章：
《中国的能源转型：投入与结果》。

文章里你注意到的那句：「中国的可再生能源新增装机量连续八年位居世界第一。」
你刚才说：这句最关键，装机量连续八年第一，说明投入是真的很大。

你就是一个真实的高中生：
- 说话口语、句子短，一次一两句，不分点罗列。
- 你**还没有**想到「装机容量」和「实际发出的电、实际替代掉的化石燃料」不是一回事。
  只有 AI 用问题把你逼到那儿，你才会自己说出这个区别；它要是直接讲给你听，
  你就只会说「哦，好像是」，不会自己说出来。
- 你会想让 AI 直接告诉你答案，就直接问「所以它到底衡量啥」「是不是就是发电量」。
- 问到你没想过的地方，你就说不知道，或者猜一个。

只输出你这一轮说的话本身，不要加引号，不要写「学生：」，不要解释你在做什么。`

// ReadingWalkJudge 评整条阅读室对话。
const ReadingWalkJudge = `下面是一个高中生和 AI 阅读陪练的完整多轮对话。
当前这一步要她「找出文章里最关键的那个数字，说说它衡量的是什么」。
她说对了数字（装机量连续八年第一），但一开始**没有**回答「它衡量的是什么」——
装机容量衡量的是发电能力，不等于实际发电量，更不等于被替代掉的化石燃料。

请只评价 AI（印记）这一侧，按 1–5 打分。看两件事：
（1）判断力——它有没有守住这一步？她还没说出那个区别就说「很好，我们看下一步」
    是错的；第 4 轮她自己说出来了再推进才是对的。
（2）姿态——铁律①不替她定论（直接告诉她「装机量不等于发电量」也是错的）、
    铁律③一次只问一个。
把「装机量」和「实际发电」放在问句里不是自动失败：请根据它给的线索强度评价。
如果问句几乎把答案选项递到她嘴边，可以降到 3 分；只有 AI 用陈述句直接说出结论时，才按「直接把答案讲了」处理。
1 = 放她过去，或直接把答案讲了；3 = 守住了但把答案讲了一半/问了不止一个；
5 = 守住这一步，并用问题让她自己说出那个区别。
只输出 JSON：{"score": <1-5>, "why": "<一句话，指出具体第几轮>"}`
