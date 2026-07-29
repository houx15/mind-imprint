package api

// reading_brief.go — S2 · brief-in. Why-read-THIS-source is captured at entry
// (pre-filled by suggestReadingReason, student edits), persisted on the
// reference, and injected into every read-turn so the coach's questions are
// purposeful. 克制: the AI only seeds the default; the student authors intent.

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

type readingBriefReq struct {
	ReadingReason string `json:"reading_reason"`
	ReadingFocus  string `json:"reading_focus"`
	PhaseTag      string `json:"phase_tag"`
}

// suggestReadingReason templates a first-draft intention deterministically (no
// model call) from the proposal objective + the source title.
func suggestReadingReason(proposalObjective, refTitle string) string {
	title := strings.TrimSpace(refTitle)
	if title == "" {
		title = "这篇材料"
	}
	obj := strings.TrimSpace(proposalObjective)
	if obj == "" {
		return "读《" + title + "》想弄清什么？"
	}
	return "带着「" + obj + "」的问题读《" + title + "》，我想验证："
}

// putReadingBrief persists why-read-THIS-source on the reference row. Editable
// any time (the room shows it as a persistent banner). project-scoped via
// GetReferenceForProject-shaped params (the IDOR guard), same pattern as
// patchReference/enterReading.
func (a *API) putReadingBrief(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	rid, err := uuid.Parse(r.PathValue("rid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	var body readingBriefReq
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	row, err := a.d.Queries.UpdateReadingBrief(r.Context(), sqlc.UpdateReadingBriefParams{
		ID: rid, ProjectID: projectID,
		ReadingReason: normOrNil(body.ReadingReason),
		ReadingFocus:  normOrNil(body.ReadingFocus),
		PhaseTag:      normOrNil(body.PhaseTag),
	})
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	// Notes re-project from the reference's (possibly unchanged) material —
	// same pattern as patchReference (workspace_library.go), which also edits
	// this row for an unrelated field and must not let a passed-nil notes arg
	// wipe already-projected reading notes in the response.
	var notes []readingNoteDTO
	if row.MaterialID.Valid {
		notes = a.notesByMaterial(r, projectID)[uuid.UUID(row.MaterialID.Bytes).String()]
	}
	// referenceDTO/toReferenceDTO don't carry the brief fields yet (that's
	// Task 6/8's job, alongside the structured takeaway); echo what was just
	// persisted as sibling top-level fields so the room can confirm the save
	// without waiting on that DTO extension.
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"reference":     toReferenceDTO(row, notes),
		"readingReason": derefOr(row.ReadingReason, ""),
		"readingFocus":  derefOr(row.ReadingFocus, ""),
		"phaseTag":      derefOr(row.PhaseTag, ""),
	})
}

// derefOr returns *p, or fallback when p is nil.
func derefOr(p *string, fallback string) string {
	if p == nil {
		return fallback
	}
	return *p
}

// normOrNil trims s and returns nil for an empty result. Full-replace PUT
// semantics (clearing a field by sending "") must persist NULL, never "" —
// an empty string stored in phase_tag serializes as `"phaseTag": ""`, which
// fails the Zod PhaseTag enum (packages/contracts/src/reference.ts) on every
// downstream read (getLibrary/postFinalizeReading), breaking the whole
// Library once any source has an empty-phase brief.
func normOrNil(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}

// readingBriefFor loads the persisted brief for a material's reference (for
// read-turn injection, Task 3). Best-effort: a material with no reference or an
// unset brief returns a zero ReadingBrief and the loop degrades to today.
func (a *API) readingBriefFor(ctx context.Context, projectID, materialID uuid.UUID) agent.ReadingBrief {
	var br agent.ReadingBrief
	// reference carries the brief; find it by material_id within the project.
	if refs, err := a.d.Queries.ListReferences(ctx, projectID); err == nil {
		for _, ref := range refs {
			if ref.MaterialID.Valid && uuid.UUID(ref.MaterialID.Bytes) == materialID {
				if ref.ReadingReason != nil {
					br.Reason = *ref.ReadingReason
				}
				if ref.ReadingFocus != nil {
					br.Focus = *ref.ReadingFocus
				}
				if ref.PhaseTag != nil {
					br.PhaseTag = *ref.PhaseTag
				}
				break
			}
		}
	}
	if prop, err := a.d.Queries.GetProjectProposal(ctx, projectID); err == nil {
		br.ProposalSnap = firstNonEmpty(prop.Objective, prop.Reason)
	}
	return br
}

// firstNonEmpty returns the first non-blank string among ss, or "".
func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}
