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
	"strconv"
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

// writingGuideTeachingRules is Task 3's four-part "怎么说话" doctrine
// (writing_plan.go's writingPlanSystem), copied verbatim rather than
// re-derived: an engineer reading these tasks out of order must not find two
// different versions of how 印记 is supposed to talk. Shared by the
// single-block and batch guide prompts below so the two paths can never
// drift apart on tone.
//
// 🔑 下面示范句里的「留悬念、设问、开门见山」是 methods.json 里逐字存在的三个
// name（opening_suspense / opening_question / opening_direct）。任何写进提示词
// 散文里的方法名都必须这样对得上 packages/contracts/vocab/methods.json——示范里
// 出现一个库里没有的名字，就是在教模型造词，而下一段恰好在禁止它造词。
const writingGuideTeachingRules = `## 怎么说话（这条比什么都重要）

你是老师，不是问答机器。每次开口都要做到四件事：
① 说清这件事为什么重要；② 说出真正的方法名（下面【可用的方法】里的，别自己造词）；
③ 给她一个真的选择，她也可以不选；④ 主动提出可以举例子一起看。

不要这样说：「有人会从一个具体场景切进去，有人直接抛个问题。你这篇你想怎么进？」
要这样说：「对于一篇文章来说，有意思的开头非常重要。留悬念、设问、开门见山等，
都是常见的方式。你想尝试哪一种？或者需要我给几个具体的案例我们一起来学习一下
这几种方法吗？」

一次仍然只问**一个**问题——「一次只问一个」说的是问题的数量，从来不是让你少说话。`

// writingGuideQuestionRules is the content discipline for `questions`,
// shared by the single-block and batch prompts for the same
// never-drift-apart reason as writingGuideTeachingRules.
const writingGuideQuestionRules = `关于问题本身：
- 必须是问题，不是建议，也不是示范。每一条都以问号结尾。
- 要**具体到能马上动笔**。「你的论点是什么？」太空；「你身边有没有哪个同学因为这件事吃过亏？」才有用。
- 要贴着这一块的作用来问，不要每一块都问同样的话。
- 要贴着她已经说过的话来问，用她提到过的人、事、场景，不要另起炉灶。
- 一条一个问题，不要在一条里塞两问。

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
- 【整篇的结构】里给出的每一块都要出现一次，id 逐字取自那里给出的 id。
- job / method_ids / questions 的要求和上面完全一样。

只输出一个 JSON 对象，不要输出对象以外的任何文字或代码块标记。`

// writingGuideAppliesTo maps a block's free-text role (her own outline
// label, or the skeleton's generic name) to one of vocab's three position
// buckets. Matched by substring against a handful of Chinese synonyms — the
// same heuristic, and the same narrow failure mode, as rootInsertPosition
// (writing_plan.go): a role matching neither list falls through to "body",
// which is the right default because most blocks in a piece ARE body
// paragraphs.
func writingGuideAppliesTo(role string) string {
	for _, kw := range []string{"开头", "引言", "开篇", "钩子", "导入"} {
		if strings.Contains(role, kw) {
			return "opening"
		}
	}
	for _, kw := range []string{"结尾", "结论", "收尾", "总结"} {
		if strings.Contains(role, kw) {
			return "closing"
		}
	}
	return "body"
}

// buildWritingGuidePrompt assembles what the model sees for ONE block: which
// block it is (role + her own heading text), the skeleton it sits in, what she
// has already drafted there, her material, and the methods usable at this
// position.
func buildWritingGuidePrompt(wr sqlc.Writing, block sqlc.WritingOutline, siblings []sqlc.WritingOutline, existing string, msgs []sqlc.AtomMessage) string {
	var b strings.Builder
	if t := strings.TrimSpace(wr.Title); t != "" {
		b.WriteString("题目/想法：" + t + "\n")
	}
	b.WriteString("写作语言：" + wr.Lang + "\n")
	if wr.TargetWords != nil {
		b.WriteString("目标篇幅：约 " + strconv.Itoa(int(*wr.TargetWords)) + " 字\n")
	}

	// There is no template name to report any more: the structure is not
	// chosen from a library, it is grown out of her own planning conversation
	// (writing_plan.go). The sibling blocks below already carry everything
	// this prompt needs to know about the shape she ended up with — and they
	// carry it in HER words rather than as a category name.

	// The sibling blocks matter: a question for 「你的回应」 is only good if it
	// knows what she put in 「反方最强的说法」. Without them the model asks the
	// same generic question in every block.
	b.WriteString("\n【整篇的结构，以及每一块她自己写下的要点】\n")
	for _, s := range siblings {
		line := "- " + s.Role
		if s.Role == "" {
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

	b.WriteString("\n【她现在停住的这一块】\n")
	b.WriteString("这一块的作用：" + block.Role + "\n")
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
	any := false
	for _, m := range msgs {
		if m.Role != "student" {
			continue
		}
		if s := strings.TrimSpace(m.Content); s != "" {
			b.WriteString("- " + s + "\n")
			any = true
		}
	}
	if !any {
		b.WriteString("（她还没在对话里说过什么。）\n")
	}

	b.WriteString("\n【可用的方法】（只能用这里的 id，不要自己编）\n")
	for _, m := range vocab.For(writingGuideAppliesTo(block.Role)) {
		b.WriteString("- id=" + m.ID + " · " + m.Name + "：" + m.Definition + "\n")
	}
	return b.String()
}

// buildWritingGuideBatchPrompt assembles what the model sees for the WHOLE
// outline at once: every block (id, role, her heading text, what she has
// already drafted there, if anything), her material, and the full method
// library annotated with where each one applies (same "list everything,
// annotate applies_to" shape buildWritingPlanPrompt already uses) — a single
// block's narrower vocab.For(...) filter does not fit here because one call
// has to serve blocks at every position at once.
func buildWritingGuideBatchPrompt(wr sqlc.Writing, blocks []sqlc.WritingOutline, textByBlock map[uuid.UUID]string, msgs []sqlc.AtomMessage) string {
	var b strings.Builder
	if t := strings.TrimSpace(wr.Title); t != "" {
		b.WriteString("题目/想法：" + t + "\n")
	}
	b.WriteString("写作语言：" + wr.Lang + "\n")
	if wr.TargetWords != nil {
		b.WriteString("目标篇幅：约 " + strconv.Itoa(int(*wr.TargetWords)) + " 字\n")
	}

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
	}

	b.WriteString("\n【她在对话里说过的话】\n")
	any := false
	for _, m := range msgs {
		if m.Role != "student" {
			continue
		}
		if s := strings.TrimSpace(m.Content); s != "" {
			b.WriteString("- " + s + "\n")
			any = true
		}
	}
	if !any {
		b.WriteString("（她还没在对话里说过什么。）\n")
	}

	b.WriteString("\n【可用的方法】（只能用这里的 id，不要自己编）\n")
	for _, m := range vocab.All() {
		b.WriteString("- id=" + m.ID + " · " + m.Name + "（" + m.AppliesTo + "）：" + m.Definition + "\n")
	}
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
type writingGuideMethodDTO struct {
	Name       string          `json:"name"`
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
			Name: m.Name, Definition: m.Definition, Examples: m.Examples, Patterns: m.Patterns,
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
		return nil, false
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

	// §model-routing: asking a GOOD question about someone's half-formed
	// argument is the hardest reasoning in this room — harder than the coach
	// turn, which only has to respond. This is the one place in the writing
	// scaffold that resolves the flagship.
	resolved, ok2 := a.resolveEval(turnCtx)
	if !ok2 {
		slog.Warn("writing block guide: no provider resolved",
			"atom_id", at.ID, "request_id", httpx.RequestIDFromContext(r.Context()))
		httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
		return
	}
	res, cerr := gateway.Collect(turnCtx, a.d.Provider, resolved, gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: writingGuideSystem},
			{Role: gateway.RoleUser, Content: buildWritingGuidePrompt(wr, block, siblings, existing, msgs)},
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
		slog.Warn("writing block guide: reply unparseable or held no questions",
			"atom_id", at.ID, "request_id", httpx.RequestIDFromContext(r.Context()))
		httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
		return
	}

	dto := writingGuideDTOOf(guide)
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
func (a *API) guideWritingBlocks(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedWritingAtom(w, r)
	if !ok {
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

	// §model-routing: same reasoning as guideWritingBlock — resolves the
	// flagship, not the ordinary chat chaperone.
	resolved, ok2 := a.resolveEval(turnCtx)
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

	knownIDs := make(map[uuid.UUID]bool, len(blocks))
	for _, b := range blocks {
		knownIDs[b.ID] = true
	}
	guides, okParse := parseWritingGuideBatch(res.Text, knownIDs)
	if !okParse {
		slog.Warn("writing block guide batch: reply unparseable",
			"atom_id", at.ID, "request_id", httpx.RequestIDFromContext(r.Context()))
		httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
		return
	}

	out := make(map[string]writingGuideDTO, len(guides))
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
