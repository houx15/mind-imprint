package api

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// evidence_map.go — slice 4a · the 证据地图 read projection + the per-paper
// evidence setters + the per-sub-question saturation review. The map IS the
// warren: sub-questions are the 子问题-edge targets of the main RQ lead; a
// sub-question's papers are its descendant leads with a connected reference.

type evidenceMapPaperDTO struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Nature  string `json:"nature"`
	Triage  string `json:"triage"`
	HasNote bool   `json:"hasNote"`
}

type evidenceMapSubQuestionDTO struct {
	ID     string                `json:"id"`
	Text   string                `json:"text"`
	Papers []evidenceMapPaperDTO `json:"papers"`
}

type evidenceMapDTO struct {
	MainQuestion string                      `json:"mainQuestion"`
	SubQuestions []evidenceMapSubQuestionDTO `json:"subQuestions"`
}

// evidenceMapProjection projects the seeded warren into the map. Returns the DTO
// plus, per sub-question lead id, the ordered references under it (for the
// saturation review to read without re-walking).
func (a *API) evidenceMapProjection(ctx context.Context, projectID uuid.UUID) (evidenceMapDTO, map[string][]sqlc.Reference, error) {
	leads, err := a.d.Queries.ListExplorationLeads(ctx, projectID)
	if err != nil {
		return evidenceMapDTO{}, nil, err
	}
	edges, err := a.d.Queries.ListQuestionEdgesByProject(ctx, projectID)
	if err != nil {
		return evidenceMapDTO{}, nil, err
	}
	refs, err := a.d.Queries.ListReferences(ctx, projectID)
	if err != nil {
		return evidenceMapDTO{}, nil, err
	}
	refByID := map[string]sqlc.Reference{}
	for _, r := range refs {
		refByID[r.ID.String()] = r
	}
	leadByID := map[string]sqlc.ExplorationLead{}
	childrenOf := map[string][]sqlc.ExplorationLead{}
	for _, l := range leads {
		leadByID[l.ID.String()] = l
		if l.ParentLeadID.Valid {
			pid := uuid.UUID(l.ParentLeadID.Bytes).String()
			childrenOf[pid] = append(childrenOf[pid], l)
		}
	}

	// The main RQ = the FROM of the 子问题 edges; its sub-questions = the TO leads.
	var mainQuestion string
	subQIDs := []string{}
	seen := map[string]bool{}
	for _, e := range edges {
		if e.Label != "子问题" {
			continue
		}
		if mainQuestion == "" {
			if l, ok := leadByID[e.FromLeadID.String()]; ok {
				mainQuestion = l.Text
			}
		}
		to := e.ToLeadID.String()
		if !seen[to] {
			seen[to] = true
			subQIDs = append(subQIDs, to)
		}
	}

	// A sub-question's papers = descendant leads with a connected reference.
	collectPapers := func(sqID string) []sqlc.Reference {
		var out []sqlc.Reference
		stack := []string{sqID}
		visited := map[string]bool{sqID: true}
		for len(stack) > 0 {
			cur := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			for _, child := range childrenOf[cur] {
				cid := child.ID.String()
				if visited[cid] {
					continue
				}
				visited[cid] = true
				stack = append(stack, cid)
				if child.ConnectedReferenceID.Valid {
					if ref, ok := refByID[uuid.UUID(child.ConnectedReferenceID.Bytes).String()]; ok && !ref.Archived {
						out = append(out, ref)
					}
				}
			}
		}
		return out
	}

	dto := evidenceMapDTO{MainQuestion: mainQuestion, SubQuestions: []evidenceMapSubQuestionDTO{}}
	papersBySubQ := map[string][]sqlc.Reference{}
	for _, sqID := range subQIDs {
		l := leadByID[sqID]
		refsUnder := collectPapers(sqID)
		papersBySubQ[sqID] = refsUnder
		papers := make([]evidenceMapPaperDTO, 0, len(refsUnder))
		for _, ref := range refsUnder {
			papers = append(papers, evidenceMapPaperDTO{
				ID: ref.ID.String(), Title: ref.Title, Nature: ref.EvidenceNature, Triage: ref.Triage,
				HasNote: strings.TrimSpace(ref.EvidenceArgument) != "" || strings.TrimSpace(ref.EvidenceFinding) != "",
			})
		}
		dto.SubQuestions = append(dto.SubQuestions, evidenceMapSubQuestionDTO{ID: sqID, Text: l.Text, Papers: papers})
	}
	return dto, papersBySubQ, nil
}

// GET /projects/{id}/evidence-map
func (a *API) getEvidenceMap(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	dto, _, err := a.evidenceMapProjection(r.Context(), projectID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, dto)
}

func (a *API) refForProject(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, bool) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return uuid.UUID{}, uuid.UUID{}, false
	}
	rid, err := uuid.Parse(r.PathValue("rid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return uuid.UUID{}, uuid.UUID{}, false
	}
	return projectID, rid, true
}

// PATCH /projects/{id}/references/{rid}/evidence
func (a *API) patchReferenceEvidence(w http.ResponseWriter, r *http.Request) {
	projectID, rid, ok := a.refForProject(w, r)
	if !ok {
		return
	}
	var body struct {
		Nature    string `json:"nature"`
		Argument  string `json:"argument"`
		Finding   string `json:"finding"`
		Placement string `json:"placement"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if body.Nature != "" && body.Nature != "support" && body.Nature != "challenge" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "nature 必须是 support / challenge / 空", nil))
		return
	}
	row, err := a.d.Queries.SetReferenceEvidence(r.Context(), sqlc.SetReferenceEvidenceParams{
		ID: rid, ProjectID: projectID, EvidenceNature: body.Nature,
		EvidenceArgument: body.Argument, EvidenceFinding: body.Finding, EvidencePlacement: body.Placement,
	})
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toReferenceDTO(row, nil))
}

// PATCH /projects/{id}/references/{rid}/triage
func (a *API) patchReferenceTriage(w http.ResponseWriter, r *http.Request) {
	projectID, rid, ok := a.refForProject(w, r)
	if !ok {
		return
	}
	var body struct {
		Triage string `json:"triage"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if body.Triage != "" && body.Triage != "red" && body.Triage != "yellow" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "triage 必须是 red / yellow / 空", nil))
		return
	}
	row, err := a.d.Queries.SetReferenceTriage(r.Context(), sqlc.SetReferenceTriageParams{ID: rid, ProjectID: projectID, Triage: body.Triage})
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toReferenceDTO(row, nil))
}

// POST /projects/{id}/references/{rid}/archive {archived?}
func (a *API) archiveReference(w http.ResponseWriter, r *http.Request) {
	projectID, rid, ok := a.refForProject(w, r)
	if !ok {
		return
	}
	body := struct {
		Archived *bool `json:"archived"`
	}{}
	_ = decodeJSON(r, &body)
	archived := true
	if body.Archived != nil {
		archived = *body.Archived
	}
	row, err := a.d.Queries.SetReferenceArchived(r.Context(), sqlc.SetReferenceArchivedParams{ID: rid, ProjectID: projectID, Archived: archived})
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toReferenceDTO(row, nil))
}

// POST /projects/{id}/evidence-map/subquestions/{sqId}/review
func (a *API) reviewSubQuestionSaturation(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	sqID := r.PathValue("sqId")

	dto, papersBySubQ, err := a.evidenceMapProjection(r.Context(), projectID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	subText := ""
	for _, sq := range dto.SubQuestions {
		if sq.ID == sqID {
			subText = sq.Text
		}
	}
	if subText == "" {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}

	verdict := agent.SubQuestionVerdictOut{Saturated: true, Why: "", Gaps: []string{}}
	if a.d.Provider != nil && a.d.EvalResolver != nil {
		if resolved, rerr := a.d.EvalResolver(r.Context()); rerr == nil {
			papers := make([]agent.EvidencePaper, 0, len(papersBySubQ[sqID]))
			for _, ref := range papersBySubQ[sqID] {
				papers = append(papers, agent.EvidencePaper{
					Title: ref.Title, Nature: ref.EvidenceNature, Argument: ref.EvidenceArgument, Finding: ref.EvidenceFinding,
				})
			}
			v, usage, verr := agent.ReviewEvidenceSaturation(r.Context(), a.d.Provider, resolved, agent.EvidenceReviewInput{
				SubQuestion: subText, Papers: papers,
			})
			a.meterCall(r.Context(), projectID, resolved, "evidence_saturation", usage)
			if verr == nil {
				verdict = v
			} else {
				slog.Warn("evidence saturation: review failed", "err", verr, "request_id", httpx.RequestIDFromContext(r.Context()))
			}
		}
	}
	gaps := verdict.Gaps
	if gaps == nil {
		gaps = []string{}
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"subQuestionId": sqID, "saturated": verdict.Saturated, "why": verdict.Why, "gaps": gaps,
	})
}
