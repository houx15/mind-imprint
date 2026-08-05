package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/materialize"
	"mindimprint/api/internal/store/sqlc"
)

// errNotRootLead is returned by getRootLeadForProject when a lead exists in
// the project but has a parent — question_edge connects top-level question
// nodes only (constraints.md), so a 分支 is not a valid endpoint.
var errNotRootLead = errors.New("lead is not a root question node")

// exploration.go — S3 rabbit-hole exploration surface, Task 4: the lead
// lifecycle handlers (no LLM spend). A "lead" is a thread worth following —
// spawned from a reading takeaway's newLeads (Task 4b, extends
// postFinalizeReading), typed in manually here, or proposed by the 深挖一层
// guide (Task 4c, SPENDS, not this file). getExploration also projects
// danglingSourceIds: read sources with nothing following up on them yet, a
// nudge to connect or prune rather than leaving them orphaned.

// -- wire DTO -----------------------------------------------------------

// explorationLeadDTO mirrors the contracts ExplorationLead (camelCase,
// following referenceDTO's conventions): DB source_reference_id/
// connected_reference_id -> sourceReferenceId/connectedReferenceId.
type explorationLeadDTO struct {
	ID                   string  `json:"id"`
	Text                 string  `json:"text"`
	Status               string  `json:"status"`
	Origin               string  `json:"origin"`
	SourceReferenceID    *string `json:"sourceReferenceId"`
	ConnectedReferenceID *string `json:"connectedReferenceId"`
	Position             int32   `json:"position"`
	ParentLeadID         *string `json:"parentLeadId"` // #12 · null = top-level thread
}

func toExplorationLeadDTO(row sqlc.ExplorationLead) explorationLeadDTO {
	return explorationLeadDTO{
		ID:                   row.ID.String(),
		Text:                 row.Text,
		Status:               row.Status,
		Origin:               row.Origin,
		SourceReferenceID:    pgUUIDToStringPtr(row.SourceReferenceID),
		ConnectedReferenceID: pgUUIDToStringPtr(row.ConnectedReferenceID),
		Position:             row.Position,
		ParentLeadID:         pgUUIDToStringPtr(row.ParentLeadID),
	}
}

var validLeadStatus = map[string]bool{"open": true, "connected": true, "pruned": true}

// -- B1 · question_edge DTO ------------------------------------------------

// questionEdgeDTO mirrors contracts's QuestionEdge (camelCase): a labeled,
// directed edge between two top-level question leads — the data foundation
// of the two-level exploration graph. B2 adds the endpoints that create/
// relabel/confirm/delete these; this task only projects them into the view.
type questionEdgeDTO struct {
	ID         string `json:"id"`
	FromLeadID string `json:"fromLeadId"`
	ToLeadID   string `json:"toLeadId"`
	Label      string `json:"label"`
	Status     string `json:"status"`
}

func toQuestionEdgeDTO(row sqlc.QuestionEdge) questionEdgeDTO {
	return questionEdgeDTO{
		ID:         row.ID.String(),
		FromLeadID: row.FromLeadID.String(),
		ToLeadID:   row.ToLeadID.String(),
		Label:      row.Label,
		Status:     row.Status,
	}
}

// validQuestionEdgeLabel is the closed 5-value vocabulary (constraints.md):
// no freeform labels, ever.
var validQuestionEdgeLabel = map[string]bool{
	"子问题": true, "支持": true, "反驳/张力": true, "细化": true, "依赖/前提": true,
}

var validQuestionEdgeStatus = map[string]bool{"proposed": true, "confirmed": true}

// getRootLeadForProject loads a lead scoped to this project (IDOR guard,
// GetExplorationLeadForProject) AND asserts it's a ROOT question node
// (parentLeadId absent) — question_edge connects top-level question nodes
// only, never a 分支. Returns a descriptive error suitable for a 400 when the
// lead doesn't exist in this project or isn't a root.
func (a *API) getRootLeadForProject(ctx context.Context, projectID, leadID uuid.UUID) (sqlc.ExplorationLead, error) {
	lead, err := a.d.Queries.GetExplorationLeadForProject(ctx, sqlc.GetExplorationLeadForProjectParams{ID: leadID, ProjectID: projectID})
	if err != nil {
		return sqlc.ExplorationLead{}, err
	}
	if lead.ParentLeadID.Valid {
		return sqlc.ExplorationLead{}, errNotRootLead
	}
	return lead, nil
}

// -- GET /exploration -----------------------------------------------------

// getExploration returns every lead for the project plus danglingSourceIds
// plus the question_edge graph — no spend, purely a projection over
// ListExplorationLeads + ListReferences + ListQuestionEdgesByProject.
func (a *API) getExploration(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	leads, err := a.d.Queries.ListExplorationLeads(r.Context(), projectID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	refs, err := a.d.Queries.ListReferences(r.Context(), projectID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	edges, err := a.d.Queries.ListQuestionEdgesByProject(r.Context(), projectID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	dtos := make([]explorationLeadDTO, 0, len(leads))
	for _, l := range leads {
		dtos = append(dtos, toExplorationLeadDTO(l))
	}
	edgeDTOs := make([]questionEdgeDTO, 0, len(edges))
	for _, e := range edges {
		edgeDTOs = append(edgeDTOs, toQuestionEdgeDTO(e))
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"leads":             dtos,
		"danglingSourceIds": computeDanglingSourceIds(refs, leads),
		"edges":             edgeDTOs,
	})
}

// computeDanglingSourceIds is a PURE function (spec §4a): a reference is
// dangling iff it has an engaged material (material_id valid — the student
// has actually opened it), its decision is undecided or "drop" (decision
// "use"/"maybe" means the student has already made something of it), AND its
// id is not the connected_reference_id of any lead whose status != "pruned"
// (a pruned connection doesn't count — the student explicitly abandoned that
// thread, so the source is dangling again). A reference is ALSO not dangling
// if it's the source_reference_id of at least one non-pruned lead — a source
// that spawned branches is USED, not dangling, even before it has an
// explicit decision (the NASA-source happy path: finalized takeaway, open
// takeaway leads under it, decision still unset). If every lead spawned from
// it is pruned, it correctly falls back to dangling — the student abandoned
// every branch, same as never having branched at all. Returns ids in stable
// order (refs' own ListReferences order).
func computeDanglingSourceIds(refs []sqlc.Reference, leads []sqlc.ExplorationLead) []string {
	excluded := map[string]bool{}
	for _, l := range leads {
		if l.Status == "pruned" {
			continue
		}
		if l.ConnectedReferenceID.Valid {
			excluded[uuid.UUID(l.ConnectedReferenceID.Bytes).String()] = true
		}
		if l.SourceReferenceID.Valid {
			excluded[uuid.UUID(l.SourceReferenceID.Bytes).String()] = true
		}
	}
	out := []string{}
	for _, ref := range refs {
		if !ref.MaterialID.Valid {
			continue
		}
		if ref.Decision != nil && *ref.Decision != "drop" {
			continue
		}
		id := ref.ID.String()
		if excluded[id] {
			continue
		}
		out = append(out, id)
	}
	return out
}

// -- POST /exploration/leads ------------------------------------------------

// createExplorationLead adds a manual lead: origin "manual", status "open",
// appended after whatever already exists.
func (a *API) createExplorationLead(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	var body struct {
		Text         string  `json:"text"`
		ParentLeadID *string `json:"parentLeadId"` // #12 · when set, this is a 分支 under that lead
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if strings.TrimSpace(body.Text) == "" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "线索不能是空的", nil))
		return
	}
	// #12 · optional parent: validate it's a lead in THIS project (IDOR) before
	// hanging a 分支 under it.
	var parent pgtype.UUID
	if body.ParentLeadID != nil && strings.TrimSpace(*body.ParentLeadID) != "" {
		pid, perr := uuid.Parse(*body.ParentLeadID)
		if perr != nil {
			httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "parentLeadId 不是有效的 id", nil))
			return
		}
		if _, err := a.d.Queries.GetExplorationLeadForProject(r.Context(), sqlc.GetExplorationLeadForProjectParams{ID: pid, ProjectID: projectID}); err != nil {
			httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "parentLeadId 不是这个项目里的线索", nil))
			return
		}
		parent = pgtype.UUID{Bytes: pid, Valid: true}
	}
	existing, err := a.d.Queries.ListExplorationLeads(r.Context(), projectID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	row, err := a.d.Queries.CreateExplorationLead(r.Context(), sqlc.CreateExplorationLeadParams{
		ProjectID:    projectID,
		Text:         body.Text,
		Status:       "open",
		Origin:       "manual",
		Position:     int32(len(existing)),
		ParentLeadID: parent,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"lead": toExplorationLeadDTO(row)})
}

// -- PATCH /exploration/leads/{lid} -----------------------------------------

// patchExplorationLead partial-merges { text?, status?, connectedReferenceId? }
// over the current row — connect (status:"connected" + connectedReferenceId),
// prune (status:"pruned"), reopen (status:"open", + null the connection), or
// a plain text edit. status must be a valid enum member when present;
// connectedReferenceId, when present & non-null, must resolve to a reference
// in this project (IDOR guard), else 400.
func (a *API) patchExplorationLead(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	lid, err := uuid.Parse(r.PathValue("lid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	cur, err := a.d.Queries.GetExplorationLeadForProject(r.Context(), sqlc.GetExplorationLeadForProjectParams{ID: lid, ProjectID: projectID})
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}

	// text/status use absent-means-keep pointers; connectedReferenceId uses
	// json.RawMessage so a present-null can clear the connection (reopen),
	// which a plain pointer can't distinguish from "absent" (patchReference's
	// pattern, workspace_library.go).
	var body struct {
		Text                 *string         `json:"text"`
		Status               *string         `json:"status"`
		ConnectedReferenceID json.RawMessage `json:"connectedReferenceId"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	next := sqlc.UpdateExplorationLeadParams{
		ID:                   lid,
		ProjectID:            projectID,
		Text:                 cur.Text,
		Status:               cur.Status,
		ConnectedReferenceID: cur.ConnectedReferenceID,
		Position:             cur.Position,
	}
	if body.Text != nil {
		next.Text = *body.Text
	}
	if body.Status != nil {
		if !validLeadStatus[*body.Status] {
			httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "status 只能是 open/connected/pruned", nil))
			return
		}
		next.Status = *body.Status
	}
	if body.ConnectedReferenceID != nil {
		pg, err := parseNullableUUID(body.ConnectedReferenceID)
		if err != nil {
			httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "connectedReferenceId 不是有效的 id", nil))
			return
		}
		if pg.Valid {
			if _, err := a.d.Queries.GetReferenceForProject(r.Context(), sqlc.GetReferenceForProjectParams{
				ID: uuid.UUID(pg.Bytes), ProjectID: projectID,
			}); err != nil {
				httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "connectedReferenceId 不是这个项目里的来源", nil))
				return
			}
		}
		next.ConnectedReferenceID = pg
	}

	row, err := a.d.Queries.UpdateExplorationLead(r.Context(), next)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"lead": toExplorationLeadDTO(row)})
}

// -- DELETE /exploration/leads/{lid} ----------------------------------------

// deleteExplorationLead hard-deletes a lead (manual/mistaken adds). IDOR-
// guarded by the WHERE id AND project_id in the query itself, but we still
// 404 up front via GetExplorationLeadForProject so a stray id in another
// project can't even be probed for existence.
func (a *API) deleteExplorationLead(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	lid, err := uuid.Parse(r.PathValue("lid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	if _, err := a.d.Queries.GetExplorationLeadForProject(r.Context(), sqlc.GetExplorationLeadForProjectParams{ID: lid, ProjectID: projectID}); err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	if err := a.d.Queries.DeleteExplorationLead(r.Context(), sqlc.DeleteExplorationLeadParams{ID: lid, ProjectID: projectID}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// -- POST /exploration/guide (METERED — the ONLY LLM spend in this file) ---

// guideDirectionDTO mirrors agent.GuideDirection; already camelCase
// (direction/why) and matches contracts's GuideDirection field-for-field.
type guideDirectionDTO struct {
	Direction string `json:"direction"`
	Why       string `json:"why"`
}

// toGuideDirectionDTOs never returns nil — the wire contract for this
// endpoint is `[]`, not `null` (nonNilStrings's convention, reading_takeaway.go).
func toGuideDirectionDTOs(ds []agent.GuideDirection) []guideDirectionDTO {
	out := make([]guideDirectionDTO, 0, len(ds))
	for _, d := range ds {
		out = append(out, guideDirectionDTO{Direction: d.Direction, Why: d.Why})
	}
	return out
}

// assembleExplorationGuideInput projects the project's exploration graph
// (engaged sources + open leads + proposal objective) into a pure
// agent.ExplorationGuideInput — no LLM call here, just reads. Per-source
// State mirrors buildSpineProjection's 文献库 block (projectcoach.go) exactly:
// 已归纳 once takeaway_finalized_at is set AND a takeaway body actually
// exists, else 在读 once a material is linked, else 未读. Tolerates a
// missing proposal (fresh project) by simply leaving ProposalObjective "".
func (a *API) assembleExplorationGuideInput(ctx context.Context, projectID uuid.UUID) agent.ExplorationGuideInput {
	var in agent.ExplorationGuideInput
	if prop, err := a.d.Queries.GetProjectProposal(ctx, projectID); err == nil {
		in.ProposalObjective = prop.Objective
	}
	if refs, err := a.d.Queries.ListReferences(ctx, projectID); err == nil {
		for _, ref := range refs {
			state := "未读"
			switch {
			case ref.TakeawayFinalizedAt.Valid && len(ref.Takeaway) > 0:
				state = "已归纳"
			case ref.MaterialID.Valid:
				state = "在读"
			}
			var decision, credibility, phase string
			if ref.Decision != nil {
				decision = *ref.Decision
			}
			if ref.Credibility != nil {
				credibility = *ref.Credibility
			}
			if ref.PhaseTag != nil {
				phase = *ref.PhaseTag
			}
			in.Sources = append(in.Sources, agent.ExplorationGraphSource{
				Title: ref.Title, Decision: decision, Credibility: credibility, PhaseTag: phase, State: state,
			})
		}
	}
	if leads, err := a.d.Queries.ListExplorationLeads(ctx, projectID); err == nil {
		for _, l := range leads {
			switch l.Status {
			case "open":
				in.OpenLeads = append(in.OpenLeads, l.Text)
			case "pruned":
				in.PrunedCount++
			case "connected":
				in.ConnectedCount++
			}
		}
	}
	return in
}

// postExplorationGuide is the "深挖一层" guide — the ONLY LLM-spending
// endpoint in S3's exploration surface. Points the student at the next
// necessary research direction from her own exploration graph (engaged
// sources + open leads), never fetching anything or concluding her research
// for her (铁律 · 克制). Clones getTakeawayDraft's discipline
// (reading_takeaway.go:144) exactly: HasEntitlement gate, assemble the input
// deterministically, and if the graph is empty (agent.HasGraphContent false)
// skip the WHOLE resolver/compose/meter block — an empty graph must never
// resolve a provider or touch the llm_call audit trail, not just skip
// metering. Metering only fires when resolved.Provider != "" AND the compose
// call completed without error (an llm_call row represents a COMPLETED
// call — a resolver success with a failed compose must not phantom-record a
// 0-token/$0 row); a compose error still returns 200 with empty directions,
// never 500 — the student can always keep going without the guide. No persistence:
// the guide only proposes, the student decides whether to turn a direction
// into a lead via createExplorationLead.
func (a *API) postExplorationGuide(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	u, _ := UserFromContext(r.Context())
	if entitled, err := HasEntitlement(r.Context(), u); err != nil {
		httpx.WriteError(w, r, err)
		return
	} else if !entitled {
		httpx.WriteError(w, r, httpx.ErrNotEntitled())
		return
	}

	// #12/#13 · optional focus: dig deeper from ONE lead, carrying the student's
	// own thinking. Absent body → whole-graph 深挖一层 (unchanged).
	var body struct {
		LeadID  string `json:"leadId"`
		Thought string `json:"thought"`
	}
	_ = decodeJSON(r, &body) // best-effort; an empty/absent body is valid

	in := a.assembleExplorationGuideInput(r.Context(), projectID)
	if strings.TrimSpace(body.LeadID) != "" {
		if lid, perr := uuid.Parse(body.LeadID); perr == nil {
			if lead, lerr := a.d.Queries.GetExplorationLeadForProject(r.Context(), sqlc.GetExplorationLeadForProjectParams{ID: lid, ProjectID: projectID}); lerr == nil {
				in.FocusLead = lead.Text
			}
		}
	}
	in.Thought = strings.TrimSpace(body.Thought)

	var directions []agent.GuideDirection
	if agent.HasGraphContent(in) {
		if resolved, rerr := a.d.ChatResolver(r.Context()); rerr == nil {
			ds, usage, cerr := agent.ComposeExplorationGuide(r.Context(), a.d.Provider, resolved, in)
			if resolved.Provider != "" && cerr == nil {
				store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
				if e := store.RecordLLMCall(r.Context(), agent.LLMCallRow{
					ProjectID: projectID, Surface: "studio", Purpose: "exploration_guide",
					Resolved: resolved, PromptTokens: int32(usage.InputTokens), CompletionTokens: int32(usage.OutputTokens),
				}); e != nil {
					slog.Warn("exploration guide: record llm", "err", e, "request_id", httpx.RequestIDFromContext(r.Context()))
				}
			}
			if cerr == nil {
				directions = ds
			}
		}
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"directions": toGuideDirectionDTOs(directions)})
}

// -- POST /exploration/dig (OpenAlex, NOT an LLM — no llm_call, no persist) -

// digCandidateDTO mirrors contracts's DigCandidate — camelCase over
// materialize.WorkMeta, straight to the client's tray. Adopting a candidate
// into the map is a separate, explicit student action (铁律①); this endpoint
// only surfaces options.
type digCandidateDTO struct {
	DOI      string `json:"doi"`
	Title    string `json:"title"`
	Authors  string `json:"authors"`
	Year     string `json:"year"`
	Journal  string `json:"journal"`
	Abstract string `json:"abstract"`
	URL      string `json:"url"`
}

func toDigCandidateDTOs(works []materialize.WorkMeta) []digCandidateDTO {
	out := make([]digCandidateDTO, 0, len(works))
	for _, w := range works {
		out = append(out, digCandidateDTO{
			DOI: w.DOI, Title: w.Title, Authors: w.Authors, Year: w.Year,
			Journal: w.Journal, Abstract: w.Abstract, URL: w.URL,
		})
	}
	return out
}

// digExploration runs an OpenAlex search from a free-text keyword, or from a
// lead's own question text, and hands candidates straight to the client-side
// tray — no persistence (adopting is a separate, explicit student action,
// 铁律①). Before searching, 印记 refines the student's own question (often a
// long Chinese sentence) into a short English keyword query — OpenAlex
// relevance on raw natural-language Chinese text is poor, and this refine is
// the whole value of dig (Task A8). That refine IS one real LLM call, so
// (unlike the search itself) it IS metered — one llm_call per dig when a
// provider is configured. On any refine error, empty result, or no provider
// configured, this falls back to the raw query text and never 500s. A
// keyword wins when both are supplied. reference has no DOI column today
// (only a free-text url), so the RelatedWorks branch — widening around a
// lead's already-connected paper instead of re-searching its question text —
// is left for a follow-up once a DOI is actually resolvable off a reference;
// SearchWorks covers both keyword and lead-text digs for now.
func (a *API) digExploration(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	u, _ := UserFromContext(r.Context())
	if entitled, err := HasEntitlement(r.Context(), u); err != nil {
		httpx.WriteError(w, r, err)
		return
	} else if !entitled {
		httpx.WriteError(w, r, httpx.ErrNotEntitled())
		return
	}

	var body struct {
		LeadID  *string `json:"leadId"`
		Keyword *string `json:"keyword"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	query := ""
	if body.Keyword != nil {
		query = strings.TrimSpace(*body.Keyword)
	}
	if query == "" && body.LeadID != nil && strings.TrimSpace(*body.LeadID) != "" {
		if lid, perr := uuid.Parse(*body.LeadID); perr == nil {
			if lead, lerr := a.d.Queries.GetExplorationLeadForProject(r.Context(), sqlc.GetExplorationLeadForProjectParams{ID: lid, ProjectID: projectID}); lerr == nil {
				query = lead.Text
			}
		}
	}
	if query == "" {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"candidates": toDigCandidateDTOs(nil)})
		return
	}

	// 印记 refines the raw question into a short English keyword query before
	// OpenAlex ever sees it. Gated so tests/deploys without a configured
	// provider are unaffected, and any failure falls back to the raw text —
	// dig must never 500 because the refine step didn't work out.
	searchQuery := query
	if a.d.ChatResolver != nil {
		if resolved, rerr := a.d.ChatResolver(r.Context()); rerr == nil && resolved.Provider != "" {
			refined, usage, cerr := agent.ComposeDigQuery(r.Context(), a.d.Provider, resolved, query)
			if cerr == nil && strings.TrimSpace(refined) != "" {
				store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
				if e := store.RecordLLMCall(r.Context(), agent.LLMCallRow{
					ProjectID: projectID, Surface: "studio", Purpose: "dig_query",
					Resolved: resolved, PromptTokens: int32(usage.InputTokens), CompletionTokens: int32(usage.OutputTokens),
				}); e != nil {
					slog.Warn("dig query: record llm", "err", e, "request_id", httpx.RequestIDFromContext(r.Context()))
				}
				searchQuery = refined
			}
		}
	}

	works := a.d.Fetcher.SearchWorks(r.Context(), searchQuery, 8)
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"candidates": toDigCandidateDTOs(works)})
}

// -- POST /exploration/adopt (no LLM, no metering) --------------------------

// adoptExploration is the ONLY way a tray candidate (digExploration's output)
// becomes a node on the map — 铁律①: the AI only ever surfaces a tray, the
// student's explicit "adopt" is what commits it. One transaction creates BOTH
// the reference (bibliographic fields from the candidate, auto-shelved
// reading_status="reading" — a source the student just pulled off the dig
// tray is, by construction, one she's about to read) AND the lead that
// connects to it (origin="guide" — it was surfaced by a dig, not typed in
// manually; status="connected" since it's born already answered;
// connectedReferenceId=the new reference; parentLeadId=the dug node, or
// absent for a fresh top-level thread) — both rows land together or neither
// does, so the map never shows a floating half-created reference with no
// lead, or a lead pointing at a reference that doesn't exist.
func (a *API) adoptExploration(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	var body struct {
		ParentLeadID *string         `json:"parentLeadId"`
		Candidate    digCandidateDTO `json:"candidate"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if strings.TrimSpace(body.Candidate.Title) == "" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "候选的标题不能是空的", nil))
		return
	}

	// Optional parent: validate it's a lead in THIS project (IDOR), exactly as
	// createExplorationLead does for a manual 分支.
	var parent pgtype.UUID
	if body.ParentLeadID != nil && strings.TrimSpace(*body.ParentLeadID) != "" {
		pid, perr := uuid.Parse(*body.ParentLeadID)
		if perr != nil {
			httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "parentLeadId 不是有效的 id", nil))
			return
		}
		if _, err := a.d.Queries.GetExplorationLeadForProject(r.Context(), sqlc.GetExplorationLeadForProjectParams{ID: pid, ProjectID: projectID}); err != nil {
			httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "parentLeadId 不是这个项目里的线索", nil))
			return
		}
		parent = pgtype.UUID{Bytes: pid, Valid: true}
	}

	// position = count of existing children under the same parent (createExplorationLead's
	// convention, scoped to this parent rather than the whole project).
	existing, err := a.d.Queries.ListExplorationLeads(r.Context(), projectID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var position int32
	for _, l := range existing {
		if l.ParentLeadID == parent {
			position++
		}
	}

	url := strings.TrimSpace(body.Candidate.URL)
	if url == "" && strings.TrimSpace(body.Candidate.DOI) != "" {
		url = "https://doi.org/" + strings.TrimSpace(body.Candidate.DOI)
	}

	tx, err := a.d.Pool.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	qtx := a.d.Queries.WithTx(tx)

	ref, err := qtx.CreateReference(r.Context(), sqlc.CreateReferenceParams{
		ProjectID:   projectID,
		Title:       body.Candidate.Title,
		Author:      body.Candidate.Authors,
		Year:        body.Candidate.Year,
		Url:         url,
		Tags:        stringsToJSONB(nil),
		Evaluation:  "",
		SearchHints: stringsToJSONB(nil),
		Abstract:    body.Candidate.Abstract,
		Journal:     body.Candidate.Journal,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	// CreateReference has no reading_status param — new rows take the column's
	// DB default ("to_read"); UpdateReference (same query patchReference uses)
	// is the only writer of reading_status, so auto-shelving to "reading" goes
	// through it, still inside this transaction so both rows commit together.
	ref, err = qtx.UpdateReference(r.Context(), sqlc.UpdateReferenceParams{
		ID: ref.ID, ProjectID: projectID,
		Title: ref.Title, Classification: ref.Classification, Author: ref.Author, Credentials: ref.Credentials,
		Year: ref.Year, Url: ref.Url, Tags: ref.Tags, CollectionID: ref.CollectionID,
		Credibility: ref.Credibility, Evaluation: ref.Evaluation, Decision: ref.Decision,
		Pending: ref.Pending, SearchHints: ref.SearchHints, ReadingNote: ref.ReadingNote,
		Abstract: ref.Abstract, Journal: ref.Journal, ReadingStatus: "reading",
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	lead, err := qtx.CreateExplorationLead(r.Context(), sqlc.CreateExplorationLeadParams{
		ProjectID:            projectID,
		Text:                 body.Candidate.Title,
		Status:               "connected",
		Origin:               "guide",
		ConnectedReferenceID: pgtype.UUID{Bytes: ref.ID, Valid: true},
		Position:             position,
		ParentLeadID:         parent,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	httpx.WriteJSON(w, http.StatusCreated, map[string]any{
		"lead":      toExplorationLeadDTO(lead),
		"reference": toReferenceDTO(ref, nil),
	})
}

// -- B2 · question_edge lifecycle (POST/PATCH/DELETE) -----------------------
//
// create/relabel/confirm/dismiss for the labeled edges between top-level
// question leads (B1 laid the table + GET projection; this wires the write
// path). 铁律① (克制): AI-proposed edges — wherever a future caller creates
// one — land status:"proposed" until the student confirms; a student-created
// edge (this endpoint, called from the student's own UI action) is confirmed
// on arrival, no separate confirm step needed for something she typed
// herself. 铁律④ (过程即数据): relabeling/dismissing is a normal, never
// hard-blocked student action — DELETE is a hard delete, not a soft
// "declined" state, matching deleteExplorationLead's own discipline.
//
// CRITICAL IDOR note (carried forward from B1 review): CreateQuestionEdge
// the sqlc query does NOT verify from_lead_id/to_lead_id belong to this
// project — it only requires they exist somewhere in the table. Every write
// handler below MUST resolve both endpoints through getRootLeadForProject
// (which chains GetExplorationLeadForProject's own project_id-scoped WHERE)
// before ever calling Create/UpdateQuestionEdge — that resolve is the IDOR
// guard, not the INSERT/UPDATE statement itself.

// createQuestionEdge wires POST /exploration/edges: both endpoints must be
// leads inside THIS project (IDOR) and both must be root question nodes (no
// 分支 endpoints), label must be one of the closed 5, and self-edges are
// rejected as a degenerate case that can't mean anything on this graph.
func (a *API) createQuestionEdge(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	var body struct {
		FromLeadID string `json:"fromLeadId"`
		ToLeadID   string `json:"toLeadId"`
		Label      string `json:"label"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if !validQuestionEdgeLabel[body.Label] {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "label 必须是固定小词表里的一个", nil))
		return
	}
	fromID, err := uuid.Parse(body.FromLeadID)
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "fromLeadId 不是有效的 id", nil))
		return
	}
	toID, err := uuid.Parse(body.ToLeadID)
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "toLeadId 不是有效的 id", nil))
		return
	}
	if fromID == toID {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "一个问题不能连到它自己", nil))
		return
	}
	if _, err := a.getRootLeadForProject(r.Context(), projectID, fromID); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "fromLeadId 不是这个项目里的顶层问题", nil))
		return
	}
	if _, err := a.getRootLeadForProject(r.Context(), projectID, toID); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "toLeadId 不是这个项目里的顶层问题", nil))
		return
	}

	row, err := a.d.Queries.CreateQuestionEdge(r.Context(), sqlc.CreateQuestionEdgeParams{
		ProjectID:  projectID,
		FromLeadID: fromID,
		ToLeadID:   toID,
		Label:      body.Label,
		// Student-created (this endpoint is only reachable from the student's
		// own map action) → confirmed on arrival, unlike a future AI-proposed
		// edge which would land "proposed".
		Status: "confirmed",
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"edge": toQuestionEdgeDTO(row)})
}

// getQuestionEdgeForProject is the IDOR-guarded lookup PATCH/DELETE share —
// question_edge has no per-id sqlc getter (only ListQuestionEdgesByProject),
// so this scans the project's own edge list rather than trusting a bare id.
func (a *API) getQuestionEdgeForProject(ctx context.Context, projectID, edgeID uuid.UUID) (sqlc.QuestionEdge, error) {
	edges, err := a.d.Queries.ListQuestionEdgesByProject(ctx, projectID)
	if err != nil {
		return sqlc.QuestionEdge{}, err
	}
	for _, e := range edges {
		if e.ID == edgeID {
			return e, nil
		}
	}
	return sqlc.QuestionEdge{}, sql404NotFound
}

// sql404NotFound is a sentinel for getQuestionEdgeForProject's not-found
// case — no row means no row, distinct from a real query error.
var sql404NotFound = errors.New("question_edge not found in project")

// patchQuestionEdge wires PATCH /exploration/edges/{eid}: relabel and/or
// proposed→confirmed, absent-means-keep over the current row (patchLead's
// merge pattern). label/status are validated against their closed sets when
// present; the underlying UPDATE is already project-scoped (belt & braces
// alongside the 404 probe above).
func (a *API) patchQuestionEdge(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	eid, err := uuid.Parse(r.PathValue("eid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	cur, err := a.getQuestionEdgeForProject(r.Context(), projectID, eid)
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}

	var body struct {
		Label  *string `json:"label"`
		Status *string `json:"status"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	next := sqlc.UpdateQuestionEdgeParams{ID: eid, ProjectID: projectID, Label: cur.Label, Status: cur.Status}
	if body.Label != nil {
		if !validQuestionEdgeLabel[*body.Label] {
			httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "label 必须是固定小词表里的一个", nil))
			return
		}
		next.Label = *body.Label
	}
	if body.Status != nil {
		if !validQuestionEdgeStatus[*body.Status] {
			httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "status 只能是 proposed/confirmed", nil))
			return
		}
		next.Status = *body.Status
	}

	row, err := a.d.Queries.UpdateQuestionEdge(r.Context(), next)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"edge": toQuestionEdgeDTO(row)})
}

// deleteQuestionEdge wires DELETE /exploration/edges/{eid}: dismiss (hard
// delete, matching deleteExplorationLead's own discipline) — 铁律④ treats
// this as a normal, always-available student action, never hard-blocked.
func (a *API) deleteQuestionEdge(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	eid, err := uuid.Parse(r.PathValue("eid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	if _, err := a.getQuestionEdgeForProject(r.Context(), projectID, eid); err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	if err := a.d.Queries.DeleteQuestionEdge(r.Context(), sqlc.DeleteQuestionEdgeParams{ID: eid, ProjectID: projectID}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
