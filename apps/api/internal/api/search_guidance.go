package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/httpx"
)

// search_guidance.go — slice 5 (§113/§116) · POST generates 印记's reading-room
// search suggestions (spends tokens on the fast model — 铁律②: the student asks by
// tapping 让印记建议检索方向). Reads the research question + sub-questions + the
// 还需要探索的 notes off studio_state. Best-effort: no provider / model error →
// empty list, 200.

// POST /projects/{id}/search-guidance → { suggestions: [{keyword, why}] }
func (a *API) postSearchGuidance(w http.ResponseWriter, r *http.Request) {
	row, ok := a.loadOwnedProjectRow(w, r)
	if !ok {
		return
	}
	projectID := row.ID
	if row.IsDemo {
		httpx.WriteJSON(w, http.StatusOK, cannedSearchGuidance())
		return
	}
	state, err := a.loadStudioStateForNeeds(r.Context(), projectID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	// Optional body: the question layer the student is currently inside, so the
	// directions bias toward it (item 3.1). Absent/empty ⇒ whole-topic guidance.
	var body struct {
		FocusQuestion string `json:"focusQuestion"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

	in := agent.SearchGuidanceInput{FocusQuestion: strings.TrimSpace(body.FocusQuestion)}
	if p, perr := a.d.Queries.GetProject(r.Context(), projectID); perr == nil {
		in.Title = p.Title
	}
	if prop, perr := a.d.Queries.GetProjectProposal(r.Context(), projectID); perr == nil {
		in.Question = prop.Objective
	}
	if state.ProposalTrack != nil {
		in.SubQuestions = state.ProposalTrack.SubQuestions
	}
	for _, n := range state.ResourceNeeds {
		if !n.Done {
			in.Needs = append(in.Needs, n.Text)
		}
	}

	suggestions := []agent.SearchSuggestionOut{}
	if a.d.Provider != nil {
		if resolved, rok := a.resolveFast(r.Context()); rok {
			out, usage, gerr := agent.ProposeSearchKeywords(r.Context(), a.d.Provider, resolved, in)
			a.meterCall(r.Context(), projectID, resolved, "search_guidance", usage)
			if gerr != nil {
				slog.Warn("search guidance: generation failed", "err", gerr, "request_id", httpx.RequestIDFromContext(r.Context()))
			} else {
				suggestions = out
			}
		}
	}

	dtos := make([]map[string]string, 0, len(suggestions))
	for _, s := range suggestions {
		dtos = append(dtos, map[string]string{"keyword": s.Keyword, "why": s.Why})
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"suggestions": dtos})
}
