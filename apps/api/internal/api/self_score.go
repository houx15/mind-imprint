package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/skills"
	"mindimprint/api/internal/store/sqlc"
)

// submitSelfScore persists the student's per-criterion self-assessment (RL-4:
// the student's own judgement, not the system grading them; RL-5: never an
// aggregate score). One band (0..2) per review criterion. No model call.
func (a *API) submitSelfScore(w http.ResponseWriter, r *http.Request) {
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
		Scores []struct {
			Code string `json:"code"`
			Band int    `json:"band"`
		} `json:"scores"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("bad_json", "请求格式不对", nil))
		return
	}
	valid := map[string]bool{}
	if sk, ok := skills.ByID("writing-project"); ok {
		for _, c := range sk.ReviewCriteria {
			valid[c.Code] = true
		}
	}
	for _, s := range req.Scores {
		if !valid[s.Code] {
			httpx.WriteError(w, r, httpx.ErrBadRequest("bad_code", "未知的评分维度", nil))
			return
		}
		if s.Band < 0 || s.Band > 2 {
			httpx.WriteError(w, r, httpx.ErrBadRequest("bad_band", "档位超出范围", nil))
			return
		}
	}
	nodeBody, _ := json.Marshal(map[string]any{"scores": req.Scores})
	if _, err := a.d.Queries.InsertGraphNode(r.Context(), sqlc.InsertGraphNodeParams{
		ProjectID: projectID, Type: "self_score", Body: nodeBody, Author: "student", SpanRef: nil,
	}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
	if err := store.AppendEvent(r.Context(), agent.EventRow{
		ProjectID: projectID, Surface: "studio", Type: "self_scored", Payload: []byte(`{}`),
	}); err != nil {
		slog.Warn("self-score: append event failed", "err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{})
}
