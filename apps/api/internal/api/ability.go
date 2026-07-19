package api

import (
	"encoding/json"
	"net/http"

	"mindimprint/api/internal/ability"
	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/httpx"
)

// getAbilityModel projects the caller's whole cross-session DualAxis history into
// a current-standing 能力素养 model. Read-only, owner-filtered, NO model call, NO
// cost (RL-5: a merge of many sessions' evidence, never a single-session 档位).
func (a *API) getAbilityModel(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFromContext(r.Context())
	rows, err := a.d.Queries.ListEvaluationsByUser(r.Context(), u.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	samples := make([]ability.Sample, 0, len(rows))
	for _, row := range rows {
		var report agent.Report
		if err := json.Unmarshal(row.Scores, &report); err != nil {
			continue // a malformed row must not sink the aggregate
		}
		samples = append(samples, ability.Sample{Report: report, CreatedAt: row.CreatedAt})
	}
	httpx.WriteJSON(w, http.StatusOK, ability.Aggregate(samples))
}
