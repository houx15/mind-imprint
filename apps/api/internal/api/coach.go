package api

import (
	"log/slog"
	"net/http"
	"strings"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/httpx"
)

// coach.go — Slice 2's room-scoped restrained coach turn (POST /coach). Unlike
// the studio /turn loop (studioturn.go), this is a plain one-shot JSON turn: it
// never enters the station/gate/summon machinery. It resolves the mid-tier
// (ChatResolver), makes a single non-streaming provider completion behind a
// restrained, one-question-at-a-time system prompt chosen by scope, meters the
// call, and returns {reply}. It embodies the four 铁律: AI 克制, one question at
// a time, never gives answers, never writes for the student.

// coachScopePrompts is the restrained system prompt per room scope. Absent a
// known scope, forming (the safest, most general) is used.
var coachScopePrompts = map[string]string{
	"forming":      "你是「印记」，陪学生想清楚一个研究项目的开题。一次只问一个问题，帮他把「目标/缘由/活动与时间/资源」四件事聊清楚；绝不替他定题、不给现成答案。回应简短。",
	"find_sources": "你是「印记」，陪学生找资料。只给方向、关键词、可信度判断；绝不替他检索或提供来源链接。一次一个建议，简短。",
	"writing":      "你是「印记」，陪学生写作。聊提纲、挑逻辑、撞反例；绝不替他写正文、不给成段文字。一次一个问题，简短。",
	"proposal_review": "你是「印记」。学生刚把开题四问填了个初稿（在下面）。请只做一件事：指出最值得再想清楚的一两点——哪里还含糊、哪个尺度没定、哪个反例没考虑；给一个具体的下一步。不要替他重写，不要给成段答案。简短。",
}

// coachFallbackReply is returned when the model yields nothing usable (empty
// text or a provider error). The student is never 500'd on a coach turn — the
// point is to keep thinking, and a restrained nudge does that even offline.
const coachFallbackReply = "先自己说说看——现在你最想弄清楚的是哪一点？"

// postCoach runs one restrained, scope-selected coach turn. Spend endpoint:
// gates on HasEntitlement, meters via RecordLLMCall before any bail.
func (a *API) postCoach(w http.ResponseWriter, r *http.Request) {
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

	var body struct {
		Scope     string `json:"scope"`
		UserInput string `json:"user_input"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if strings.TrimSpace(body.UserInput) == "" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "user_input 不能为空", nil))
		return
	}
	system, ok := coachScopePrompts[body.Scope]
	if !ok {
		system = coachScopePrompts["forming"]
	}

	resolved, err := a.d.ChatResolver(r.Context())
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrInternal())
		return
	}

	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)

	// One-shot completion (mirrors agent/coach.go's ProposeIntervention shape:
	// gateway.Collect over a system+user message pair).
	res, cerr := gateway.Collect(r.Context(), a.d.Provider, resolved, gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: system},
			{Role: gateway.RoleUser, Content: body.UserInput},
		},
	})

	// Meter the call BEFORE any bail — a call that yields nothing still cost
	// money (design's 记录档位 + token + 成本). Metering failure never fails the
	// turn. Only record when a real call happened (Resolved populated).
	if resolved.Provider != "" {
		if rerr := store.RecordLLMCall(r.Context(), agent.LLMCallRow{
			ProjectID: projectID, Surface: "studio", Purpose: "coach",
			Resolved: resolved, PromptTokens: int32(res.Usage.InputTokens), CompletionTokens: int32(res.Usage.OutputTokens),
		}); rerr != nil {
			slog.Warn("coach: record llm call failed", "err", rerr, "request_id", httpx.RequestIDFromContext(r.Context()))
		}
	}

	reply := strings.TrimSpace(res.Text)
	if cerr != nil || reply == "" {
		if cerr != nil {
			slog.Warn("coach: provider completion failed", "err", cerr, "request_id", httpx.RequestIDFromContext(r.Context()))
		}
		reply = coachFallbackReply
	}

	// Record the exchange as a coach_turn event so the process tree carries it.
	// Best-effort — never fails the turn.
	if err := store.AppendEvent(r.Context(), agent.EventRow{
		ProjectID: projectID, Surface: "studio", Type: "coach_turn",
		Payload: mustJSON(map[string]any{"scope": body.Scope, "student": body.UserInput, "ai": reply}),
	}); err != nil {
		slog.Warn("coach: append coach_turn event failed", "err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]any{"reply": reply})
}
