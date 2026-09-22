package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// errProposalEmpty is regeneratePlan's sentinel for an empty kick-off (no
// proposal row, or a proposal with none of the four dims filled). Its two
// callers each handle it differently — postPlanGenerate writes the 422
// proposal_empty response, the coach's generate_plan tool just no-ops (the
// narration lands regardless).
var errProposalEmpty = errors.New("proposal_empty")

// workspace_plan_generate.go — BE3: POST /projects/{id}/plan/generate. Unlike
// the plain plan CRUD (workspace_plan.go), this one SPENDS: it resolves the
// mid-tier chaperone, makes one restrained JSON completion grounded in the
// kick-off proposal, and persists the produced tasks as todo plan_items. It
// never blocks on the model — a failed/parse-failed call degrades to a small
// deterministic default board so the button always yields something usable.

// planGenItem is the model's per-task JSON contract.
type planGenItem struct {
	Title string `json:"title"`
	Tag   string `json:"tag"`
	Stage string `json:"stage"`
	Start int32  `json:"start"`
	Days  int32  `json:"days"`
}

const planGenSystem = `你是「印记」。学生刚把研究项目的「大框架」讨论清楚（目标/缘由/活动与时间/资源/可能的反例）——这是研究方向的框架，还不是正式提案。请据此拟一份可执行的完整项目计划。

【计划内容】以学生自己在「活动与时间」里、以及【开题讨论】里描述的步骤、顺序为主干：将已经明确的做法和顺序整理为具体任务，保留学生的安排和用词，不用通用模板替换。只有在现有安排缺少研究项目必要的环节（写研究提案、溯源关键文献、找论点并收集证据、搭提纲与论证结构、写正文、全文审阅与复盘）时，才补上。任务措辞要贴合这个具体课题，不要泛泛而谈。

【时间安排】先从「活动与时间」和【开题讨论】读出学生打算用多长时间、分成哪几个阶段，然后让整条时间线的总长度符合学生说明的总时长：
- 例：学生提出「前三周读文献和定义、四到五周做数据提取与分析、六到七周写作、最后一周复盘和反例」——那总跨度约 8 周 ≈ 56 天：读文献 start≈0 days≈21、数据分析 start≈21 days≈14、写作 start≈35 days≈14、复盘 start≈49 days≈7，各任务落到对应的那几周上。
- start 是从第 0 天（也就是今天）起算的第几天，days 是任务持续天数；把每个任务排到学生描述的那一周/那几天，让最后一个任务的结束日 ≈ 学生说的总时长。
- 学生明确给出 6–8 周时，计划总时长应保持在该范围内。只有当学生完全没提时间时，才按任务量给一个合理跨度（约 3–4 周）。

只输出一个 JSON 数组，每个元素形如 {"title":"...","tag":"read|write|review","stage":"...","start":<第几天,整数>,"days":<持续天数,整数>}。
要求：任务数量随计划长短而定（短计划 5–7 个，长计划可到 10–12 个）；必须包含一个「写研究提案」类任务（tag=write，尽量排在最前）；title 和 stage 会直接展示给学生；title 用中文，明确说明任务动作，内容与当前课题相关；tag 只能是 read/write/review 三者之一，且三类都要有；stage 用「阶段一 · 提案」「阶段二 · 研究」「阶段三 · 写作」这样的中文分段；start 从 0 起、按时间递增，days ≥ 1。不要输出数组以外的任何文字、解释或代码块标记。`

// postPlanGenerate reads the proposal, refuses an empty kick-off (422
// proposal_empty), then one-shot-generates + persists the plan. Spend endpoint:
// gates on HasEntitlement, meters purpose="plan_gen" before any bail.
func (a *API) postPlanGenerate(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	u, _ := UserFromContext(r.Context())

	entitled, err := HasEntitlement(r.Context(), u)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if !entitled {
		httpx.WriteError(w, r, httpx.ErrNotEntitled())
		return
	}

	out, gerr := a.regeneratePlan(r.Context(), projectID)
	if gerr != nil {
		if errors.Is(gerr, errProposalEmpty) {
			httpx.WriteError(w, r, &httpx.APIError{
				Status: http.StatusUnprocessableEntity, Code: "proposal_empty",
				Message: "先聊清楚开题，再生成计划",
			})
			return
		}
		httpx.WriteError(w, r, gerr)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": out})
}

// regeneratePlan does the actual work: read the proposal (422-worthy empty
// kick-off → errProposalEmpty), one-shot-generate + persist the tasks
// (delete+recreate wholesale, #15), auto-log, and fold the shaping dialogue
// out of the coach's active window. Shared by postPlanGenerate AND the coach's
// generate_plan tool (coach.go) — the button is gone, 印记 triggers this
// itself once the four proposal dims are filled.
func (a *API) regeneratePlan(ctx context.Context, projectID uuid.UUID) ([]planItemDTO, error) {
	prop, perr := a.d.Queries.GetProjectProposal(ctx, projectID)
	if perr == nil && !anyProposalDim(prop) {
		return nil, errProposalEmpty
	}
	if perr != nil {
		// No proposal row at all is also an empty kick-off.
		return nil, errProposalEmpty
	}

	items := a.generatePlanItems(ctx, projectID, prop)

	// Order the board chronologically: the model can emit tasks out of date
	// order (e.g. all 阶段三 before 阶段二), and the board/spine render by
	// position. Sort by start day (then stage) so position — and thus the
	// kanban todo column and the plan spine — reads left-to-right in time.
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Start != items[j].Start {
			return items[i].Start < items[j].Start
		}
		return items[i].Stage < items[j].Stage
	})

	// Persist all produced items in one tx (column="todo", position by index).
	tx, err := a.d.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := a.d.Queries.WithTx(tx)

	// #15: regenerating replaces the plan wholesale. Clear the existing board
	// first so re-generating (after 聊聊计划) reschedules the project instead of
	// stacking a second plan on top of the first. No-op on the first generate.
	if err := qtx.DeletePlanItemsByProject(ctx, projectID); err != nil {
		return nil, err
	}

	out := make([]planItemDTO, 0, len(items))
	for i, it := range items {
		row, cerr := qtx.CreatePlanItem(ctx, sqlc.CreatePlanItemParams{
			ProjectID: projectID,
			Title:     it.Title,
			Tag:       it.Tag,
			Col:       "todo",
			Stage:     it.Stage,
			StartDay:  it.Start,
			Days:      it.Days,
			Position:  int32(i),
		})
		if cerr != nil {
			return nil, cerr
		}
		out = append(out, toPlanItemDTO(row))
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	if err := a.appendAutoLog(ctx, a.d.Queries, projectID, "印记根据开题生成了项目计划"); err != nil {
		slog.Warn("plan generate: append auto-log failed",
			"err", err, "request_id", httpx.RequestIDFromContext(ctx))
	}
	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)

	// G1 · record the 立题完成 (framework-finished) milestone the FIRST time a
	// plan is generated — that IS the moment 立题 finishes (gaps doc decision).
	// Emitted at most once per project: regenerating the plan (聊聊计划 → 重新生成)
	// replaces the board but must not move the milestone. Best-effort — never
	// fails plan generation.
	if n, cerr := a.d.Queries.CountEventsByType(ctx, sqlc.CountEventsByTypeParams{
		ProjectID: pgtype.UUID{Bytes: projectID, Valid: true},
		Type:      "milestone:framework_finished",
	}); cerr != nil {
		slog.Warn("plan generate: count framework milestone failed",
			"err", cerr, "request_id", httpx.RequestIDFromContext(ctx))
	} else if n == 0 {
		if eerr := store.AppendEvent(ctx, agent.EventRow{
			ProjectID: projectID, Surface: "studio",
			Type: "milestone:framework_finished", Payload: []byte("{}"),
		}); eerr != nil {
			slog.Warn("plan generate: emit framework milestone failed",
				"err", eerr, "request_id", httpx.RequestIDFromContext(ctx))
		}
	}

	// S1 · lever 1 (compaction): the plan has solidified — fold the shaping
	// dialogue (which reuses the forming/proposal_review scopes) out of the
	// coach's active window. Idempotent (only non-folded rows), best-effort.
	if err := store.FoldCoachSurfaces(ctx, projectID, solidifyFoldSurfaces); err != nil {
		slog.Warn("plan generate: fold shaping turns failed",
			"err", err, "request_id", httpx.RequestIDFromContext(ctx))
	}
	return out, nil
}

// generatePlanItems makes the one-shot mid-tier completion, meters it (purpose=
// "plan_gen") BEFORE any bail, and returns validated tasks. On a missing
// provider, a model error, or an unparseable/empty reply it degrades to a small
// deterministic default board so the button always yields a usable plan.
func (a *API) generatePlanItems(ctx context.Context, projectID uuid.UUID, prop sqlc.ProjectProposal) []planGenItem {
	// §model-routing · compose. Plan generation derives structure from what the
	// student has already stated — a deterministic system step (AGENTS.md is
	// explicit that 铁律② does not reach it), not a judgement on her work.
	resolved, ok := a.route(ctx, gateway.ClassCompose)
	if !ok {
		slog.Warn("plan generate: no provider")
		return defaultPlanItems()
	}
	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)

	var b strings.Builder
	// The essay prompt/title anchors the plan to THIS course's task.
	if p, perr := a.d.Queries.GetProject(ctx, projectID); perr == nil && strings.TrimSpace(p.Title) != "" {
		fmt.Fprintf(&b, "题目：%s\n\n", strings.TrimSpace(p.Title))
	}
	// All FIVE confirmed framework dims — including 可能的反例, which the plan must
	// leave room to test, and which was previously dropped from this prompt.
	fmt.Fprintf(&b, "【框架要点（学生确认过的）】\n目标：%s\n缘由：%s\n活动与时间：%s\n资源：%s\n可能的反例：%s\n",
		strings.TrimSpace(prop.Objective), strings.TrimSpace(prop.Reason),
		strings.TrimSpace(prop.Activities), strings.TrimSpace(prop.Resources),
		strings.TrimSpace(prop.Counterpoints))
	// The detailed shaping dialogue: the student often describes their plan in
	// far more detail than the compressed 活动与时间 one-liner captures. Feed the
	// (bounded) discussion so the plan follows the design they actually talked
	// through, not just the distilled note.
	if disc := a.frameworkDiscussion(ctx, projectID); disc != "" {
		fmt.Fprintf(&b, "\n【开题讨论（学生和印记怎么聊这个计划的，节选）】\n%s\n", disc)
	}

	res, cerr := gateway.Collect(ctx, a.d.Provider, resolved, gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: planGenSystem},
			{Role: gateway.RoleUser, Content: b.String()},
		},
	})

	// Meter BEFORE any bail — a call that yields nothing still cost money. Only
	// record when a real call happened (Resolved populated).
	if res.Usage.InputTokens > 0 || res.Usage.OutputTokens > 0 {
		if err := store.RecordLLMCall(ctx, agent.LLMCallRow{
			ProjectID: projectID, Surface: "studio", Purpose: "plan_gen",
			Resolved: resolved, PromptTokens: int32(res.Usage.InputTokens), CompletionTokens: int32(res.Usage.OutputTokens),
		}); err != nil {
			slog.Warn("plan generate: record llm call failed", "err", err)
		}
	}

	if cerr != nil {
		slog.Warn("plan generate: provider completion failed", "err", cerr)
		return defaultPlanItems()
	}
	items := parsePlanItems(res.Text)
	if len(items) == 0 {
		slog.Warn("plan generate: model reply unparseable — seeding default plan")
		return defaultPlanItems()
	}
	return items
}

// frameworkDiscussion returns a bounded transcript of the student↔印记 shaping
// dialogue (the active coach thread right before plan generation) so the plan-
// gen prompt can follow the design the student actually talked through, not
// just the distilled 活动与时间 note. Chronological, capped to the most recent
// turns and a rune budget; "" when there's nothing to show.
func (a *API) frameworkDiscussion(ctx context.Context, projectID uuid.UUID) string {
	rows, err := a.d.Queries.ListActiveChatMessagesByProject(ctx, pgtype.UUID{Bytes: projectID, Valid: true})
	if err != nil || len(rows) == 0 {
		return ""
	}
	const maxTurns = 24
	if len(rows) > maxTurns {
		rows = rows[len(rows)-maxTurns:]
	}
	var b strings.Builder
	const maxRunes = 4500
	used := 0
	for _, m := range rows {
		content := strings.TrimSpace(m.Content)
		if content == "" {
			continue
		}
		who := "学生"
		switch m.Role {
		case "assistant":
			who = "印记"
		case "user":
			who = "学生"
		default:
			continue // skip system/tool rows
		}
		line := who + "：" + content
		rc := len([]rune(line))
		if used+rc > maxRunes {
			break
		}
		b.WriteString(line)
		b.WriteByte('\n')
		used += rc
	}
	return strings.TrimSpace(b.String())
}

// parsePlanItems defensively decodes the model's JSON array: strip code fences,
// clamp to the first '['..last ']', unmarshal, then keep only well-formed tasks
// (valid tag, non-empty title, days ≥ 1) — up to 9. Returns nil when nothing
// usable survives, so the caller can seed the default board.
func parsePlanItems(text string) []planGenItem {
	c := strings.TrimSpace(text)
	if strings.HasPrefix(c, "```json") {
		c = strings.TrimLeft(strings.TrimPrefix(c, "```json"), " \t\r\n")
	} else if strings.HasPrefix(c, "```") {
		c = strings.TrimLeft(c[3:], " \t\r\n")
	}
	if strings.HasSuffix(c, "```") {
		c = strings.TrimRight(c[:len(c)-3], " \t\r\n")
	}
	if i := strings.IndexByte(c, '['); i > 0 {
		c = c[i:]
	}
	if j := strings.LastIndexByte(c, ']'); j >= 0 && j < len(c)-1 {
		c = c[:j+1]
	}
	var raw []planGenItem
	if err := json.Unmarshal([]byte(strings.TrimSpace(c)), &raw); err != nil {
		return nil
	}
	out := make([]planGenItem, 0, len(raw))
	for _, it := range raw {
		if strings.TrimSpace(it.Title) == "" || !validPlanTags[it.Tag] {
			continue
		}
		if it.Days < 1 {
			it.Days = 1
		}
		if it.Start < 0 {
			it.Start = 0
		}
		it.Title = strings.TrimSpace(it.Title)
		it.Stage = strings.TrimSpace(it.Stage)
		if it.Stage == "" {
			it.Stage = "阶段一 · 研究"
		}
		out = append(out, it)
		if len(out) >= 12 {
			break
		}
	}
	return out
}

// defaultPlanItems is the deterministic 4-item fallback (read/write/write/review
// across two stages) so the generate button always yields a usable board even
// when the model is unavailable or unparseable.
func defaultPlanItems() []planGenItem {
	return []planGenItem{
		{Title: "写一份完整的研究提案", Tag: "write", Stage: "阶段一 · 提案", Start: 0, Days: 3},
		{Title: "通读并溯源关键文献", Tag: "read", Stage: "阶段二 · 研究", Start: 3, Days: 4},
		{Title: "梳理论点、收集支撑证据", Tag: "write", Stage: "阶段二 · 研究", Start: 7, Days: 3},
		{Title: "搭建提纲与论证结构", Tag: "write", Stage: "阶段三 · 写作", Start: 10, Days: 2},
		{Title: "写第一版正文草稿", Tag: "write", Stage: "阶段三 · 写作", Start: 12, Days: 5},
		{Title: "整稿体检并复盘修订", Tag: "review", Stage: "阶段三 · 写作", Start: 17, Days: 3},
	}
}
