package api

import (
	"context"
	"encoding/json"
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

// -- GET /exploration -----------------------------------------------------

// getExploration returns every lead for the project plus danglingSourceIds —
// no spend, purely a projection over ListExplorationLeads + ListReferences.
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
	dtos := make([]explorationLeadDTO, 0, len(leads))
	for _, l := range leads {
		dtos = append(dtos, toExplorationLeadDTO(l))
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"leads":             dtos,
		"danglingSourceIds": computeDanglingSourceIds(refs, leads),
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
// tray — no persistence, no metering (OpenAlex is not an LLM; a fetch isn't a
// spend). A keyword wins when both are supplied. reference has no DOI column
// today (only a free-text url), so the RelatedWorks branch — widening around
// a lead's already-connected paper instead of re-searching its question text
// — is left for a follow-up once a DOI is actually resolvable off a
// reference; SearchWorks covers both keyword and lead-text digs for now.
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

	works := a.d.Fetcher.SearchWorks(r.Context(), query, 8)
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"candidates": toDigCandidateDTOs(works)})
}
