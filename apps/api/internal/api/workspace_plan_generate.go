package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

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

const planGenSystem = `你是「印记」。学生刚把研究项目的开题四问填好（目标/缘由/活动与时间/资源）。请据此拟一份可执行的项目计划：5 到 9 个任务，覆盖「读—写—回顾」的完整节奏，落在大约 18 天的时间线上。
只输出一个 JSON 数组，每个元素形如 {"title":"...","tag":"read|write|review","stage":"...","start":<第几天,整数>,"days":<持续天数,整数>}。
要求：title 用中文、具体可动手；tag 只能是 read/write/review 三者之一，且三类都要有；stage 用「阶段一 · 研究」「阶段二 · 写作」这样的中文分段；start 从 0 起、按时间递增，days ≥ 1。不要输出数组以外的任何文字、解释或代码块标记。`

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

	prop, perr := a.d.Queries.GetProjectProposal(r.Context(), projectID)
	if perr == nil && !anyProposalDim(prop) {
		httpx.WriteError(w, r, &httpx.APIError{
			Status: http.StatusUnprocessableEntity, Code: "proposal_empty",
			Message: "先聊清楚开题，再生成计划",
		})
		return
	}
	if perr != nil {
		// No proposal row at all is also an empty kick-off.
		httpx.WriteError(w, r, &httpx.APIError{
			Status: http.StatusUnprocessableEntity, Code: "proposal_empty",
			Message: "先聊清楚开题，再生成计划",
		})
		return
	}

	items := a.generatePlanItems(r.Context(), projectID, prop)

	// Persist all produced items in one tx (column="todo", position by index).
	tx, err := a.d.Pool.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	qtx := a.d.Queries.WithTx(tx)

	out := make([]planItemDTO, 0, len(items))
	for i, it := range items {
		row, cerr := qtx.CreatePlanItem(r.Context(), sqlc.CreatePlanItemParams{
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
			httpx.WriteError(w, r, cerr)
			return
		}
		out = append(out, toPlanItemDTO(row))
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	if err := a.appendAutoLog(r.Context(), a.d.Queries, projectID, "印记根据开题生成了项目计划"); err != nil {
		slog.Warn("plan generate: append auto-log failed",
			"err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
	}
	// S1 · lever 1 (compaction): the plan has solidified — fold the shaping
	// dialogue (which reuses the forming/proposal_review scopes) out of the
	// coach's active window. Idempotent (only non-folded rows), best-effort.
	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
	if err := store.FoldCoachSurfaces(r.Context(), projectID, solidifyFoldSurfaces); err != nil {
		slog.Warn("plan generate: fold shaping turns failed",
			"err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": out})
}

// generatePlanItems makes the one-shot mid-tier completion, meters it (purpose=
// "plan_gen") BEFORE any bail, and returns validated tasks. On a missing
// provider, a model error, or an unparseable/empty reply it degrades to a small
// deterministic default board so the button always yields a usable plan.
func (a *API) generatePlanItems(ctx context.Context, projectID uuid.UUID, prop sqlc.ProjectProposal) []planGenItem {
	resolved, rerr := a.d.ChatResolver(ctx)
	if rerr != nil {
		slog.Warn("plan generate: no provider", "err", rerr)
		return defaultPlanItems()
	}
	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)

	var b strings.Builder
	fmt.Fprintf(&b, "目标：%s\n缘由：%s\n活动与时间：%s\n资源：%s\n",
		strings.TrimSpace(prop.Objective), strings.TrimSpace(prop.Reason),
		strings.TrimSpace(prop.Activities), strings.TrimSpace(prop.Resources))

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
		if len(out) >= 9 {
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
		{Title: "通读并溯源关键资料", Tag: "read", Stage: "阶段一 · 研究", Start: 0, Days: 4},
		{Title: "梳理论点与证据提纲", Tag: "write", Stage: "阶段一 · 研究", Start: 4, Days: 3},
		{Title: "写第一版正文草稿", Tag: "write", Stage: "阶段二 · 写作", Start: 8, Days: 5},
		{Title: "回顾修订并检查反例", Tag: "review", Stage: "阶段二 · 写作", Start: 14, Days: 3},
	}
}
