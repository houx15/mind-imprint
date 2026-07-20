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

// submitReflection persists the student's S6 研究回顾 (RL-4: written by the
// student, the platform never authors reflective text). This is the `reflection`
// node the reflect_archive gate names. No model call.
func (a *API) submitReflection(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r) // 404 hides other users'
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
	var req struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("bad_json", "请求格式不对", nil))
		return
	}
	if utf8.RuneCountInString(req.Text) < 20 {
		httpx.WriteError(w, r, httpx.ErrBadRequest("reflection_too_short", "多写一点你的真实回顾（至少 20 字）。", nil))
		return
	}
	nodeBody, _ := json.Marshal(map[string]any{"text": req.Text})
	if _, err := a.d.Queries.InsertGraphNode(r.Context(), sqlc.InsertGraphNodeParams{
		ProjectID: projectID, Type: "reflection", Body: nodeBody, Author: "student", SpanRef: nil,
	}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	payload, _ := json.Marshal(map[string]string{"text": req.Text})
	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
	if err := store.AppendEvent(r.Context(), agent.EventRow{
		ProjectID: projectID, Surface: "studio", Type: "reflection_written", Payload: payload,
	}); err != nil {
		slog.Warn("reflection: append event failed", "err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{})
}
