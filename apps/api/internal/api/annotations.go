package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/httpx"
)

// annotationDTO is one addressable 整稿体检 review item, surfaced from its
// persisted review_item intervention row.
type annotationDTO struct {
	ID        string `json:"id"`
	Criterion string `json:"criterion"`
	Band      string `json:"band"`
	Text      string `json:"text"`
}

// getAnnotations lists the project's whole-draft review items as addressable
// annotations — the id + criterion/band + an actionable text built from the
// item's missing/fix fields (RL-1: still advice, never a rewritten sentence).
// Read-only projection over intervention rows already written by orderReview
// (writing.go) — no new persistence.
func (a *API) getAnnotations(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	rows, err := a.d.Queries.ListReviewItemsByProject(r.Context(), projectID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := make([]annotationDTO, 0, len(rows))
	for _, row := range rows {
		var item agent.ReviewItem
		if err := json.Unmarshal([]byte(row.Body), &item); err != nil {
			continue // skip an unparseable row rather than fail the whole list
		}
		missing, fix := strings.TrimSpace(item.Missing), strings.TrimSpace(item.Fix)
		var text string
		switch {
		case missing != "" && fix != "":
			text = missing + " → " + fix
		case missing != "":
			text = missing
		case fix != "":
			text = fix
		}
		if text == "" {
			continue // nothing actionable to anchor
		}
		out = append(out, annotationDTO{
			ID:        row.ID.String(),
			Criterion: item.CriterionName,
			Band:      item.Band,
			Text:      text,
		})
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"annotations": out})
}
