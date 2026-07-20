package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"unicode/utf8"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// submitOnboarding persists the student's S0 work: their own restate (≥15 runes)
// and which rubric rows they judged weakest. Recorded as BOTH a student graph
// node (so it survives reload / feeds the graph) and a studio event (过程即数据 →
// the assessor). No model call.
func (a *API) submitOnboarding(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFromContext(r.Context())
	projectID, ok := a.loadOwnedProject(w, r) // writes 404 itself on miss
	if !ok {
		return
	}
	entitled, err := HasEntitlement(r.Context(), u)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if !entitled {
		httpx.WriteError(w, r, httpx.ErrNotEntitled())
		return
	}
	var req struct {
		Restate   string `json:"restate"`
		WeakPicks []int  `json:"weakPicks"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("bad_json", "请求格式不对", nil))
		return
	}
	if utf8.RuneCountInString(req.Restate) < 15 {
		httpx.WriteError(w, r, httpx.ErrBadRequest("restate_too_short", "用自己的话多写一点（至少 15 字）。", nil))
		return
	}
	if len(req.WeakPicks) > 2 {
		httpx.WriteError(w, r, httpx.ErrBadRequest("too_many_picks", "最多选 2 条。", nil))
		return
	}
	if req.WeakPicks == nil {
		req.WeakPicks = []int{}
	}

	nodeBody, _ := json.Marshal(map[string]any{"restate": req.Restate, "weak_picks": req.WeakPicks})
	if _, err := a.d.Queries.InsertGraphNode(r.Context(), sqlc.InsertGraphNodeParams{
		ProjectID: projectID, Type: "task_restatement", Body: nodeBody, Author: "student", SpanRef: nil,
	}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	// Best-effort telemetry, exactly like studioturn's prompt_sent event.
	payload, _ := json.Marshal(map[string]string{"text": req.Restate})
	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
	if err := store.AppendEvent(r.Context(), agent.EventRow{
		ProjectID: projectID, Surface: "studio", Type: "onboarding_restated", Payload: payload,
	}); err != nil {
		slog.Warn("onboarding submit: append event failed",
			"err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{})
}
