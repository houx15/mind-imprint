package api

import (
	"net/http"

	"mindimprint/api/internal/httpx"
)

func (a *API) adminOverview(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFromContext(r.Context())
	c, err := a.d.Queries.GetSchoolCounts(r.Context(), u.SchoolID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	rows, err := a.d.Queries.GetSchoolUsageByTier(r.Context(), u.SchoolID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	usage := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		usage = append(usage, map[string]any{
			"tier":              row.Tier,
			"prompt_tokens":     row.PromptTokens,
			"completion_tokens": row.CompletionTokens,
			"cost":              numericString(row.Cost),
		})
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"counts": map[string]any{
			"student":        c.StudentCount,
			"teacher":        c.TeacherCount,
			"class":          c.ClassCount,
			"task":           c.TaskCount,
			"evaluation":     c.EvaluationCount,
			"active_student": c.ActiveStudentCount,
		},
		"usage_by_tier": usage,
	})
}
