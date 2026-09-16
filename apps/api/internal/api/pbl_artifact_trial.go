package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/pbl"
	"mindimprint/api/internal/store/sqlc"
)

type artifactTrialDraftDTO struct {
	Document pbl.ArtifactTrial `json:"document"`
	Revision int32             `json:"revision"`
}

func trialDraftDTO(row sqlc.PblArtifactTrialDraft) (artifactTrialDraftDTO, error) {
	out := artifactTrialDraftDTO{Revision: row.Revision}
	err := json.Unmarshal(row.Document, &out.Document)
	return out, err
}

func (a *API) ownedTrialArtifact(w http.ResponseWriter, r *http.Request) (sqlc.GetPblArtifactRow, bool) {
	atom, ok := a.loadOwnedPblProject(w, r)
	if !ok {
		return sqlc.GetPblArtifactRow{}, false
	}
	id, err := uuid.Parse(r.PathValue("aid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("成果不存在"))
		return sqlc.GetPblArtifactRow{}, false
	}
	row, err := a.d.Queries.GetPblArtifact(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && row.AtomID != atom) {
		httpx.WriteError(w, r, httpx.ErrNotFound("成果不存在"))
		return row, false
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return row, false
	}
	return row, true
}

func (a *API) getPblArtifactTrialDraft(w http.ResponseWriter, r *http.Request) {
	artifact, ok := a.ownedTrialArtifact(w, r)
	if !ok {
		return
	}
	row, err := a.d.Queries.GetPblArtifactTrialDraft(r.Context(), artifact.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.WriteJSON(w, http.StatusOK, artifactTrialDraftDTO{Document: pbl.EmptyArtifactTrial()})
		return
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out, err := trialDraftDTO(row)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

func (a *API) savePblArtifactTrialDraft(w http.ResponseWriter, r *http.Request) {
	artifact, ok := a.ownedTrialArtifact(w, r)
	if !ok {
		return
	}
	var req struct {
		Document pbl.ArtifactTrial `json:"document"`
		Revision *int32            `json:"revision"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 50000))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		httpx.WriteError(w, r, errBadJSON(err))
		return
	}
	if req.Revision == nil || *req.Revision < 0 {
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_revision", "保存失败：缺少有效草稿版本", nil))
		return
	}
	if err := req.Document.Validate(false); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_trial", err.Error(), nil))
		return
	}
	if err := a.d.Queries.EnsurePblArtifactTrialDraft(r.Context(), artifact.ID); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	raw, _ := json.Marshal(req.Document)
	row, err := a.d.Queries.SavePblArtifactTrialDraft(r.Context(), sqlc.SavePblArtifactTrialDraftParams{ArtifactID: artifact.ID, Document: raw, Revision: *req.Revision})
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.WriteError(w, r, httpx.ErrConflict("试用草稿已在其他页面修改，请比较后保存"))
		return
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out, err := trialDraftDTO(row)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

// Create the entry and clear its draft in one transaction. An immediate retry
// with the submitted revision returns that entry, without touching newer work.
func (a *API) submitPblArtifactTrialDraft(w http.ResponseWriter, r *http.Request) {
	artifact, ok := a.ownedTrialArtifact(w, r)
	if !ok {
		return
	}
	var req struct {
		Revision *int32 `json:"revision"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		httpx.WriteError(w, r, errBadJSON(err))
		return
	}
	if req.Revision == nil || *req.Revision < 0 {
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_revision", "保存失败：缺少有效草稿版本", nil))
		return
	}
	tx, err := a.d.Pool.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	q := a.d.Queries.WithTx(tx)
	// The atom lock serializes submission; the draft's conditional UPDATE also
	// detects autosaves arriving while this transaction validates its snapshot.
	if _, err = q.LockAtom(r.Context(), artifact.AtomID); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	row, err := q.GetPblArtifactTrialDraft(r.Context(), artifact.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.WriteError(w, r, httpx.ErrConflict("请先保存试用草稿"))
		return
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if row.SubmittedRevision != nil && *row.SubmittedRevision == *req.Revision && row.SubmittedEntryID.Valid {
		entry, err := q.GetPblTrialSubmittedEntry(r.Context(), sqlc.GetPblTrialSubmittedEntryParams{ID: uuid.UUID(row.SubmittedEntryID.Bytes), AtomID: artifact.AtomID})
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		draft, err := trialDraftDTO(row)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"entry": toPblKeepDTO(entry), "draft": draft})
		return
	}
	if row.Revision != *req.Revision {
		httpx.WriteError(w, r, httpx.ErrConflict("试用草稿已修改，请核对后重新保存记录"))
		return
	}
	draft, err := trialDraftDTO(row)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err = draft.Document.Validate(true); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("incomplete_trial", err.Error(), nil))
		return
	}
	kind, stage := "feedback", "observe"
	if draft.Document.Mode == "not_tested" {
		kind, stage = "thought", "change"
	}
	entry, err := q.CreatePblKeepEntry(r.Context(), sqlc.CreatePblKeepEntryParams{AtomID: artifact.AtomID, Kind: kind, Stage: stage, Body: draft.Document.Body(artifact.ID.String(), artifact.Title)})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	empty, _ := json.Marshal(pbl.EmptyArtifactTrial())
	updated, err := q.SubmitPblArtifactTrialDraft(r.Context(), sqlc.SubmitPblArtifactTrialDraftParams{ArtifactID: artifact.ID, Document: empty, SubmittedEntryID: pgtype.UUID{Bytes: entry.ID, Valid: true}, Revision: row.Revision})
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.WriteError(w, r, httpx.ErrConflict("试用草稿已修改，请核对后重新保存记录"))
		return
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out, err := trialDraftDTO(updated)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"entry": toPblKeepDTO(entry), "draft": out})
}
