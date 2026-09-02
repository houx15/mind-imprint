package api

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
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
	in, err := a.explorationReviewInput(r.Context(), row)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	review := ""
	if a.d.Provider != nil {
		if resolved, rok := a.route(r.Context(), gateway.ClassReview); rok {
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

// explorationReviewInput projects the WHOLE warren for the 理一理 review: every
// question node (root or sub) with the papers hanging under it, plus the 未归类
// bucket.
//
// This used to read evidenceMapProjection, which walks 子问题 edges only. Two
// consequences, both reported by a student on 2026-08-28: a project whose
// warren was never proposal-seeded has no 子问题 edges at all, so the review ran
// on an empty map and answered "材料还太少"; and 未归类 sources hang under no
// question by definition, so the one thing students actually ask 印记 to help
// with — "帮我把这些文献理一理" — was the one thing it could not see.
//
// Each paper is attributed to its NEAREST ancestor question node, so a paper
// under a sub-question is counted there and not double-counted into its root.
func (a *API) explorationReviewInput(ctx context.Context, row sqlc.Project) (agent.ExplorationReviewInput, error) {
	projectID := row.ID
	leads, err := a.d.Queries.ListExplorationLeads(ctx, projectID)
	if err != nil {
		return agent.ExplorationReviewInput{}, err
	}
	refs, err := a.d.Queries.ListReferences(ctx, projectID)
	if err != nil {
		return agent.ExplorationReviewInput{}, err
	}
	edges, err := a.d.Queries.ListQuestionEdgesByProject(ctx, projectID)
	if err != nil {
		return agent.ExplorationReviewInput{}, err
	}

	refByID := map[string]sqlc.Reference{}
	for _, r := range refs {
		refByID[r.ID.String()] = r
	}
	leadByID := map[string]sqlc.ExplorationLead{}
	for _, l := range leads {
		leadByID[l.ID.String()] = l
	}
	// A QUESTION node is a live lead carrying no reference of its own; a PAPER
	// node is one that does. Pruned leads are neither (the student dropped that
	// thread), and a paper under a pruned question falls back to 未归类.
	isQuestion := func(l sqlc.ExplorationLead) bool {
		return l.Status != "pruned" && !l.ConnectedReferenceID.Valid
	}
	// nearestQuestion walks up the parent chain to the first live question
	// node; "" when the chain runs out (or hits a pruned lead) — 未归类.
	nearestQuestion := func(l sqlc.ExplorationLead) string {
		cur := l
		for i := 0; i < 32; i++ { // depth guard; a cycle must never hang a request
			if !cur.ParentLeadID.Valid {
				return ""
			}
			parent, ok := leadByID[uuid.UUID(cur.ParentLeadID.Bytes).String()]
			if !ok {
				return ""
			}
			if isQuestion(parent) {
				return parent.ID.String()
			}
			cur = parent
		}
		return ""
	}

	paper := func(ref sqlc.Reference) agent.ExplorationReviewPaper {
		return agent.ExplorationReviewPaper{
			Title: ref.Title, Nature: ref.EvidenceNature,
			Argument: ref.EvidenceArgument, Finding: ref.EvidenceFinding,
		}
	}

	papersByQuestion := map[string][]agent.ExplorationReviewPaper{}
	filed := map[string]bool{} // reference ids that hang under SOME live question
	for _, l := range leads {
		if l.Status == "pruned" || !l.ConnectedReferenceID.Valid {
			continue
		}
		refID := uuid.UUID(l.ConnectedReferenceID.Bytes).String()
		ref, ok := refByID[refID]
		if !ok || ref.Archived {
			continue
		}
		qid := nearestQuestion(l)
		if qid == "" {
			continue // a paper node with no question above it reads as 未归类
		}
		filed[refID] = true
		papersByQuestion[qid] = append(papersByQuestion[qid], paper(ref))
	}

	// The main question: the FROM of a 子问题 edge when the warren was
	// proposal-seeded; otherwise the first root question node; otherwise the
	// project's own title (which is also what seeds the first node).
	mainID := ""
	for _, e := range edges {
		if e.Label == "子问题" {
			if l, ok := leadByID[e.FromLeadID.String()]; ok && isQuestion(l) {
				mainID = l.ID.String()
				break
			}
		}
	}
	if mainID == "" {
		for _, l := range leads {
			if isQuestion(l) && !l.ParentLeadID.Valid {
				mainID = l.ID.String()
				break
			}
		}
	}
	in := agent.ExplorationReviewInput{Question: strings.TrimSpace(row.Title)}
	if mainID != "" {
		in.Question = leadByID[mainID].Text
	}

	for _, l := range leads {
		if !isQuestion(l) {
			continue
		}
		id := l.ID.String()
		// The main question is already the header; only list it as its own
		// bucket when papers hang directly under it.
		if id == mainID && len(papersByQuestion[id]) == 0 {
			continue
		}
		in.SubQuestions = append(in.SubQuestions, agent.ExplorationReviewSubQuestion{
			Text: l.Text, Papers: papersByQuestion[id],
		})
	}

	// 未归类 · every non-archived reference no live question holds. Mirrors the
	// client's unfiledReferences so the review sees exactly the list the
	// student is looking at.
	for _, ref := range refs {
		if ref.Archived || filed[ref.ID.String()] {
			continue
		}
		in.Unfiled = append(in.Unfiled, paper(ref))
	}
	return in, nil
}
