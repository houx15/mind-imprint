package api

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// placement.go — POST suggest-placement · 印记 suggests which research question
// a just-added reference belongs under (or null → 未归类). Fast model, spends
// tokens (metered). Best-effort: no provider / model error / no questions →
// { leadId: null, reason: "" }, 200. Advisory (铁律②): the student confirms via
// the attach endpoint.
func (a *API) postSuggestPlacement(w http.ResponseWriter, r *http.Request) {
	row, ok := a.loadOwnedProjectRow(w, r)
	if !ok {
		return
	}
	projectID := row.ID
	if row.IsDemo {
		httpx.WriteJSON(w, http.StatusOK, cannedPlacement())
		return
	}
	var body struct {
		ReferenceID string `json:"referenceId"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	refUUID, perr := uuid.Parse(strings.TrimSpace(body.ReferenceID))
	if perr != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "referenceId 不是有效的 id", nil))
		return
	}
	ref, err := a.d.Queries.GetReferenceForProject(r.Context(), sqlc.GetReferenceForProjectParams{ID: refUUID, ProjectID: projectID})
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "referenceId 不是这个项目里的来源", nil))
		return
	}

	// Question leads = non-pruned leads with no connected reference (root main
	// questions + nested sub-questions). Papers (connected_reference_id set) and
	// pruned leads are not placement targets.
	leads, err := a.d.Queries.ListExplorationLeads(r.Context(), projectID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var questions []agent.PlacementQuestion
	for _, l := range leads {
		if l.Status == "pruned" || l.ConnectedReferenceID.Valid {
			continue
		}
		questions = append(questions, agent.PlacementQuestion{ID: l.ID.String(), Text: l.Text})
	}

	// Best-effort default (also the no-questions / no-provider answer).
	leadID := ""
	reason := ""
	if len(questions) > 0 && a.d.Provider != nil {
		if resolved, rok := a.resolveFast(r.Context()); rok {
			out, usage, gerr := agent.SuggestBestQuestion(r.Context(), a.d.Provider, resolved, agent.SuggestPlacementInput{
				Title: ref.Title, Abstract: ref.Abstract, Journal: ref.Journal, Year: ref.Year, Questions: questions,
			})
			a.meterCall(r.Context(), projectID, resolved, "suggest_placement", usage)
			if gerr != nil {
				slog.Warn("suggest placement: generation failed", "err", gerr, "request_id", httpx.RequestIDFromContext(r.Context()))
			} else {
				leadID = out.LeadID
				reason = out.Reason
			}
		}
	}

	// leadId serialises as null when empty (未归类).
	var leadOut any
	if leadID != "" {
		leadOut = leadID
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"leadId": leadOut, "reason": reason})
}
