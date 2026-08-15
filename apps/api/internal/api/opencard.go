package api

// opencard.go — deadlock-prevention fix: when a leftover proposed/active card
// instance blocks all new summons (the read-together one-active mutex,
// readturn.go), the reading room used to show NO card at all — the student
// was stuck with no way to see or dismiss the thing that's blocking her.
// GET .../materials/{mid}/open-card lets the room, on mount, ask "is there
// already an open card for THIS material?" and if so load it so it renders
// (and can be completed or skipped) instead of vanishing into the mutex.

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/httpx"
)

// openCardResp is the wire shape of getOpenReadingCard's response. AnchorsJSON
// carries the raw anchors array verbatim (same shape the read-turn/summon-card
// SSE `card` frame emits — agent.Anchor's own JSON tags), so the client can
// reuse the exact same parsing path. CardInstanceID is "" when no card is
// open for this material — the room's signal to render nothing.
type openCardResp struct {
	CardInstanceID string          `json:"card_instance_id"`
	CardID         string          `json:"card_id"`
	Status         string          `json:"status"`
	Anchors        json.RawMessage `json:"anchors"`
}

var emptyOpenCardResp = openCardResp{Anchors: json.RawMessage("[]")}

// getOpenReadingCard returns the open (proposed/active) card_instance for
// THIS material, if any — derived the SAME way readturn.go's own
// OpenCard/onMaterial scan does (a card instance whose status is
// proposed/active and whose persisted anchors carry this material's id).
// Because the mutex it mirrors is project-wide (readturn.go's `openCard`
// bool), at most one such instance can ever exist across the whole project,
// so the first match is returned. As a recovery fallback it also surfaces an
// anchor-less open card (a stranded graceful-degrade summon) so the mutex can
// never jam the room invisibly.
func (a *API) getOpenReadingCard(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	mid, err := uuid.Parse(r.PathValue("mid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}

	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
	g, err := store.LoadGraph(r.Context(), projectID)
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrInternal())
		return
	}
	// Ownership: mid must be one of THIS project's materials — same
	// parse-based compare postReadingTurn uses (MaterialView.ID is a
	// pgtype.UUID rendering that may differ in dashing/case from
	// google/uuid's canonical String()).
	foundMaterial := false
	for _, m := range g.Materials {
		if pid, perr := uuid.Parse(m.ID); perr == nil && pid == mid {
			foundMaterial = true
			break
		}
	}
	if !foundMaterial {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}

	matID := mid.String()
	// An open card with NO anchors is a stranded graceful-degrade summon
	// (summoncard.go's no-example branch mints `proposed` with `[]` anchors):
	// it can't be scoped to a material, yet because the one-active mutex is
	// project-wide it still blocks every new lens. Such a card is invisible to
	// the on-material scan below, so without surfacing it the room shows nothing
	// while every summon is jammed — the exact deadlock this endpoint prevents.
	// Prefer an on-material anchored card; fall back to the anchor-less one so it
	// renders (at the first block, client-side) and can be completed or skipped.
	var fallback *openCardResp
	for _, ci := range g.CardInstances {
		if ci.Status != "proposed" && ci.Status != "active" {
			continue
		}
		onMaterial := false
		for _, an := range ci.Anchors {
			if an.MaterialID == matID {
				onMaterial = true
				break
			}
		}
		if !onMaterial {
			if len(ci.Anchors) == 0 && fallback == nil {
				fallback = &openCardResp{
					CardInstanceID: ci.ID,
					CardID:         ci.CardID,
					Status:         ci.Status,
					Anchors:        json.RawMessage("[]"),
				}
			}
			continue
		}
		anchorsJSON, merr := json.Marshal(ci.Anchors)
		if merr != nil {
			anchorsJSON = []byte("[]")
		}
		httpx.WriteJSON(w, http.StatusOK, openCardResp{
			CardInstanceID: ci.ID,
			CardID:         ci.CardID,
			Status:         ci.Status,
			Anchors:        anchorsJSON,
		})
		return
	}

	if fallback != nil {
		httpx.WriteJSON(w, http.StatusOK, *fallback)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, emptyOpenCardResp)
}
