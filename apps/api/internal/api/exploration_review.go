package api

import (
	"log/slog"
	"net/http"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/httpx"
)

// exploration_review.go — §5 (user follow-up) · POST runs 印记's holistic review
// of the collected + tagged materials (across all sub-questions), suggesting what's
// most correlated/important, what's weakly related (trim), and gaps. Flagship
// reviewer (EvalResolver, reasoning on). Best-effort: no provider → empty review.

// POST /projects/{id}/exploration/review → { review: "..." }
func (a *API) postExplorationReview(w http.ResponseWriter, r *http.Request) {
	row, ok := a.loadOwnedProjectRow(w, r)
	if !ok {
		return
	}
	projectID := row.ID
	if row.IsDemo {
		httpx.WriteJSON(w, http.StatusOK, cannedExplorationReview())
		return
	}
	dto, papersBySubQ, err := a.evidenceMapProjection(r.Context(), projectID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	in := agent.ExplorationReviewInput{Question: dto.MainQuestion}
	for _, sq := range dto.SubQuestions {
		rsq := agent.ExplorationReviewSubQuestion{Text: sq.Text}
		for _, ref := range papersBySubQ[sq.ID] {
			rsq.Papers = append(rsq.Papers, agent.ExplorationReviewPaper{
				Title: ref.Title, Nature: ref.EvidenceNature, Argument: ref.EvidenceArgument, Finding: ref.EvidenceFinding,
			})
		}
		in.SubQuestions = append(in.SubQuestions, rsq)
	}

	review := ""
	if a.d.Provider != nil {
		if resolved, rok := a.resolveEval(r.Context()); rok {
			out, usage, verr := agent.ReviewExploration(r.Context(), a.d.Provider, resolved, in)
			a.meterCall(r.Context(), projectID, resolved, "exploration_review", usage)
			if verr != nil {
				slog.Warn("exploration review: failed", "err", verr, "request_id", httpx.RequestIDFromContext(r.Context()))
			} else {
				review = out
			}
		}
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"review": review})
}
