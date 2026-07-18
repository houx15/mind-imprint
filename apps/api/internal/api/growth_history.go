package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/studio"
)

// growthHistoryEntry is one row of the 成长报告 history hub: a surface + label +
// date, with the full report embedded (the list is already owner-filtered, so
// no second fetch and no per-report auth). RL-5: no score/level at this level —
// the diagnostic lives inside report.dimensions only.
type growthHistoryEntry struct {
	Surface   string               `json:"surface"` // "project" | "course" | "chat"
	ScopeID   string               `json:"scopeId"`
	Label     string               `json:"label"`
	Sublabel  *string              `json:"sublabel"`
	CreatedAt string               `json:"createdAt"`
	Report    studio.AssessmentDTO `json:"report"`
}

// getGrowthHistory returns every report the caller owns across project, course,
// and chat scopes, newest-first. No model call, ever.
func (a *API) getGrowthHistory(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFromContext(r.Context())
	rows, err := a.d.Queries.ListGrowthHistory(r.Context(), u.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	entries := make([]growthHistoryEntry, 0, len(rows))
	for _, row := range rows {
		var dims []studio.AssessmentDimensionDTO
		if err := json.Unmarshal(row.Scores, &dims); err != nil {
			// A malformed row must not sink the whole list; skip it.
			continue
		}
		// sqlc actual types (verified in Task 2): row.Sublabel is *string (nil
		// when absent), row.CreatedAt is time.Time, row.ScopeID is pgtype.UUID.
		created := row.CreatedAt.Format(time.RFC3339)
		entries = append(entries, growthHistoryEntry{
			Surface:   row.Surface,
			ScopeID:   uuidText(row.ScopeID),
			Label:     row.Label,
			Sublabel:  row.Sublabel,
			CreatedAt: created,
			Report: studio.AssessmentDTO{
				Dimensions:  dims,
				Narrative:   row.Narrative,
				GeneratedAt: created,
			},
		})
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"entries": entries})
}

// uuidText renders a pgtype.UUID as its canonical string form; no existing
// helper does this generically in the api package (grepped for "uuidText"
// and pgtype.UUID+".String()" — every call site converts uuid.UUID->pgtype.UUID,
// never the reverse), so this is the one direction-of-conversion this
// package needed added.
func uuidText(id pgtype.UUID) string {
	return uuid.UUID(id.Bytes).String()
}
