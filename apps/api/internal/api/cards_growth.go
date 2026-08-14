package api

import (
	"net/http"
	"time"

	"mindimprint/api/internal/httpx"
)

// collectedCardDTO is one tool card the caller has completed, with a usage
// summary. Instance-derived only — name/purpose/category are resolved on the web
// from CARD_REGISTRY (single source of truth), never re-encoded here. RL-5: uses
// is descriptive, never a grade.
type collectedCardDTO struct {
	CardID   string   `json:"cardId"`
	Uses     int      `json:"uses"`
	Surfaces []string `json:"surfaces"` // subset of {"project","course","chat"}
	LastUsed string   `json:"lastUsed"` // RFC3339
}

// getGrowthCards returns every tool card the caller has completed, across
// project/course/chat, deduped with a usage summary. No model call, ever.
func (a *API) getGrowthCards(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFromContext(r.Context())
	rows, err := a.d.Queries.ListCollectedCardsByUser(r.Context(), u.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	cards := make([]collectedCardDTO, 0, len(rows))
	for _, row := range rows {
		surfaces := row.Surfaces
		if surfaces == nil {
			surfaces = []string{}
		}
		// Task 1's ListCollectedCardsByUserRow.LastUsed is interface{} (sqlc can't
		// infer a concrete type for max(created_at) over the UNION ALL). pgx scans
		// the timestamptz into a time.Time; assert it. GROUP BY guarantees ≥1 row
		// per group so it is never nil, but fall back to "" rather than panic.
		lastUsed := ""
		if ts, ok := row.LastUsed.(time.Time); ok {
			lastUsed = ts.Format(time.RFC3339)
		}
		cards = append(cards, collectedCardDTO{
			CardID:   row.CardID,
			Uses:     int(row.Uses),
			Surfaces: surfaces,
			LastUsed: lastUsed,
		})
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"cards": cards})
}
