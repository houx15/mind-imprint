package api

// reading_block.go — 段落工具：点开一段，把它拆给她看。
//
// 产品的原话：「for lite-level students, they first need to be taught about the
// paragraphs. for english reading materials, they need — after click a
// paragraph, they can click 翻译、关键单词讲解、语法讲解、写作解析; chinese
// paragraph — 成语/修辞运用、案例、结构解析。then we can guide students to
// focus on information/subject lens then.」
//
// 那个 then 是重点：**先能读懂一段，才谈得上用透镜读一篇**。学科透镜这个房间
// 一直有，缺的是它下面那一级台阶。
//
// ## 铁律 检查
//
// 这些工具是**讲解**，这正是它们安全的原因。铁律① 禁的是 AI 替学生写**她自己的**
// 文字。讲解一段**别人已经发表的**文章，是老师干的事；不给，只会让房间更没用，
// 不会让它更诚实。
//
// 真正要守的那条线：每个工具都锚在**文章的** blockId 上，并且这个文件里没有
// 任何一条写入学生自己字段（摘要 / 批注 / takeaway）的路径。是结构上没有，不是
// prompt 里承诺没有。
//
// ## 缓存
//
// 一段的翻译不会变。所以按 (blockId, tool) 存一份重放：第二次点开是瞬时的，
// 也不会重复付钱。这也是它读起来像工具而不像聊天的原因之一。

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// readingBlockContextRunes bounds how much surrounding article rides along.
// A paragraph's JOB ("这一段在整篇里在干什么") cannot be judged from the
// paragraph alone — 结构解析 and 写作解析 are both about position — so the
// neighbours travel with it, bounded.
const readingBlockContextRunes = 1200

const readingBlockSystem = `你是「印记」，正在给一个中学生讲解她点开的**这一段**。

你只讲这一段。上下文给你，是为了让你知道这一段在整篇里的位置，不是让你去讲整篇。

共同的规矩：
- 直接讲，不要「好的」「让我们来看看」这类开场白。
- 讲给一个中学生听：把话说清楚，不要用他没学过的术语；非用不可就顺手解释一句。
- 不超过 200 个字。讲不满不要凑。
- 用中文讲解（哪怕原文是英文）。

这一次要做的是：`

func buildReadingBlockPrompt(title string, blocks []Block, idx int) string {
	var b strings.Builder
	if t := strings.TrimSpace(title); t != "" {
		b.WriteString("文章标题：" + t + "\n")
	}
	b.WriteString("\n【要讲解的这一段】\n" + strings.TrimSpace(blocks[idx].Text) + "\n")

	// Neighbours, nearest first, until the budget runs out. Position is what
	// 结构解析 / 写作解析 are ABOUT, so a paragraph with no neighbours would
	// make those two tools guess.
	var before, after []string
	budget := readingBlockContextRunes
	for step := 1; budget > 0 && (idx-step >= 0 || idx+step < len(blocks)); step++ {
		if i := idx - step; i >= 0 {
			t := strings.TrimSpace(blocks[i].Text)
			if n := len([]rune(t)); n > 0 && n <= budget {
				before = append([]string{t}, before...)
				budget -= n
			}
		}
		if i := idx + step; i < len(blocks) {
			t := strings.TrimSpace(blocks[i].Text)
			if n := len([]rune(t)); n > 0 && n <= budget {
				after = append(after, t)
				budget -= n
			}
		}
	}
	if len(before) > 0 {
		b.WriteString("\n【它前面的内容（只作参考，不要讲解）】\n" + strings.Join(before, "\n") + "\n")
	} else {
		b.WriteString("\n（这是文章的第一段。）\n")
	}
	if len(after) > 0 {
		b.WriteString("\n【它后面的内容（只作参考，不要讲解）】\n" + strings.Join(after, "\n") + "\n")
	} else {
		b.WriteString("\n（这是文章的最后一段。）\n")
	}
	return b.String()
}

// listReadingBlockTools is GET /api/v1/readings/{id}/blocks/tools.
//
// Which tools exist depends on the ARTICLE's language, not on a setting: the
// student already said what she wants to read by pasting it. Serving the list
// (rather than hardcoding it in the frontend) is what keeps the buttons she
// sees and the ids the server accepts from drifting apart.
func (a *API) listReadingBlockTools(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedReadingAtom(w, r)
	if !ok {
		return
	}
	src, err := a.d.Queries.GetReadingSource(r.Context(), at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	lang := readingLangOf(src.Body)
	tools := readingBlockToolsFor(lang)
	out := make([]map[string]string, 0, len(tools))
	for _, t := range tools {
		out = append(out, map[string]string{"id": t.ID, "label": t.Label})
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"lang": lang, "tools": out})
}

// listReadingBlockNotes is GET /api/v1/readings/{id}/blocks/notes — every
// explanation she has already opened, so a reload restores them instead of
// making her pay for them twice.
func (a *API) listReadingBlockNotes(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedReadingAtom(w, r)
	if !ok {
		return
	}
	rows, err := a.d.Queries.ListReadingBlockNotes(r.Context(), at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := make([]map[string]string, 0, len(rows))
	for _, row := range rows {
		out = append(out, map[string]string{"blockId": row.BlockID, "tool": row.Tool, "body": row.Body})
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"notes": out})
}

// explainReadingBlock is POST /api/v1/readings/{id}/blocks/{bid}/explain.
//
// Cached by (blockId, tool): a replay costs nothing and returns instantly, so
// the entitlement gate and the model call are both SKIPPED on that path —
// gating a replay would charge her for reading something she already has.
func (a *API) explainReadingBlock(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedReadingAtom(w, r)
	if !ok {
		return
	}
	blockID := r.PathValue("bid")
	var req struct {
		Tool string `json:"tool"`
	}
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	tool, found := findReadingBlockTool(strings.TrimSpace(req.Tool))
	if !found {
		httpx.WriteError(w, r, httpx.ErrBadRequest("unknown_tool", "这不是可用的段落工具。", nil))
		return
	}

	// Replay first — before the entitlement gate, before any model call.
	if row, err := a.d.Queries.GetReadingBlockNote(r.Context(), sqlc.GetReadingBlockNoteParams{
		AtomID: at.ID, BlockID: blockID, Tool: tool.ID,
	}); err == nil {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{
			"blockId": blockID, "tool": tool.ID, "body": row.Body, "cached": true,
		})
		return
	} else if !errors.Is(err, pgx.ErrNoRows) {
		httpx.WriteError(w, r, err)
		return
	}

	src, err := a.d.Queries.GetReadingSource(r.Context(), at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	blocks := SplitBlocks(src.Body)
	idx := -1
	for i, blk := range blocks {
		if blk.ID == blockID {
			idx = i
			break
		}
	}
	if idx < 0 {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	// A tool from the other language set is a mis-click, not a preference:
	// 语法讲解 on a Chinese paragraph would produce something confidently
	// useless. Refuse rather than spend.
	if tool.Lang != readingLangOf(src.Body) {
		httpx.WriteError(w, r, httpx.ErrBadRequest("tool_language_mismatch", "这个工具不适用于这篇文章。", nil))
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

	// §model-routing: explaining a paragraph is explanation, not judgement, and
	// it is the most frequently-clicked call in the room. Chaperone tier —
	// the flagship is reserved for the planning call that has to decide which
	// paragraph matters.
	resolved, rerr := a.d.ChatResolver(turnCtx)
	if rerr != nil {
		slog.Warn("reading block explain: resolve model failed", "err", rerr,
			"atom_id", at.ID, "request_id", httpx.RequestIDFromContext(r.Context()))
		httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
		return
	}
	res, cerr := gateway.Collect(turnCtx, a.d.Provider, resolved, gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: readingBlockSystem + tool.Instruction},
			{Role: gateway.RoleUser, Content: buildReadingBlockPrompt(src.Title, blocks, idx)},
		},
	})
	a.recordLiteLLMCall(turnCtx, u.ID, at.ID, "block_"+tool.ID, resolved, res.Usage)
	body := strings.TrimSpace(res.Text)
	if cerr != nil || body == "" {
		slog.Warn("reading block explain: model call failed", "err", cerr,
			"atom_id", at.ID, "tool", tool.ID, "request_id", httpx.RequestIDFromContext(r.Context()))
		httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
		return
	}

	if _, err := a.d.Queries.InsertReadingBlockNote(turnCtx, sqlc.InsertReadingBlockNoteParams{
		AtomID: at.ID, BlockID: blockID, Tool: tool.ID, Body: body,
	}); err != nil {
		// Failing to CACHE must not cost her the explanation she just paid
		// for — it is already in hand, so serve it and let the next click pay
		// again rather than turning a storage hiccup into a 500.
		slog.Warn("reading block explain: cache write failed", "err", err, "atom_id", at.ID)
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"blockId": blockID, "tool": tool.ID, "body": body, "cached": false,
	})
}
