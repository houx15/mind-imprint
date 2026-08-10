package api

import (
	"net/http"
	"strings"

	"mindimprint/api/internal/httpx"
)

// card_tags.go — §4 gap G8 · per-guided-part status tags (green=写好了 / yellow=待完善),
// keyed by the step key, stored on studio_state (jsonb, no migration). GET reads
// the whole map; PUT sets one key. No LLM spend.

// GET /projects/{id}/card-tags → { tags: { <stepKey>: "green"|"yellow" } }
func (a *API) getCardTags(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	state, err := a.loadStudioStateForNeeds(r.Context(), projectID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	tags := state.CardTags
	if tags == nil {
		tags = map[string]string{}
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"tags": tags})
}

// PUT /projects/{id}/card-tags { key, status } — status "" clears the tag.
func (a *API) putCardTag(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	var body struct {
		Key    string `json:"key"`
		Status string `json:"status"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	key := strings.TrimSpace(body.Key)
	if key == "" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "key 不能为空", nil))
		return
	}
	status := strings.TrimSpace(body.Status)
	if status != "" && status != "green" && status != "yellow" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "status 必须是 green / yellow 或空", nil))
		return
	}
	state, err := a.loadStudioStateForNeeds(r.Context(), projectID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if state.CardTags == nil {
		state.CardTags = map[string]string{}
	}
	if status == "" {
		delete(state.CardTags, key)
	} else {
		state.CardTags[key] = status
	}
	if serr := a.saveTrackState(r.Context(), projectID, state); serr != nil {
		httpx.WriteError(w, r, serr)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"tags": state.CardTags})
}
