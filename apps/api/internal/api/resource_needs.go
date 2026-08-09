package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/httpx"
)

// resource_needs.go — slice 5 · the "还需要探索的" (needs-resources) box (§101/§115).
// A student-authored list of things to look up, stored on studio_state (jsonb, no
// migration). GET reads it; PUT replaces the whole list (like the reflection doc).
// No LLM spend. The reading room's search-guidance box also reads these to seed
// keyword suggestions.

type resourceNeedDTO struct {
	ID   string `json:"id"`
	Text string `json:"text"`
	Done bool   `json:"done"`
}

func (a *API) loadStudioStateForNeeds(ctx context.Context, projectID uuid.UUID) (agent.StudioState, error) {
	state := agent.DefaultStudioState()
	if raw, err := a.d.Queries.GetStudioState(ctx, projectID); err == nil && len(raw) > 0 {
		if uerr := json.Unmarshal(raw, &state); uerr != nil {
			return state, uerr
		}
	}
	return state, nil
}

func resourceNeedDTOs(needs []agent.ResourceNeed) []resourceNeedDTO {
	out := make([]resourceNeedDTO, 0, len(needs))
	for _, n := range needs {
		out = append(out, resourceNeedDTO{ID: n.ID, Text: n.Text, Done: n.Done})
	}
	return out
}

// GET /projects/{id}/resource-needs
func (a *API) getResourceNeeds(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	state, err := a.loadStudioStateForNeeds(r.Context(), projectID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"needs": resourceNeedDTOs(state.ResourceNeeds)})
}

// PUT /projects/{id}/resource-needs { needs: [{id,text,done}] } — whole-list replace.
func (a *API) putResourceNeeds(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	var body struct {
		Needs []resourceNeedDTO `json:"needs"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	state, err := a.loadStudioStateForNeeds(r.Context(), projectID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	needs := make([]agent.ResourceNeed, 0, len(body.Needs))
	for _, n := range body.Needs {
		text := strings.TrimSpace(n.Text)
		if text == "" {
			continue
		}
		id := strings.TrimSpace(n.ID)
		if id == "" {
			id = uuid.NewString()
		}
		needs = append(needs, agent.ResourceNeed{ID: id, Text: text, Done: n.Done})
	}
	state.ResourceNeeds = needs
	if serr := a.saveTrackState(r.Context(), projectID, state); serr != nil {
		httpx.WriteError(w, r, serr)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"needs": resourceNeedDTOs(needs)})
}
