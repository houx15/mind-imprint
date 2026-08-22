package api

import (
	"net/http"
	"strings"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// feedback.go — Task 10 · the nav rail's 反馈 button. Free-text feedback with
// no structure, no rubric, no evaluation weight — a plain landing pad
// separate from the process-evaluation data model.

// postFeedback handles POST /api/v1/feedback with body {text}. Scoped to the
// signed-in user; empty/whitespace-only text 400s.
func (a *API) postFeedback(w http.ResponseWriter, r *http.Request) {
	u, ok := UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, r, httpx.ErrUnauthorized("未登录"))
		return
	}
	var req struct {
		Text string `json:"text"`
	}
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	text := strings.TrimSpace(req.Text)
	if text == "" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("empty_feedback", "反馈内容不能为空。", nil))
		return
	}
	row, err := a.d.Queries.CreateFeedback(r.Context(), sqlc.CreateFeedbackParams{UserID: u.ID, Text: text})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"id": row.ID.String()})
}
