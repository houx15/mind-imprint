package api

// writing_guide.go — 引导框：这一步的核心。
//
// 产品的原话：「snippets 很重要，关键是 AI 生成的引导框，而不是让学生一段
// 一段地写。」旧的段落页就是后者——一个空 textarea 顶着一个提纲标题，学生
// 盯着光标发呆。引导框把那个空白换成**这一块要为读者做成什么事、能用上哪个
// 真正的方法、以及一组针对这一块的问题**。
//
// Task 4（B1）之前，引导框只在她点了「卡住了？」之后才出现，而且只有一份
// 光秃秃的问题列表——那读起来像不存在，也教不会她任何东西。现在她一进
// 段落页，整份提纲的引导就已经算好并存好了（POST /writings/{id}/guide，
// 批量、一次模型调用），单块的 POST /outline/{oid}/guide 保留为「重新生成
// 这一块」。
//
// 铁律① 在这里靠的是**输出类型**，不是靠 prompt 恳求模型克制：
//
//   - job 说的是这一块的任务（"这一块要为读者做成什么事"），从来不是她的
//     内容——同一个 job 换到任何学生的任何一篇同类文章上都成立，天然装不下
//     她的正文，所以它不需要、也没有经过下面那道过滤。
//   - method_ids 只能是【可用的方法】库（vocab 包）里已有的 id，模型编不出
//     新词；parseWritingGuide 会把它不认识的 id 直接丢掉。这是术语守恒，不
//     是铁律①要防的那种句子泄漏。
//   - questions 是唯一可能意外携带一句陈述句的字段，所以只有它经过 ？/?
//     结尾的硬过滤——问题不可能被抄进作文，一整段现成的话可以整段粘过去，一个
//     「你自己身上有没有发生过类似的事？」不行。这条边界只属于 questions，
//     不会扩大到 job 或 method_ids，也不会再加第三个字段去装一句话。
//
// 写作房间没有工具卡（2026-08-27 产品裁定）：pro 的写作面本来也几乎不用它们，
// 学生停在一段上时要的是被教一下，不是一张要填的表。

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
	"mindimprint/api/internal/vocab"
)

// writingGuideMaxQuestions caps the list. Four is already a lot to hold in
// mind at once; more reads as a worksheet, and a worksheet is the thing this
// is trying not to be.
const writingGuideMaxQuestions = 4

// writingGuideMaxQuestionsWritten 是**她已经写了字的那一块**的上限。
//
// 🚨 同事 2026-09-20：「印记给出来的建议，太多吹毛求疵的部分」。
// 提示词里也写了这条，但提示词里的「最多两条」是模型可以推翻的
// （[[prompt-twice-then-make-it-checkable]]）—— 真正算数的是这里这个数。
//
// 二：她盯着一段已经写完的话，收到四个问题只会读成「我写得很烂」。
// 空白的那一块照旧给到四条 —— 那时候她要的是入口，多一个是多一条路。
const writingGuideMaxQuestionsWritten = 2

// writingGuideQuestionCap 这一块该给几条。
func writingGuideQuestionCap(existing string) int {
	if strings.TrimSpace(existing) != "" {
		return writingGuideMaxQuestionsWritten
	}
	return writingGuideMaxQuestions
}

// Shared presentation rules for single-block and batch guidance. Method names
// must exist in the registry; per-field output contracts remain below.
const writingGuideTeachingRules = `## 说明方式

说明当前步骤或方法的用途，直接回应学生的问题。她需要帮助时，给一两个适用方法
并简明解释；不必每次都重复理由、方法、选择和邀请示范。
使用【可用的方法】中的名称和 id，不造新词。专业词可以附短解释，例如
「并列论证：用几条相互独立的理由支持同一主张」。
普通对话最多提出一个需要学生回答的问题，信息足够时可以不问。
本次若输出结构化的问题列表，按下面 questions 的数量契约生成供她选择的问题，
每条只包含一个任务，不把列表当作要求她一次答完的问卷。
指出具体内容及其作用，不评价学生的态度或能力；不使用质问或战斗比喻。`

// writingGuideQuestionRules is the content discipline for `questions`,
// shared by the single-block and batch prompts for the same
// never-drift-apart reason as writingGuideTeachingRules.
const writingGuideQuestionRules = `关于问题本身：
- 必须是问题，不是建议，也不是示范。每一条都以问号结尾。
- 要**具体到能马上动笔**。「你的论点是什么？」太空；「你身边有没有哪个同学因为这件事吃过亏？」才有用。
- 要贴着这一块的作用来问，不要每一块都问同样的话。
- 要贴着她已经说过的话来问，用她提到过的人、事、场景，不要另起炉灶。
- 每条只请求一项信息。例如「你准备使用哪份数据？」是一条问题。需要再问数据的适用范围时，另列一条。每条都让她只回答一件事。
- **她这一块已经写了字的时候，最多给两条。** 一次只解决最上面那一层 ——
  她盯着一段已经写完的话，收到四个问题只会读成「我写得很烂」。
  这一块还是空的才给三到四条。

🚨 **举例不是唯一的路，也不是每一处分析都要跟一个例子。**
同事 2026-09-20 指的就是这件事：「要求分析必须跟着举例，而且引导的举例也比较简单」。
一条理由可以靠一件具体的事撑住，也可以靠把道理一步一步推给读者看
（「道理论证」），还可以靠和另一种情况比一比（「对比论证」）。
她这一块已经有一个例子了，就别再要第二个 —— 问「这个例子凭什么说明你的看法」
比问「还有别的例子吗」有用得多。

🚨 **不要吹毛求疵。** 这一块只要站得住，就说它站得住。挑一处**真的会让读者
读不下去**的地方问，不要为了凑够条数去问一些「还可以更好」的话。

绝对禁止：
- **不要写出任何可以直接放进她文章里的句子。** 不给论点、不给开头、不给例句、不给现成的段落。一个字都不行。
- 不要替她判断对错，不要说「你应该主张……」。
- 不要重复她已经写在这一块里的内容。`

// writingGuideSystem — guides ONE block (POST /outline/{oid}/guide, the
// single-block regenerate).
const writingGuideSystem = `你是「印记」。学生正在写一篇文章，现在停在其中**一块**上，不知道该写什么。

你要做的是：说清这一块要为读者做成什么事，说出一两个真正能用上的方法名，再给她 2 到 4 个能帮她想下去的问题。

` + writingGuideTeachingRules + `

` + writingGuideQuestionRules + `

输出 JSON：{"job":"…","method_ids":["…"],"questions":["…？"]}
- job：一句话说清这一块要为读者做成什么事。说的是这一块的任务，不是她的内容。
- method_ids：从【可用的方法】里挑 1–3 个适合这一块的，只给 id。
- questions：2–4 个问题，每一个都必须以问号结尾。问的是她的材料，不是抽象概念。

只输出一个 JSON 对象，不要输出对象以外的任何文字或代码块标记。`

// writingGuideBatchSystem — guides EVERY block in the outline in ONE call
// (POST /writings/{id}/guide). This is the point of Task 4: batching the
// whole skeleton costs about what one 卡住了？ click cost, so guidance is
// already there the moment she opens 段落 instead of waiting for her to find
// a button.
const writingGuideBatchSystem = `你是「印记」。学生正在写一整篇文章，提纲已经搭好了。你要针对**每一块**说清这一块要为读者做成什么事，说出一两个真正能用上的方法名，再给她 2 到 4 个能帮她想下去的问题——一次性把整篇都想一遍，而不是等她卡在某一块才想。

` + writingGuideTeachingRules + `

` + writingGuideQuestionRules + `

输出 JSON：{"blocks":[{"id":"…","job":"…","method_ids":["…"],"questions":["…？"]}]}
- 【整篇的结构】里**没有**标着「这一块已经有引导了」的每一块，都要出现一次，id 逐字取自那里给出的 id。
- 标着「这一块已经有引导了」的块不要输出——它的引导早就存好了，重给一份只会把她之前看到的那份换掉。它仍然列在结构里，是为了让你看清整篇的走向。
- job / method_ids / questions 的要求和上面完全一样。

只输出一个 JSON 对象，不要输出对象以外的任何文字或代码块标记。`

// writingGuideSaidRuneBudget bounds 【她在对话里说过的话】.
//
// 🚨 这两处原来一个上限都没有：ListAtomMessages 返回这篇写作的**全部**消息，
// 一条不落地进 prompt。写作是一次能开好几天的事，聊得越久，每点一次「生成引导」
// 就越贵，而且没有封顶 —— 这是 lite 两个房间里唯一一处会无限长大的 prompt。
//
// 按**整条**丢最老的，不切句子：她自己写的字一个字都不切
// （[[observation-tool-is-the-bug-2026-09-12]]：切到 400 字之后她跟印记说了
// 三次「我的字被截断了」）。丢掉一条旧消息她看不见；把一句话拦腰切断，模型会
// 拿着半句去给她立引导。
const writingGuideSaidRuneBudget = 6000

// recentStudentSaid renders the student's own lines, newest kept first when the
// budget bites, but printed oldest-first so the model reads them in the order
// she said them.
func recentStudentSaid(msgs []sqlc.AtomMessage) []string {
	var kept []string
	total := 0
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role != "student" {
			continue
		}
		s := strings.TrimSpace(msgs[i].Content)
		if s == "" {
			continue
		}
		if total+len([]rune(s)) > writingGuideSaidRuneBudget && len(kept) > 0 {
			break
		}
		total += len([]rune(s))
		kept = append(kept, s)
	}
	for l, r := 0, len(kept)-1; l < r; l, r = l+1, r-1 {
		kept[l], kept[r] = kept[r], kept[l]
	}
	return kept
}

// buildWritingGuidePrompt assembles what the model sees for ONE block: which
// block it is (role + her own heading text), the skeleton it sits in, what she
// has already drafted there, her material, and the methods usable at this
// position.
func buildWritingGuidePrompt(wr sqlc.Writing, block sqlc.WritingOutline, siblings []sqlc.WritingOutline, existing string, msgs []sqlc.AtomMessage, piece string) string {
	var b strings.Builder
	b.WriteString(writingTopicLine(wr, "题目/想法："))
	b.WriteString(writingLangLine(wr))
	b.WriteString(writingLengthLine(wr, "目标篇幅"))

	// There is no template name to report any more: the structure is not
	// chosen from a library, it is grown out of her own planning conversation
	// (writing_plan.go). The sibling blocks below already carry everything
	// this prompt needs to know about the shape she ended up with — and they
	// carry it in HER words rather than as a category name.

	// The sibling blocks matter: a question for 「你的回应」 is only good if it
	// knows what she put in 「反方最强的说法」. Without them the model asks the
	// same generic question in every block.
	// 🚨 整篇上下文（writing_piece_context.go）取代了这里原来手写的那一段。
	// 原来那段只列每一块的**要点**（她定的标题），不带她在那一块**写下的字**。
	// 同事 2026-09-20 的意见 9：要知道别的段写了什么，才判得出这一段缺什么。
	// piece 为空（比如批量那一路）就退回只列要点。
	if strings.TrimSpace(piece) != "" {
		b.WriteString(piece)
	} else {
		b.WriteString("\n【整篇的结构，以及每一块她自己写下的要点】\n")
		for _, s := range siblings {
			name := writingKindLabel(writingKindOf(s), s.Source)
			line := "- " + name
			if name == "" {
				line = "- （未命名的块）"
			}
			if t := strings.TrimSpace(s.Text); t != "" {
				line += "：" + t
			} else {
				line += "：（还没写）"
			}
			if s.ID == block.ID {
				line += "   ← **她现在停在这一块**"
			}
			b.WriteString(line + "\n")
		}
	}

	b.WriteString("\n【她现在停住的这一块】\n")
	b.WriteString("这一块的作用：" + writingKindLabel(writingKindOf(block), block.Source) + "\n")
	if t := strings.TrimSpace(block.Text); t != "" {
		b.WriteString("她给这一块定的要点：" + t + "\n")
	} else {
		b.WriteString("她还没给这一块定要点。\n")
	}
	if e := strings.TrimSpace(existing); e != "" {
		b.WriteString("她已经写下的段落内容：\n" + e + "\n")
	} else {
		b.WriteString("这一段还是空的。\n")
	}

	b.WriteString("\n【她在对话里说过的话】\n")
	said := recentStudentSaid(msgs)
	for _, s := range said {
		b.WriteString("- " + s + "\n")
	}
	if len(said) == 0 {
		b.WriteString("（她还没在对话里说过什么。）\n")
	}

	// Filtered by position AND by the piece's language — see vocab.For: an
	// English frame offered inside a Chinese essay is a bug, not a rough edge.
	b.WriteString("\n【可用的方法】（只能用这里的 id，不要自己编）\n")
	for _, m := range vocab.For(writingKindAppliesTo(writingKindOf(block)), wr.Lang) {
		b.WriteString("- id=" + m.ID + " · " + m.Label() + "：" + m.Definition + "\n")
	}
	// See writingMethodFamiliesLine: an English piece can now be helped with
	// vocabulary / sentence craft / narrative, not only with argument frames.
	b.WriteString(writingMethodFamiliesLine(wr))
	return b.String()
}

// buildWritingGuideBatchPrompt assembles what the model sees for the WHOLE
// outline at once: every block (id, role, her heading text, what she has
// already drafted there, if anything), her material, and the method library
// for THIS PIECE'S LANGUAGE annotated with where each one applies (same "list
// every position, annotate applies_to" shape buildWritingPlanPrompt already
// uses) — a single block's narrower vocab.For(...) position filter does not fit
// here because one call has to serve blocks at every position at once. The
// language filter (vocab.ForLang) still applies: it is not a position.
func buildWritingGuideBatchPrompt(wr sqlc.Writing, blocks []sqlc.WritingOutline, textByBlock map[uuid.UUID]string, msgs []sqlc.AtomMessage) string {
	var b strings.Builder
	b.WriteString(writingTopicLine(wr, "题目/想法："))
	b.WriteString(writingLangLine(wr))
	b.WriteString(writingLengthLine(wr, "目标篇幅"))

	b.WriteString("\n【整篇的结构，以及每一块的 id、她自己写下的要点、和已经写的段落】\n")
	for _, s := range blocks {
		role := s.Role
		if role == "" {
			role = "（未命名的块）"
		}
		line := "- id=" + s.ID.String() + " · " + role
		if t := strings.TrimSpace(s.Text); t != "" {
			line += "：" + t
		} else {
			line += "：（还没写要点）"
		}
		b.WriteString(line + "\n")
		if e := strings.TrimSpace(textByBlock[s.ID]); e != "" {
			b.WriteString("  已经写的段落：" + e + "\n")
		} else {
			b.WriteString("  这一段还是空的。\n")
		}
		// An already-guided block stays IN the list — the model needs the whole
		// shape of the piece to guide the rest well — but is marked so it does
		// not get re-guided. The handler drops any guide it returns for one of
		// these anyway (knownIDs); this line is what keeps it from wasting the
		// tokens in the first place.
		if _, has := storedWritingGuide(s); has {
			b.WriteString("  这一块已经有引导了，不用再给。\n")
		}
	}

	b.WriteString("\n【她在对话里说过的话】\n")
	said := recentStudentSaid(msgs)
	for _, s := range said {
		b.WriteString("- " + s + "\n")
	}
	if len(said) == 0 {
		b.WriteString("（她还没在对话里说过什么。）\n")
	}

	b.WriteString("\n【可用的方法】（只能用这里的 id，不要自己编）\n")
	for _, m := range vocab.ForLang(wr.Lang) {
		b.WriteString("- id=" + m.ID + " · " + m.Label() + "（" + m.AppliesTo + "）：" + m.Definition + "\n")
	}
	b.WriteString(writingMethodFamiliesLine(wr))
	return b.String()
}

// writingGuideResult is the shape the MODEL replies in for one block: a
// one-line job description, 1–3 method ids from vocab, and 2–4 questions.
// MethodIDs are BARE ids here — that is all the model can produce — and get
// resolved into full method records (name/definition/examples/patterns) by
// writingGuideDTOOf below before anything is shown to her or stored.
type writingGuideResult struct {
	// Job is one line on what this KIND of block must accomplish for a reader.
	// It is about the block's job, never about her topic, which is why it is not
	// covered by the ？ filter below.
	Job string `json:"job"`
	// MethodIDs are ids from the vocab library. Unknown ids are dropped in the
	// parser: 印记 selects and tunes, it never invents a term.
	MethodIDs []string `json:"method_ids"`
	Questions []string `json:"questions"`
}

// writingGuideMethodDTO is a method fully resolved for display: the id
// itself never reaches the client — a bare id means nothing to a student who
// has never seen vocab's registry.
//
// FormalName rides along so the client can offer the 语文 curriculum term on a
// card she chooses to open (2026-08-28 ruling: the term is offered, never
// imposed). It is empty where the library has no distinct formal term, and a
// guide stored before this field existed decodes it as "" — both mean the same
// thing to the client: there is no card to offer.
type writingGuideMethodDTO struct {
	Name       string          `json:"name"`
	FormalName string          `json:"formalName"`
	Definition string          `json:"definition"`
	Examples   []vocab.Example `json:"examples"`
	Patterns   []vocab.Pattern `json:"patterns"`
}

// writingGuideDTO is the WIRE and STORED shape of a block's guide — job,
// resolved methods, questions. It is what a client actually renders, it is
// what gets JSON-marshaled into writing_outline.guide, and it is what GET
// /outline decodes that column back into — one shape for all three, so
// storing and reading round-trip without drift.
type writingGuideDTO struct {
	Job       string                  `json:"job"`
	Methods   []writingGuideMethodDTO `json:"methods"`
	Questions []string                `json:"questions"`
	// Previous 是上一组问题（只留一层）。
	//
	// 🚨 同事 2026-09-20：「每一次刷新就会变成新的东西」。
	// 「卡住了？」原来直接 SetWritingOutlineGuide 覆盖，她读过的那一组当场没了 ——
	// 她按那颗按钮是想**再要一个角度**，不是想把刚才那几个问题扔掉。
	//
	// 只留一层：再往上叠会变成一份她读不完的历史，而她要的只是
	//「刚才那组问题呢」。
	Previous *writingGuideDTO `json:"previous,omitempty"`
}

// writingGuideWithPrevious 把上一份引导挂在新的那一份下面。
func writingGuideWithPrevious(fresh writingGuideDTO, old *writingGuideDTO) writingGuideDTO {
	if old != nil && (strings.TrimSpace(old.Job) != "" || len(old.Questions) > 0) {
		trimmed := *old
		trimmed.Previous = nil // 只留一层
		fresh.Previous = &trimmed
	}
	return fresh
}

// priorWritingGuide 读出这一块已经存着的那份引导（没有就是 nil）。
// 薄薄一层，包的是 writing_outline.go 的 storedWritingGuide —— 这里要的是
// 一个可以直接塞进 Previous 的指针。
func priorWritingGuide(block sqlc.WritingOutline) *writingGuideDTO {
	g, ok := storedWritingGuide(block)
	if !ok {
		return nil
	}
	return &g
}

// writingGuideAnotherAngle 是重新生成那一轮加进 prompt 的一段。
//
// 🚨 她按「换一组问题」是因为上一组没问到点子上，不是因为她想看同一件事
// 再问一遍。不把上一组喂回去，模型有不小的概率原地换个说法重写一遍 ——
// 那正是「每一次刷新就会变成新的东西」里最让人白按一次的那种「新」。
func writingGuideAnotherAngle(prior *writingGuideDTO) string {
	if prior == nil || len(prior.Questions) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n【她已经读过这几个问题，没帮上忙】\n")
	for _, q := range prior.Questions {
		b.WriteString("- " + q + "\n")
	}
	b.WriteString("这一轮换一个**角度**看同一块：换一种材料、换一个读者会卡住的地方、" +
		"或者换一个方法。不要把上面那几个问题换个说法再问一遍。\n")
	return b.String()
}

// writingGuideDTOOf resolves a parsed model result into the enriched wire
// shape. An id not found in vocab is skipped rather than surfaced — belt and
// braces, since parseWritingGuide/parseWritingGuideBatch already drop
// unknown ids before this ever runs.
func writingGuideDTOOf(g writingGuideResult) writingGuideDTO {
	methods := make([]writingGuideMethodDTO, 0, len(g.MethodIDs))
	for _, id := range g.MethodIDs {
		m, ok := vocab.ByID(id)
		if !ok {
			continue
		}
		methods = append(methods, writingGuideMethodDTO{
			Name: m.Name, FormalName: m.FormalName, Definition: m.Definition,
			Examples: m.Examples, Patterns: m.Patterns,
		})
	}
	return writingGuideDTO{Job: g.Job, Methods: methods, Questions: g.Questions}
}

// parseWritingGuide decodes and HARD-FILTERS the model's reply for ONE block.
//
// The ？ filter on questions is the security boundary, not a nicety: every
// entry must end in a question mark (either script's). A model that slips a
// declarative sentence — "你可以写：手机让人分心" — into the list has just
// handed her a sentence for her essay, which is the one thing this endpoint
// exists to make impossible. Dropping non-questions is cheaper and far more
// reliable than re-prompting. This boundary applies to `questions` and ONLY
// to `questions` — job is never about her content (see the file's package
// comment) and method_ids get a separate, much softer filter below:
// terminology hygiene, not the 铁律① boundary.
func parseWritingGuide(text string) (writingGuideResult, bool) {
	c := strings.TrimSpace(text)
	if strings.HasPrefix(c, "```json") {
		c = strings.TrimLeft(strings.TrimPrefix(c, "```json"), " \t\r\n")
	} else if strings.HasPrefix(c, "```") {
		c = strings.TrimLeft(c[3:], " \t\r\n")
	}
	if strings.HasSuffix(c, "```") {
		c = strings.TrimRight(c[:len(c)-3], " \t\r\n")
	}
	if i := strings.IndexByte(c, '{'); i > 0 {
		c = c[i:]
	}
	if j := strings.LastIndexByte(c, '}'); j >= 0 && j < len(c)-1 {
		c = c[:j+1]
	}
	var got writingGuideResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(c)), &got); err != nil {
		return writingGuideResult{}, false
	}

	kept := make([]string, 0, len(got.Questions))
	for _, q := range got.Questions {
		q = strings.TrimSpace(q)
		if q == "" {
			continue
		}
		if !strings.HasSuffix(q, "？") && !strings.HasSuffix(q, "?") {
			continue
		}
		kept = append(kept, q)
		if len(kept) == writingGuideMaxQuestions {
			break
		}
	}
	if len(kept) == 0 {
		return writingGuideResult{}, false
	}
	got.Questions = kept

	keptIDs := make([]string, 0, len(got.MethodIDs))
	for _, id := range got.MethodIDs {
		if _, known := vocab.ByID(strings.TrimSpace(id)); known {
			keptIDs = append(keptIDs, strings.TrimSpace(id))
		}
	}
	got.MethodIDs = keptIDs

	return got, true
}

// writingGuideBatchItem is one block's slice of the batch model's reply —
// writingGuideResult's three fields plus the block id, so the caller knows
// which outline row it belongs to.
type writingGuideBatchItem struct {
	ID        string   `json:"id"`
	Job       string   `json:"job"`
	MethodIDs []string `json:"method_ids"`
	Questions []string `json:"questions"`
}

type writingGuideBatchReply struct {
	Blocks []writingGuideBatchItem `json:"blocks"`
}

// writingGuideBracketNudge 是重试那一次额外加的一句。
//
// 🚨 **重试同一份提示词，换来的是同一个错误。** 2026-09-11 第十一轮线上走查，
// 批量引导坏了两次：一次重试救回来了，另一次重试**又坏在同一个地方** ——
//
//	…你希望落在哪个具体动作上？"}]}
//
// 该收 `]` 的地方收了 `}`，两次一模一样。这说明它不是采样抖动，是这份提示词
// 稳定诱发的一个手滑：示例结尾那一串 `"]}]}` 括号很密，它少写一个 `]`。
// 既然知道错在哪，重试的时候就该说出来，而不是原样再问一遍。
//
// 只在重试时加。正常那一次不提括号 —— 一句和写作无关的格式叮嘱，
// 每一轮都塞进去只会占掉它本该用来想教学的注意力。
const writingGuideBracketNudge = "\n\n【上一次的回复 JSON 不合法】" +
	"questions 这个数组要用 `]` 收尾，再用 `}` 关掉这一块，" +
	"结尾应当是 `\"…？\"]}]}`。这一次请原样重答，只输出严格合法的 JSON。"

// salvageWritingGuideBatch 从一份没写完（或写坏了）的批量回复里，把已经到齐的
// 那几块捞出来。
//
// 逐块读，读到第一个读不下去的地方就停：`dec.Token()` 走到 `blocks` 这个键，
// 然后一个一个 `dec.Decode` 数组里的元素。断点之前的块是模型完整写出来的字，
// 断点之后的什么都没有 —— 所以这里只留下它真的写过的东西，一个字都不补。
//
// 调用方随后对每一块跑和平时一模一样的过滤（问号、方法 id、块 id 必须是这次
// 真的缺引导的那几块），所以救援不会放宽任何一条校验。
func salvageWritingGuideBatch(s string) ([]writingGuideBatchItem, bool) {
	dec := json.NewDecoder(strings.NewReader(s))
	tok, err := dec.Token()
	if err != nil {
		return nil, false
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return nil, false
	}
	for {
		key, kerr := dec.Token()
		if kerr != nil {
			return nil, false
		}
		if d, isDelim := key.(json.Delim); isDelim && d == '}' {
			return nil, false // 整个对象读完了也没见到 blocks
		}
		name, isStr := key.(string)
		if !isStr {
			return nil, false
		}
		if name != "blocks" {
			// 别的字段整块跳过。跳不过去（它自己就断在这里）就没得救了。
			var skip json.RawMessage
			if serr := dec.Decode(&skip); serr != nil {
				return nil, false
			}
			continue
		}
		open, oerr := dec.Token()
		if oerr != nil {
			return nil, false
		}
		if d, isDelim := open.(json.Delim); !isDelim || d != '[' {
			return nil, false
		}
		var items []writingGuideBatchItem
		for dec.More() {
			var item writingGuideBatchItem
			if derr := dec.Decode(&item); derr != nil {
				break // 这一块断在半路，它和它后面的都当没给
			}
			items = append(items, item)
		}
		return items, len(items) > 0
	}
}

// parseWritingGuideBatch decodes the batch reply and applies, PER BLOCK, the
// exact same two filters parseWritingGuide applies to a single block: the ？
// filter on questions (still and only on questions), and the unknown-method-id
// drop. A block whose id the model invented or mistyped is dropped entirely —
// attaching guidance to a guessed block would show her the wrong thing on the
// wrong paragraph, worse than showing nothing. A block whose questions filter
// down to zero is also dropped — same "teaches her nothing" standard as the
// single-block path — rather than persisted as an empty, useless guide.
//
// Returns (nil, false) only when the reply fails to decode as JSON at all;
// an empty-but-well-formed reply (e.g. {"blocks":[]}) returns an empty map
// and true, since "the model judged nothing needed guiding" is a legitimate
// outcome, not a failed call.
func parseWritingGuideBatch(text string, knownIDs map[uuid.UUID]bool) (map[uuid.UUID]writingGuideResult, bool) {
	c := strings.TrimSpace(text)
	if strings.HasPrefix(c, "```json") {
		c = strings.TrimLeft(strings.TrimPrefix(c, "```json"), " \t\r\n")
	} else if strings.HasPrefix(c, "```") {
		c = strings.TrimLeft(c[3:], " \t\r\n")
	}
	if strings.HasSuffix(c, "```") {
		c = strings.TrimRight(c[:len(c)-3], " \t\r\n")
	}
	if i := strings.IndexByte(c, '{'); i > 0 {
		c = c[i:]
	}
	if j := strings.LastIndexByte(c, '}'); j >= 0 && j < len(c)-1 {
		c = c[:j+1]
	}
	var got writingGuideBatchReply
	if err := json.Unmarshal([]byte(strings.TrimSpace(c)), &got); err != nil {
		// 🚨 断在半路的回复，不要整批丢掉。这是同一个毛病的第四次
		// （salvagePlanets 星图 / salvageCoachReply 带读 / salvageReadingPlan
		// 排读法）：模型 `finish_reason` 报正常，JSON 却停在某一块中间。
		// 这一批尤其疼 —— 她那一屏上每一块都靠它，整批丢掉的结果是三块全空，
		// 而先到的那两块本来是好的。
		//
		// 到齐的留下，没写完的当没给：没拿到引导的那几块照旧摆「获取引导」，
		// 那个按钮本来就在。见 memory: model-json-half-arrived-2026-09-08。
		// 🚨 救援要喂**剥过外壳的 `c`**，不是原始的 `text`。
		//
		// 这一行原来传的是 `text`。模型只要把回复包进 ```json 围栏里，
		// `salvageWritingGuideBatch` 第一个 `dec.Token()` 读到的就是反引号，
		// 直接 return false —— 于是那份**本来救得回来**的回复被整批丢掉，
		// 她那一屏上每一块都是空的，外加一个「后台错误：AI 响应错误」的弹窗。
		//
		// 2026-09-11 的模拟学生走查里这个弹窗挡住了她两次（中英各一次）。
		// 上面那段注释写着「不要整批丢掉」，而这一行让那段注释在带围栏的回复上
		// 一次都没生效过。
		items, okSalvage := salvageWritingGuideBatch(strings.TrimSpace(c))
		if !okSalvage {
			return nil, false
		}
		got.Blocks = items
	}

	out := make(map[uuid.UUID]writingGuideResult, len(got.Blocks))
	for _, item := range got.Blocks {
		id, err := uuid.Parse(strings.TrimSpace(item.ID))
		if err != nil || !knownIDs[id] {
			continue // an id the model invented or mistyped — drop rather than guess which block it meant
		}

		kept := make([]string, 0, len(item.Questions))
		for _, q := range item.Questions {
			q = strings.TrimSpace(q)
			if q == "" {
				continue
			}
			if !strings.HasSuffix(q, "？") && !strings.HasSuffix(q, "?") {
				continue
			}
			kept = append(kept, q)
			if len(kept) == writingGuideMaxQuestions {
				break
			}
		}
		if len(kept) == 0 {
			continue
		}

		keptIDs := make([]string, 0, len(item.MethodIDs))
		for _, mid := range item.MethodIDs {
			mid = strings.TrimSpace(mid)
			if _, known := vocab.ByID(mid); known {
				keptIDs = append(keptIDs, mid)
			}
		}

		out[id] = writingGuideResult{
			Job:       strings.TrimSpace(item.Job),
			MethodIDs: keptIDs,
			Questions: kept,
		}
	}
	return out, true
}

// guideWritingBlock is POST /api/v1/writings/{id}/outline/{oid}/guide — the
// single-block REGENERATE. A spend endpoint (one model call), metered as
// purpose="block_guide".
//
// PERSISTED, deliberately — and this reverses an earlier decision, so the
// reason matters. The old comment argued that storing model prose in the
// writing's own record would let a later reader mistake it for hers. That
// risk is real and is handled by WHERE it is stored: `guide` is a sibling of
// `role` (scaffold), not of `text` (hers) — the separation migration 0100
// established for exactly this. Nothing composes a draft from the outline;
// writing_draft is built from writing_snippet.text alone, and
// TestComposeDraft_NeverIncludesGuideText pins it.
func (a *API) guideWritingBlock(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedWritingAtom(w, r)
	if !ok {
		return
	}
	oid, err := uuid.Parse(r.PathValue("oid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	u, _ := UserFromContext(r.Context())
	entitled, eerr := HasEntitlement(r.Context(), u)
	if eerr != nil {
		httpx.WriteError(w, r, eerr)
		return
	}
	if !entitled {
		httpx.WriteError(w, r, httpx.ErrNotEntitled())
		return
	}

	turnCtx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 150*time.Second)
	defer cancel()

	wr, err := a.d.Queries.GetWriting(turnCtx, at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	siblings, err := a.d.Queries.ListWritingOutline(turnCtx, at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var block sqlc.WritingOutline
	found := false
	for _, s := range siblings {
		if s.ID == oid {
			block, found = s, true
			break
		}
	}
	if !found {
		// Scoped to THIS atom's outline, so a valid uuid belonging to someone
		// else's writing is a 404 here, not a leak.
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}

	snippets, err := a.d.Queries.ListWritingSnippets(turnCtx, at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	existing := ""
	for _, s := range snippets {
		if s.OutlineID.Valid && s.OutlineID.Bytes == oid {
			existing = s.Text
			break
		}
	}
	msgs, err := a.d.Queries.ListAtomMessages(turnCtx, at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	// 整篇上下文：别的段写了什么、她这一块收到过什么意见。读不到就给空，
	// buildWritingGuidePrompt 会退回只列要点。
	guidePiece := ""
	if snippets, serr := a.d.Queries.ListWritingSnippets(turnCtx, at.ID); serr == nil {
		prior, _ := a.d.Queries.ListWritingComments(turnCtx, at.ID)
		guidePiece = buildWritingPieceContext(wr, siblings, snippets, prior, &block)
	}

	// §model-routing · compose. Asking a GOOD question about someone's
	// half-formed argument is the hardest reasoning in this room — harder than
	// the dialogue turn, which only has to respond. compose is where that
	// difference is now spent: a reasoning budget, not the reviewer tier.
	resolved, ok2 := a.route(turnCtx, gateway.ClassCompose)
	if !ok2 {
		slog.Warn("writing block guide: no provider resolved",
			"atom_id", at.ID, "request_id", httpx.RequestIDFromContext(r.Context()))
		httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
		return
	}
	res, cerr := gateway.Collect(turnCtx, a.d.Provider, resolved, gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: writingGuideSystem},
			{Role: gateway.RoleUser, Content: buildWritingGuidePrompt(wr, block, siblings, existing, msgs, guidePiece) +
				writingGuideAnotherAngle(priorWritingGuide(block))},
		},
	})
	a.recordLiteLLMCall(turnCtx, u.ID, at.ID, "block_guide", resolved, res.Usage)
	if cerr != nil {
		slog.Warn("writing block guide: provider call failed", "err", cerr,
			"atom_id", at.ID, "request_id", httpx.RequestIDFromContext(r.Context()))
		httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
		return
	}
	guide, okParse := parseWritingGuide(res.Text)
	if !okParse {
		// 和批量那一路同一件事、同一个修法。2026-09-11 第九轮走查里批量那边
		// 一次都没坏（重试接住了），坏的是这条单块的路 —— 她的原话是
		//「第四段刚才报了个 model_unavailable 的错误，不知道现在按获取引导
		// 能不能正常出来」。这条路上既没有救援也没有重试，模型收错一个括号
		// 就直接弹错。
		//
		// 一次。她正同步等着，而且每次都要花钱；第二次还坏就老实报错。
		slog.Warn("writing block guide: reply unparseable or held no questions",
			"atom_id", at.ID, "request_id", httpx.RequestIDFromContext(r.Context()),
			"stop_reason", res.StopReason, "reply_bytes", len(res.Text),
			"reply_head", headRunes(res.Text, 220), "reply_tail", tailRunes(res.Text, 200))

		res2, cerr2 := gateway.Collect(turnCtx, a.d.Provider, resolved, gateway.ChatRequest{
			Messages: []gateway.ChatMessage{
				{Role: gateway.RoleSystem, Content: writingGuideSystem},
				{Role: gateway.RoleUser, Content: buildWritingGuidePrompt(wr, block, siblings, existing, msgs, guidePiece) +
					writingGuideAnotherAngle(priorWritingGuide(block)) + writingGuideBracketNudge},
			},
		})
		a.recordLiteLLMCall(turnCtx, u.ID, at.ID, "block_guide", resolved, res2.Usage)
		if cerr2 != nil {
			slog.Warn("writing block guide: retry provider call failed", "err", cerr2,
				"atom_id", at.ID, "request_id", httpx.RequestIDFromContext(r.Context()))
			httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
			return
		}
		guide, okParse = parseWritingGuide(res2.Text)
		if !okParse {
			slog.Warn("writing block guide: retry also unparseable",
				"atom_id", at.ID, "request_id", httpx.RequestIDFromContext(r.Context()),
				"stop_reason", res2.StopReason, "reply_bytes", len(res2.Text),
				"reply_head", headRunes(res2.Text, 220), "reply_tail", tailRunes(res2.Text, 200))
			httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
			return
		}
		slog.Info("writing block guide: retry parsed fine", "atom_id", at.ID)
	}

	// 🚨 重新生成 = **再要一个角度**，不是把她读过的那一组扔掉。
	fresh := writingGuideDTOOf(guide)
	// 她已经写了字的那一块最多两条 —— 见 writingGuideQuestionCap。
	if cap := writingGuideQuestionCap(existing); len(fresh.Questions) > cap {
		fresh.Questions = fresh.Questions[:cap]
	}
	dto := writingGuideWithPrevious(fresh, priorWritingGuide(block))
	if payload, merr := json.Marshal(dto); merr != nil {
		slog.Warn("writing block guide: marshal for persistence failed", "err", merr, "atom_id", at.ID, "outline_id", oid)
	} else if serr := a.d.Queries.SetWritingOutlineGuide(turnCtx, sqlc.SetWritingOutlineGuideParams{
		ID: oid, Guide: payload,
	}); serr != nil {
		// Best-effort, same reasoning as relinkWritingSnippetsToOutline
		// (writing_outline.go): a persistence hiccup must not cost her the
		// guidance she just paid a model call for — she still sees it now,
		// she will simply see the previous stored guide (or none) next time
		// she reopens this block until it is regenerated again.
		slog.Warn("writing block guide: persist failed", "err", serr, "atom_id", at.ID, "outline_id", oid)
	}

	httpx.WriteJSON(w, http.StatusOK, dto)
}

// guideWritingBlocks is POST /api/v1/writings/{id}/guide — the batch route
// Task 4 (B1) adds: guide EVERY block in the outline in ONE model call,
// persist each result, so opening 段落 shows guidance for every block from
// the first paint instead of only after she clicks 卡住了？ on one of them.
// A spend endpoint (ONE model call for the whole skeleton), metered as
// purpose="block_guide" — the same purpose as the single-block regenerate;
// it is the same KIND of spend at a different batch size, not a new kind.
//
// IDEMPOTENT ON THE SERVER (2026-08-28). It used to re-guide the WHOLE
// outline on every call, and the only thing standing between a second tab and
// a second full flagship bill was `needsBatch` in SnippetsStage.tsx — a
// client-side guard, which is to say no guard. Now a block that already has a
// stored guide is RETURNED, never regenerated; only blocks lacking one reach
// the model; and if every block already has one, the handler makes no model
// call at all. Opening 段落 is a one-time cost, and every reopen is free.
//
// NO CONCURRENCY GUARD, deliberately — unlike postWritingOpening, which takes
// an advisory lock. The asymmetry is the damage, not the shape: a duplicated
// greeting is permanently visible to her as a room that says hello twice,
// while two tabs racing the first 段落 open produce two valid guides for the
// same block and the last write wins — she sees one coherent guide either
// way, and the only cost is one duplicated call, once, in the single window
// that now exists (the very first entry; every later open is short-circuited
// above). Buying that back would mean holding a pool connection across the
// most expensive call in the room — the flagship, over the whole outline, up
// to 150 s. Pool exhaustion is a worse failure than one extra call.
func (a *API) guideWritingBlocks(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedWritingAtom(w, r)
	if !ok {
		return
	}
	u, _ := UserFromContext(r.Context())

	turnCtx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 150*time.Second)
	defer cancel()

	wr, err := a.d.Queries.GetWriting(turnCtx, at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	blocks, err := a.d.Queries.ListWritingOutline(turnCtx, at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if len(blocks) == 0 {
		// Nothing to guide yet — an empty outline is not a failure, just
		// nothing worth spending a model call on.
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"guides": map[string]writingGuideDTO{}})
		return
	}

	// The stored/missing split. `out` starts as everything already paid for,
	// so the response shape is the same whether the model ran or not: the
	// client always gets a guide for every block that has one.
	out := make(map[string]writingGuideDTO, len(blocks))
	missing := make(map[uuid.UUID]bool, len(blocks))
	for _, b := range blocks {
		if g, has := storedWritingGuide(b); has {
			out[b.ID.String()] = g
			continue
		}
		missing[b.ID] = true
	}
	if len(missing) == 0 {
		// The common case after the first open: zero model calls. Note this
		// returns BEFORE the entitlement gate — reading back guidance she has
		// already paid for is not a new spend, so it must not be gated on
		// having credit left (the same reasoning setWritingSetup states).
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"guides": out})
		return
	}

	entitled, eerr := HasEntitlement(turnCtx, u)
	if eerr != nil {
		httpx.WriteError(w, r, eerr)
		return
	}
	if !entitled {
		httpx.WriteError(w, r, httpx.ErrNotEntitled())
		return
	}

	snippets, err := a.d.Queries.ListWritingSnippets(turnCtx, at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	textByBlock := make(map[uuid.UUID]string, len(snippets))
	for _, s := range snippets {
		if s.OutlineID.Valid {
			textByBlock[uuid.UUID(s.OutlineID.Bytes)] = s.Text
		}
	}
	msgs, err := a.d.Queries.ListAtomMessages(turnCtx, at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	// §model-routing · compose — same reasoning as guideWritingBlock.
	resolved, ok2 := a.route(turnCtx, gateway.ClassCompose)
	if !ok2 {
		slog.Warn("writing block guide batch: no provider resolved",
			"atom_id", at.ID, "request_id", httpx.RequestIDFromContext(r.Context()))
		httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
		return
	}
	res, cerr := gateway.Collect(turnCtx, a.d.Provider, resolved, gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: writingGuideBatchSystem},
			{Role: gateway.RoleUser, Content: buildWritingGuideBatchPrompt(wr, blocks, textByBlock, msgs)},
		},
	})
	a.recordLiteLLMCall(turnCtx, u.ID, at.ID, "block_guide", resolved, res.Usage)
	if cerr != nil {
		slog.Warn("writing block guide batch: provider call failed", "err", cerr,
			"atom_id", at.ID, "request_id", httpx.RequestIDFromContext(r.Context()))
		httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
		return
	}

	// knownIDs is the MISSING set, not every block: a guide the model volunteers
	// for a block that already has one is dropped here rather than persisted,
	// so a stored guide can never be overwritten by this route. Overwriting is
	// what POST /outline/{oid}/guide is for, and only when she asks.
	guides, okParse := parseWritingGuideBatch(res.Text, missing)
	if !okParse {
		// 🚨 把模型到底回了什么记下来。上一版只说一句「unparseable」，而排查一次
		// 模型回复唯一有用的证据就是它写了什么 —— 2026-09-08 这条 502 就是靠
		// 补上 reply_tail 才两分钟定位的（同 reading_plan.go）。
		// 🚨 尾巴不够，**开头也要记**。
		//
		// 整批被丢掉有两种完全不同的原因，而它们要靠开头才分得开：
		//   一 · 前几块是好的，坏在最后一块 —— 那么救援本该捞回前几块，
		//        它没捞到就是救援这条路上出了问题。
		//   二 · **第一块**就坏了 —— 救援一条都捞不到是正确行为，
		//        该改的是提示词或者档位，不是救援。
		//
		// 2026-09-11 第四轮走查抓到的那一条属于第二种：整份回复只有一块，
		// 而它的 questions 数组用 `}` 收的尾（`…收尾？"}]}`）—— 和
		// 2026-09-08 一模一样的形状。stop_reason 是 "stop"，一个字都没少：
		// **「写完了」和「写对了」是两件事。** 只有一块的时候，那一块坏掉就是
		// 整份坏掉，救援无处可救。
		//
		// 🚨 reply_len 记的是**字节**。4693 个字节的中文只有一千六百字上下 ——
		// 我第一次看这条日志时按字数读，于是以为它有四五块，白找了一圈救援的毛病。
		slog.Warn("writing block guide batch: reply unparseable",
			"atom_id", at.ID, "request_id", httpx.RequestIDFromContext(r.Context()),
			"stop_reason", res.StopReason, "reply_bytes", len(res.Text),
			"reply_head", headRunes(res.Text, 220),
			"reply_tail", tailRunes(res.Text, 200))

		// 再要一次。
		//
		// 这不是「多试几次总会好」那种重试 —— 它治的是一个具体的、反复出现的
		// 毛病：模型偶尔把一个括号收错，而这件事和她写了什么没关系，
		// 换一次采样几乎总能过。阅读室那边同一个判断早就这么做了
		// （reading_coach.go：「reply looks broken, retrying once」）。
		//
		// 一次，不是三次。这是学生正等着的一条同步请求，而且每一次都要花钱；
		// 第二次还坏就老老实实报错——她那一屏的「获取引导」按钮本来就在，
		// 不会没有退路。
		res2, cerr2 := gateway.Collect(turnCtx, a.d.Provider, resolved, gateway.ChatRequest{
			Messages: []gateway.ChatMessage{
				{Role: gateway.RoleSystem, Content: writingGuideBatchSystem},
				{Role: gateway.RoleUser, Content: buildWritingGuideBatchPrompt(wr, blocks, textByBlock, msgs) + writingGuideBracketNudge},
			},
		})
		// 打到了通道就要记账，哪怕这一份也读不出来。
		a.recordLiteLLMCall(turnCtx, u.ID, at.ID, "block_guide", resolved, res2.Usage)
		if cerr2 != nil {
			slog.Warn("writing block guide batch: retry provider call failed", "err", cerr2,
				"atom_id", at.ID, "request_id", httpx.RequestIDFromContext(r.Context()))
			httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
			return
		}
		guides, okParse = parseWritingGuideBatch(res2.Text, missing)
		if !okParse {
			slog.Warn("writing block guide batch: retry also unparseable",
				"atom_id", at.ID, "request_id", httpx.RequestIDFromContext(r.Context()),
				"stop_reason", res2.StopReason, "reply_bytes", len(res2.Text),
				"reply_head", headRunes(res2.Text, 220),
				"reply_tail", tailRunes(res2.Text, 200))
			httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
			return
		}
		slog.Info("writing block guide batch: retry parsed fine",
			"atom_id", at.ID, "blocks", len(guides))
	}

	for id, g := range guides {
		dto := writingGuideDTOOf(g)
		payload, merr := json.Marshal(dto)
		if merr != nil {
			slog.Warn("writing block guide batch: marshal for persistence failed", "err", merr, "atom_id", at.ID, "outline_id", id)
			continue
		}
		if serr := a.d.Queries.SetWritingOutlineGuide(turnCtx, sqlc.SetWritingOutlineGuideParams{
			ID: id, Guide: payload,
		}); serr != nil {
			slog.Warn("writing block guide batch: persist failed", "err", serr, "atom_id", at.ID, "outline_id", id)
			continue
		}
		out[id.String()] = dto
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"guides": out})
}
