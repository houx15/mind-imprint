package api

// writing_guide.go — 引导框：这一步的核心。
//
// 产品的原话：「snippets 很重要，关键是 AI 生成的引导框，而不是让学生一段
// 一段地写。」旧的段落页就是后者——一个空 textarea 顶着一个提纲标题，学生
// 盯着光标发呆。引导框把那个空白换成**一组针对这一块的问题**。
//
// 为什么输出是问题、而且只能是问题：
//
//   - 问题不可能被抄进作文。一句示范段落可以整段粘过去，一个「你自己身上
//     有没有发生过类似的事？」不行。铁律① 在这里不是靠 prompt 恳求模型克制，
//     而是靠**输出类型**本身就装不下正文。
//   - 问题把「想不出来」拆成「答不上来的是哪一问」。学生卡住时需要的不是
//     更大的空白，是更小的问题。
//
// 它另外允许模型提名一张工具卡（cardId），但**只提名，不打开**——前端把它
// 渲染成一句「要不要用《让步段》拆一下？」，由学生点了才召唤。这正是常驻
// 卡片架被撤掉之后，卡片重新出现的唯一入口：需要时才出现，不需要时不占地方。

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// writingGuideMaxQuestions caps the list. Four is already a lot to hold in
// mind at once; more reads as a worksheet, and a worksheet is the thing this
// is trying not to be.
const writingGuideMaxQuestions = 4

// writingGuideSystem. The card vocabulary is injected rather than hardcoded
// in the prompt text so the deck and the prompt cannot drift (writingDeckIDs,
// writing_lens.go).
const writingGuideSystem = `你是「印记」。学生正在写一篇文章，现在停在其中**一块**上，不知道该写什么。

你要做的**只有一件事**：针对这一块，给她 2 到 4 个能帮她想下去的问题。

关于这些问题：
- 必须是问题，不是建议，也不是示范。每一条都以问号结尾。
- 要**具体到能马上动笔**。「你的论点是什么？」太空；「你身边有没有哪个同学因为这件事吃过亏？」才有用。
- 要贴着这一块的作用来问。如果这一块是「反方最强的说法」，就去问她想象中的反对者会怎么说，而不是问她自己的立场。
- 要贴着她已经说过的话来问，用她提到过的人、事、场景，不要另起炉灶。
- 一条一个问题，不要在一条里塞两问。

绝对禁止：
- **不要写出任何可以直接放进她文章里的句子。** 不给论点、不给开头、不给例句、不给示范段落。一个字都不行。
- 不要替她判断对错，不要说「你应该主张……」。
- 不要重复她已经写在这一块里的内容。

你还可以（不是必须）提名一张工具卡，如果这一块正好适合用它拆开来想。可用的卡只有这几张，cardId 必须逐字取自其中：
%s

只输出一个 JSON 对象：
{"questions":["...","..."],"cardId":"","cardReason":""}
cardId 不提名就留空字符串。不要输出对象以外的任何文字或代码块标记。`

// buildWritingGuideCardMenu renders the writing deck for the prompt, from the
// same writingDeckIDs the summon endpoint validates against.
func buildWritingGuideCardMenu() string {
	var b strings.Builder
	for _, id := range writingDeckIDs {
		spec, ok := cards.ByID(id)
		if !ok {
			continue
		}
		b.WriteString("- cardId=" + id + " · " + spec.Name + "：" + spec.Purpose + "\n")
	}
	if b.Len() == 0 {
		return "（这次没有可提名的工具卡，cardId 留空。）"
	}
	return b.String()
}

// buildWritingGuidePrompt assembles what the model sees for ONE block: which
// block it is (role + her own heading text), the skeleton it sits in, what she
// has already drafted there, and her material.
func buildWritingGuidePrompt(wr sqlc.Writing, block sqlc.WritingOutline, siblings []sqlc.WritingOutline, existing string, msgs []sqlc.AtomMessage) string {
	var b strings.Builder
	if t := strings.TrimSpace(wr.Title); t != "" {
		b.WriteString("题目/想法：" + t + "\n")
	}
	b.WriteString("写作语言：" + wr.Lang + "\n")
	if wr.TargetWords != nil {
		b.WriteString("目标篇幅：约 " + strconv.Itoa(int(*wr.TargetWords)) + " 字\n")
	}

	if st, ok := findWritingStructure(wr.StructureKey); ok {
		b.WriteString("她选的结构：" + st.Name + "\n")
	}

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
	return b.String()
}

// writingGuideResult is the wire shape.
type writingGuideResult struct {
	Questions  []string `json:"questions"`
	CardID     string   `json:"cardId"`
	CardReason string   `json:"cardReason"`
}

// parseWritingGuide decodes and HARD-FILTERS the model's reply.
//
// The filtering is the security boundary, not a nicety. Two rules:
//
//   - Every entry must end in a question mark (either script's). A model that
//     slips a declarative sentence — "你可以写：手机让人分心" — into the list
//     has just handed her a sentence for her essay, which is the one thing
//     this endpoint exists to make impossible. Dropping non-questions is
//     cheaper and far more reliable than asking the prompt again.
//   - cardId must be in the writing deck, or it is cleared. An unresolvable
//     card would render as an offer that dead-ends on click.
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

	if got.CardID != "" && !inWritingDeck(got.CardID) {
		got.CardID = ""
		got.CardReason = ""
	}
	got.CardReason = strings.TrimSpace(got.CardReason)
	return got, true
}

// guideWritingBlock is POST /api/v1/writings/{id}/outline/{oid}/guide.
// A spend endpoint (one model call), metered as purpose="block_guide".
//
// PERSISTS NOTHING. The questions are scaffolding she reads and answers in
// her own words; storing them would put model prose inside the writing's own
// record, where a later reader could mistake it for hers.
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
	system := strings.Replace(writingGuideSystem, "%s", buildWritingGuideCardMenu(), 1)
	res, cerr := gateway.Collect(turnCtx, a.d.Provider, resolved, gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: system},
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
	httpx.WriteJSON(w, http.StatusOK, guide)
}
