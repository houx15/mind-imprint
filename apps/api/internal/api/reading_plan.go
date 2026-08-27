package api

// reading_plan.go — 任务清单：印记 给一篇文章排出的读法。
//
// 产品的原话：「an intelligent reading coach would generate a task list after
// students giving a paragraph.」
//
// 模型在这条链路上做的是**挑选和调参**，不是自由编任务：
//
//   - 挑哪一套 routine（reading_routines.go 里写死的四套之一）
//   - 指出哪一两段是重点（focus_block 的 blockId）——这是真判断，也是这个教练
//     能贡献的最有价值的东西
//   - 按这篇文章把每一步的 detail 说得更具体一点
//   - 篇幅短就砍掉几步
//
// 它**不能**发明步骤。步骤的 kind 和顺序来自 routine，模型只填得进那几个槽。
// 「以后按学生能力和文章难度生成」因此不需要任何结构改动——那只是这一次挑选
// 调用的额外输入。
//
// 这一步不是关卡：清单排出来之后，每一步都能直接点「跳过」，跳过被记录
// （reading_task.status='skipped'，铁律④），不被拦住。

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
)

// readingPlanArticleRuneBudget bounds how much article feeds the planning
// call. Generous — the model has to judge which paragraphs matter, which
// means seeing them — but bounded, because lite has no compaction layer.
const readingPlanArticleRuneBudget = 9000

const readingPlanSystem = `你是「印记」，要给一个中学生排出读这篇文章的**任务清单**。

下面会给你：这篇文章（按段落编号）、可选的几套读法、以及她的语言。

你要做的只有四件事：

1. 从给出的读法里**挑一套**（routineKey 必须逐字取自表里）。
2. 指出哪 **1–2 段**值得精读（focusBlocks，用段落编号 b1/b2/…）。这是你最重要的
   判断：挑那种「读懂了这一段，整篇就通了」的段落，或者那种最难、最容易被跳过去
   的段落。不要挑第一段就了事。
3. 给每一步写一句**贴着这篇文章**的说明（steps[].detail）。比如不要写「精读重点
   段」，要写「这一段是全文唯一给出数据的地方，值得慢慢看」。
4. 篇幅很短、或者内容很浅的文章，可以少排几步——一步都不能编，但可以不排。

严格规则：
- **不要替她读。** 说明里不要出现这篇文章的结论、主旨、答案。你在说「这一步要
  干什么」，不是在说「这篇讲了什么」。她还没读呢。
- 步骤的 kind 和顺序**只能**来自你挑的那套读法，不能新增、不能改顺序。
- focusBlocks 必须是真实存在的段落编号。
- detail 每条不超过 40 个字。

只输出一个 JSON 对象：
{"routineKey":"...","focusBlocks":["b3"],"steps":[{"kind":"read","detail":"..."}]}

steps 按顺序对应你挑的那套读法的步骤；kind 逐字照抄。不要输出对象以外的任何
文字或代码块标记。`

func buildReadingPlanPrompt(lang, title string, blocks []Block) string {
	var b strings.Builder
	if t := strings.TrimSpace(title); t != "" {
		b.WriteString("标题：" + t + "\n")
	}
	b.WriteString("她读这篇用的语言：" + lang + "\n")

	b.WriteString("\n【可选的读法（routineKey 只能从这里挑）】\n")
	for _, r := range readingRoutinesFor(lang) {
		b.WriteString("- routineKey=" + r.Key + " · " + r.Name + " · 适合：" + r.Blurb + "\n")
		for i, s := range r.Steps {
			b.WriteString("    " + itoaSmall(i+1) + ". kind=" + string(s.Kind) + " · " + s.Label + "\n")
		}
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
			// Truncate rather than drop: the model still needs to know that a
			// later paragraph EXISTS, or it can never pick it as a focus.
			keep := readingPlanArticleRuneBudget - total
			if keep > 60 {
				b.WriteString(blk.ID + "：" + string(runes[:keep]) + "…（这一段更长，已截断）\n")
				total = readingPlanArticleRuneBudget
			} else {
				b.WriteString(blk.ID + "：（这一段没放进来，但它存在）\n")
			}
			continue
		}
		total += len(runes)
		b.WriteString(blk.ID + "：" + text + "\n")
	}
	return b.String()
}

func itoaSmall(n int) string {
	if n < 10 {
		return string(rune('0' + n))
	}
	return string(rune('0'+n/10)) + string(rune('0'+n%10))
}

type readingPlanReply struct {
	RoutineKey  string   `json:"routineKey"`
	FocusBlocks []string `json:"focusBlocks"`
	Steps       []struct {
		Kind   string `json:"kind"`
		Detail string `json:"detail"`
	} `json:"steps"`
}

// parseReadingPlan decodes and VALIDATES against the library. The routine key
// must resolve and match her language; anything else is treated as no plan at
// all rather than passed through — a routine key that does not resolve would
// render as an empty task list she cannot act on.
func parseReadingPlan(text, lang string) (readingPlanReply, readingRoutine, bool) {
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
	var got readingPlanReply
	if err := json.Unmarshal([]byte(strings.TrimSpace(c)), &got); err != nil {
		return readingPlanReply{}, readingRoutine{}, false
	}
	routine, ok := findReadingRoutine(strings.TrimSpace(got.RoutineKey))
	if !ok {
		return readingPlanReply{}, readingRoutine{}, false
	}
	want := lang
	if want != "en" {
		want = "zh"
	}
	if routine.Lang != want {
		return readingPlanReply{}, readingRoutine{}, false
	}
	return got, routine, true
}

// buildReadingTasks turns the routine plus the model's tuning into the rows to
// insert.
//
// The ROUTINE owns the kinds and their order; the model may only fill in
// `detail` and say which blocks to focus on. A model that returned steps in a
// different order, or a kind the routine does not have, is simply ignored —
// the loop walks the ROUTINE, not the reply.
func buildReadingTasks(routine readingRoutine, plan readingPlanReply, valid map[string]bool) (
	positions []int32, kinds, labels, details, blockIDs []string,
) {
	focus := make([]string, 0, len(plan.FocusBlocks))
	for _, id := range plan.FocusBlocks {
		if valid[strings.TrimSpace(id)] {
			focus = append(focus, strings.TrimSpace(id))
		}
	}
	nextFocus := 0
	pos := int32(0)
	for i, step := range routine.Steps {
		detail := step.Detail
		if i < len(plan.Steps) && strings.TrimSpace(plan.Steps[i].Detail) != "" &&
			plan.Steps[i].Kind == string(step.Kind) {
			detail = strings.TrimSpace(plan.Steps[i].Detail)
		}
		blockID := ""
		if step.Kind == taskFocusBlock {
			if nextFocus < len(focus) {
				blockID = focus[nextFocus]
				nextFocus++
			} else {
				// A focus step with no paragraph behind it is a dead step —
				// she would be told to read "the highlighted paragraph" with
				// nothing highlighted. Drop it rather than render a lie.
				continue
			}
		}
		positions = append(positions, pos)
		kinds = append(kinds, string(step.Kind))
		labels = append(labels, step.Label)
		details = append(details, detail)
		blockIDs = append(blockIDs, blockID)
		pos++
	}
	return positions, kinds, labels, details, blockIDs
}

// generateReadingPlan is POST /api/v1/readings/{id}/plan.
//
// A spend endpoint (one model call), metered as purpose="reading_plan". It
// REPLACES any existing plan, which is what makes 重新排一份 possible; the
// statuses of the old plan go with it, and that is correct — a new plan is a
// new set of steps, not the old ones renumbered.
func (a *API) generateReadingPlan(w http.ResponseWriter, r *http.Request) {
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

	src, err := a.d.Queries.GetReadingSource(r.Context(), at.ID)
	if err != nil {
		httpx.WriteError(w, r, err) // no article yet → 404
		return
	}
	blocks := SplitBlocks(src.Body)
	if len(blocks) == 0 {
		httpx.WriteError(w, r, httpx.ErrBadRequest("empty_article", "这篇还没有正文，先把文章贴进来。", nil))
		return
	}
	lang := readingLangOf(src.Body)

	turnCtx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 150*time.Second)
	defer cancel()

	// §model-routing: judging which paragraph is the one worth slowing down on
	// is the hardest call in this room — flagship, like the writing room's
	// planning turn, not the chaperone an ordinary reply uses.
	resolved, okResolve := a.resolveEval(turnCtx)
	if !okResolve {
		slog.Warn("reading plan: no provider resolved",
			"atom_id", at.ID, "request_id", httpx.RequestIDFromContext(r.Context()))
		httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
		return
	}
	res, cerr := gateway.Collect(turnCtx, a.d.Provider, resolved, gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: readingPlanSystem},
			{Role: gateway.RoleUser, Content: buildReadingPlanPrompt(lang, src.Title, blocks)},
		},
	})
	a.recordLiteLLMCall(turnCtx, u.ID, at.ID, "reading_plan", resolved, res.Usage)
	if cerr != nil {
		slog.Warn("reading plan: provider call failed", "err", cerr,
			"atom_id", at.ID, "request_id", httpx.RequestIDFromContext(r.Context()))
		httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
		return
	}
	plan, routine, okParse := parseReadingPlan(res.Text, lang)
	if !okParse {
		// No canned fallback routine. A silently-substituted default would be
		// indistinguishable from a real plan, and she would never know the
		// coach had not actually looked at her article.
		slog.Warn("reading plan: unparseable or out-of-library reply",
			"atom_id", at.ID, "request_id", httpx.RequestIDFromContext(r.Context()))
		httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
		return
	}

	valid := make(map[string]bool, len(blocks))
	for _, blk := range blocks {
		valid[blk.ID] = true
	}
	positions, kinds, labels, details, blockIDs := buildReadingTasks(routine, plan, valid)
	if len(positions) == 0 {
		slog.Warn("reading plan: routine produced no usable steps",
			"atom_id", at.ID, "routine", routine.Key)
		httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
		return
	}

	tx, err := a.d.Pool.Begin(turnCtx)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer func() { _ = tx.Rollback(turnCtx) }()
	qtx := a.d.Queries.WithTx(tx)

	if _, err := qtx.SetReadingRoutine(turnCtx, sqlc.SetReadingRoutineParams{
		AtomID: at.ID, RoutineKey: routine.Key,
	}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if _, err := qtx.ReplaceReadingTasks(turnCtx, sqlc.ReplaceReadingTasksParams{
		AtomID: at.ID, Positions: positions, Kinds: kinds,
		Labels: labels, Details: details, BlockIds: blockIDs,
	}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := tx.Commit(turnCtx); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	rows, err := a.d.Queries.ListReadingTasks(r.Context(), at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"routineKey":  routine.Key,
		"routineName": routine.Name,
		"tasks":       readingTaskDTOs(rows),
	})
}

type readingTaskDTO struct {
	ID          string  `json:"id"`
	Position    int32   `json:"position"`
	Kind        string  `json:"kind"`
	Label       string  `json:"label"`
	Detail      string  `json:"detail"`
	BlockID     string  `json:"blockId"`
	Status      string  `json:"status"`
	CompletedAt *string `json:"completedAt"`
}

func readingTaskDTOs(rows []sqlc.ReadingTask) []readingTaskDTO {
	out := make([]readingTaskDTO, 0, len(rows))
	for _, row := range rows {
		dto := readingTaskDTO{
			ID: row.ID.String(), Position: row.Position, Kind: row.Kind,
			Label: row.Label, Detail: row.Detail, BlockID: row.BlockID, Status: row.Status,
		}
		if row.CompletedAt.Valid {
			s := row.CompletedAt.Time.Format(time.RFC3339)
			dto.CompletedAt = &s
		}
		out = append(out, dto)
	}
	return out
}

// getReadingPlan is GET /api/v1/readings/{id}/plan. No model call — an empty
// list is the honest "no plan yet" answer, not an error.
func (a *API) getReadingPlan(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedReadingAtom(w, r)
	if !ok {
		return
	}
	rows, err := a.d.Queries.ListReadingTasks(r.Context(), at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	rd, err := a.d.Queries.GetReading(r.Context(), at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	name := ""
	if routine, found := findReadingRoutine(rd.RoutineKey); found {
		name = routine.Name
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"routineKey": rd.RoutineKey, "routineName": name, "tasks": readingTaskDTOs(rows),
	})
}

// setReadingTaskStatus is POST /api/v1/readings/{id}/plan/tasks/{tid}.
//
// 铁律②: 'skipped' is a first-class outcome, not a failure. The task list is a
// map she can walk past, and skipping is RECORDED (过程即数据) rather than
// prevented.
func (a *API) setReadingTaskStatus(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedReadingAtom(w, r)
	if !ok {
		return
	}
	tid, err := uuid.Parse(r.PathValue("tid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	var req struct {
		Status string `json:"status"`
	}
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	switch req.Status {
	case "pending", "done", "skipped":
	default:
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_status", "无效的状态。", nil))
		return
	}
	row, err := a.d.Queries.SetReadingTaskStatus(r.Context(), sqlc.SetReadingTaskStatusParams{
		AtomID: at.ID, ID: tid, Status: req.Status,
	})
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	httpx.WriteJSON(w, http.StatusOK, readingTaskDTOs([]sqlc.ReadingTask{row})[0])
}

// readingLangOf guesses the article's language from its own characters, and
// deliberately does NOT ask the student.
//
// She already told us what she wants to read by pasting it; a language picker
// on top of that is a question whose answer is sitting right there. The rule
// is deliberately blunt — any meaningful run of CJK means the paragraph tools
// should be the Chinese set — because the cost of being wrong is offering the
// wrong four buttons, which she can simply not press.
func readingLangOf(body string) string {
	cjk := 0
	for _, ch := range body {
		if (ch >= 0x4e00 && ch <= 0x9fff) || (ch >= 0x3400 && ch <= 0x4dbf) {
			cjk++
			if cjk >= 24 {
				return "zh"
			}
		}
	}
	return "en"
}
