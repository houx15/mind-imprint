package api

// reading_coach.go — 带读：由 印记 领着走的阅读。
//
// 2026-08-27 的第二轮裁定，推翻了同一天早些时候那个「任务清单 + 复选框」：
//
//   > merge the tasks with the AI bar. at start the AI begins with 让我来带你
//   > 详细阅读这篇文章吧, clicks 开始. then AI generates the plan, introduces
//   > the plan, then we will enter a stage directly. student doesn't handle the
//   > stages themselves, but the AI directs these.
//
// 所以这一版里，**学生不再管理阶段**。她不点「做完了」，也不点「跳过」——她
// 只是读、只是回答。是否往下走、走到哪一步，由 印记 判断。清单还在库里（它是
// 过程证据），但它不再是一张要她操作的表，而是 印记 手里的教案。
//
// ## 三条硬规则
//
//  1. **一次只领一步。** 每一轮只说当前这一步要做什么，说完就停。铁律③。
//  2. **不替她读。** 带读的话里不能出现这篇文章的结论、答案、主旨。她还没读
//     呢——把答案先说了，后面每一步都成了走过场。
//  3. **推进由模型判断，但只能往前一步。** 一轮最多推进一步：一次跳三步等于
//     替她把整篇读完了。
//
// 跳过没有消失，只是不再是一颗按钮：她说「这步跳过吧」，模型把它标成 skipped
// 再往下走。跳过依然被记录（铁律④），只是现在记录的是一句她真说过的话。

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// readingCoachTurnsWindow bounds the transcript fed to a guided turn. Same
// reasoning as everywhere else in lite: no compaction layer, so this window is
// the only thing bounding prompt growth.
const readingCoachTurnsWindow = 14

const readingCoachSystem = `你是「印记」，正在**带着**一个中学生读一篇文章。你是领读的人，不是答疑的人。

下面会给你：这篇文章按段落的全文、你为她排的读法清单（每一步的状态）、你们刚才聊的话，以及她刚说的话。

## 你怎么带

- **一次只领一步。** 说清楚当前这一步要她做什么，说完就停，等她。不要一口气讲两步。
- 说话要短。不超过 120 个字。
- 要具体到这篇文章：不要说「精读重点段」，要说「往下翻到第二段，那段里有三个数字，先把它们圈出来」。
- 她答完一步之后，先接住她说的（一句就够），再领下一步。
- 她问问题的时候先回答她，回答完再把她带回当前这一步。

## 你绝对不能做的事

- **不要替她读。** 不要说出这篇文章的结论、主旨、答案、要点总结。她还没读呢——你先说了，后面每一步都成了走过场。
- 不要一次问好几个问题。
- 不要催她、不要评价她读得快慢。
- 不要说"作为AI"、不要空夸。

## 什么时候往下走

- 她确实做完了当前这一步（哪怕做得粗糙）→ advance 给 "done"。
- 她说想跳过、说这步没意思、说她已经会了 → advance 给 "skipped"。**不要劝她**。
- 她还没做、或者答得完全没碰到这一步要她做的事 → advance 给 ""，留在原地，把这一步再说一遍（换个说法，别重复原话）。
- 一轮最多往前一步。

## 输出格式

只输出一个 JSON 对象：

{"reply":"你要对她说的话","advance":"","focusBlock":""}

- advance：""（留在当前步）/ "done"（当前步完成）/ "skipped"（她想跳过当前步）。
- focusBlock：如果这一步要她看某一段，给出段落编号（b1/b2/…）；否则留空。必须是真实存在的段落。

不要输出对象以外的任何文字或代码块标记。`

func buildReadingCoachPrompt(
	title string,
	blocks []Block,
	tasks []sqlc.ReadingTask,
	msgs []sqlc.AtomMessage,
	studentText string,
) string {
	var b strings.Builder
	if t := strings.TrimSpace(title); t != "" {
		b.WriteString("文章标题：" + t + "\n")
	}

	b.WriteString("\n【文章，按段落】\n")
	total := 0
	for _, blk := range blocks {
		text := strings.TrimSpace(blk.Text)
		if text == "" {
			continue
		}
		runes := []rune(text)
		if total+len(runes) > readingPlanArticleRuneBudget {
			b.WriteString(blk.ID + "：（这一段没放进来，但它存在）\n")
			continue
		}
		total += len(runes)
		b.WriteString(blk.ID + "：" + text + "\n")
	}

	b.WriteString("\n【你排的读法】\n")
	current := currentReadingTask(tasks)
	for _, t := range tasks {
		mark := "待办"
		switch t.Status {
		case "done":
			mark = "已完成"
		case "skipped":
			mark = "已跳过"
		}
		line := "- [" + mark + "] " + t.Label
		if t.Detail != "" {
			line += "：" + t.Detail
		}
		if t.BlockID != "" {
			line += "（这一步看 " + t.BlockID + "）"
		}
		if current != nil && t.ID == current.ID {
			line += "   ← **她现在在这一步**"
		}
		b.WriteString(line + "\n")
	}
	if current == nil {
		b.WriteString("\n所有步骤都走完了。跟她说一句收尾的话，别再领新的一步。\n")
	}

	b.WriteString("\n【你们刚才聊的】\n")
	tail := msgs
	if len(tail) > readingCoachTurnsWindow {
		tail = tail[len(tail)-readingCoachTurnsWindow:]
	}
	any := false
	for _, m := range tail {
		var who string
		switch m.Role {
		case "student":
			who = "她"
		case "ai":
			who = "你"
		default:
			continue
		}
		if s := strings.TrimSpace(m.Content); s != "" {
			b.WriteString(who + "：" + s + "\n")
			any = true
		}
	}
	if !any {
		b.WriteString("（还没聊过。）\n")
	}

	if studentText != "" {
		b.WriteString("\n【她刚刚说的】\n" + studentText + "\n")
	} else {
		b.WriteString("\n【她刚刚说的】\n（她刚点了「开始」，还没说话。介绍一下你排的读法，然后领她进第一步。）\n")
	}
	return b.String()
}

// currentReadingTask is the first step not yet settled. Nil when everything is
// done or skipped — the state where the coach stops leading rather than
// inventing a step to fill the silence.
func currentReadingTask(tasks []sqlc.ReadingTask) *sqlc.ReadingTask {
	for i := range tasks {
		if tasks[i].Status == "pending" {
			return &tasks[i]
		}
	}
	return nil
}

type readingCoachReply struct {
	Reply      string `json:"reply"`
	Advance    string `json:"advance"`
	FocusBlock string `json:"focusBlock"`
}

func parseReadingCoachReply(text string, valid map[string]bool) (readingCoachReply, bool) {
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
	var got readingCoachReply
	if err := json.Unmarshal([]byte(strings.TrimSpace(c)), &got); err != nil {
		return readingCoachReply{}, false
	}
	got.Reply = strings.TrimSpace(got.Reply)
	if got.Reply == "" {
		return readingCoachReply{}, false
	}
	// Only the two advances the contract names. Anything else — including a
	// model trying to jump several steps by inventing a value — leaves her
	// exactly where she is, which is the safe direction to fail in.
	if got.Advance != "done" && got.Advance != "skipped" {
		got.Advance = ""
	}
	if got.FocusBlock != "" && !valid[got.FocusBlock] {
		got.FocusBlock = ""
	}
	return got, true
}

// postReadingCoachTurn is POST /api/v1/readings/{id}/coach.
//
// One guided turn. `text` empty means she pressed 开始 — the coach introduces
// the plan and leads her into step one. A plan is generated on demand if there
// isn't one yet, so 开始 is genuinely the only button she needs.
func (a *API) postReadingCoachTurn(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedReadingAtom(w, r)
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

	var req struct {
		Text string `json:"text"`
	}
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	studentText := strings.TrimSpace(req.Text)

	src, err := a.d.Queries.GetReadingSource(r.Context(), at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	blocks := SplitBlocks(src.Body)
	if len(blocks) == 0 {
		httpx.WriteError(w, r, httpx.ErrBadRequest("empty_article", "这篇还没有正文，先把文章贴进来。", nil))
		return
	}

	turnCtx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 150*time.Second)
	defer cancel()

	// A plan on demand: 开始 is the only button, so pressing it with no plan
	// yet must produce one rather than refusing. This is the ONLY caller that
	// plans implicitly — the explicit endpoint stays for 重排.
	tasks, err := a.d.Queries.ListReadingTasks(turnCtx, at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if len(tasks) == 0 {
		if tasks, err = a.planReadingTasks(turnCtx, u.ID, at.ID, src, blocks); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
	}

	msgs, err := a.d.Queries.ListAtomMessages(turnCtx, at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	// §model-routing: leading someone through a text — deciding whether what
	// she just said actually counts as having done this step — is judgement,
	// not conversation. Flagship, like the planning call.
	resolved, okResolve := a.resolveEval(turnCtx)
	if !okResolve {
		slog.Warn("reading coach: no provider resolved",
			"atom_id", at.ID, "request_id", httpx.RequestIDFromContext(r.Context()))
		httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
		return
	}
	res, cerr := gateway.Collect(turnCtx, a.d.Provider, resolved, gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: readingCoachSystem},
			{Role: gateway.RoleUser, Content: buildReadingCoachPrompt(src.Title, blocks, tasks, msgs, studentText)},
		},
	})
	a.recordLiteLLMCall(turnCtx, u.ID, at.ID, "reading_coach", resolved, res.Usage)
	if cerr != nil {
		slog.Warn("reading coach: provider call failed", "err", cerr,
			"atom_id", at.ID, "request_id", httpx.RequestIDFromContext(r.Context()))
		httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
		return
	}
	valid := make(map[string]bool, len(blocks))
	for _, blk := range blocks {
		valid[blk.ID] = true
	}
	parsed, okParse := parseReadingCoachReply(res.Text, valid)
	if !okParse {
		slog.Warn("reading coach: reply unparseable",
			"atom_id", at.ID, "request_id", httpx.RequestIDFromContext(r.Context()))
		httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
		return
	}

	// The student turn, the coach turn, and the step advance land together.
	// Split, a crash between them leaves the transcript saying one thing and
	// the plan another — and the plan is what the next turn reads to decide
	// where she is.
	tx, err := a.d.Pool.Begin(turnCtx)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer func() { _ = tx.Rollback(turnCtx) }()
	qtx := a.d.Queries.WithTx(tx)

	seq, err := qtx.NextAtomMessageSeq(turnCtx, at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if studentText != "" {
		if _, err := qtx.AppendAtomMessage(turnCtx, sqlc.AppendAtomMessageParams{
			AtomID: at.ID, Seq: seq, Role: "student", Content: studentText,
		}); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		seq++
	}
	if _, err := qtx.AppendAtomMessage(turnCtx, sqlc.AppendAtomMessageParams{
		AtomID: at.ID, Seq: seq, Role: "ai", Content: parsed.Reply,
	}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	if current := currentReadingTask(tasks); current != nil && parsed.Advance != "" {
		if _, err := qtx.SetReadingTaskStatus(turnCtx, sqlc.SetReadingTaskStatusParams{
			AtomID: at.ID, ID: current.ID, Status: parsed.Advance,
		}); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
	}
	if err := tx.Commit(turnCtx); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	after, err := a.d.Queries.ListReadingTasks(r.Context(), at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	next := currentReadingTask(after)
	currentID := ""
	focus := parsed.FocusBlock
	if next != nil {
		currentID = next.ID.String()
		// A step that names its own paragraph wins over the model's guess: the
		// plan already decided which paragraph this step is about, and letting
		// a per-turn guess override it would scroll her somewhere the step
		// never meant.
		if next.BlockID != "" {
			focus = next.BlockID
		}
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"reply":         parsed.Reply,
		"tasks":         readingTaskDTOs(after),
		"currentTaskId": currentID,
		"focusBlock":    focus,
		"finished":      next == nil,
	})
}
