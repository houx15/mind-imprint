package api

// writing_versions.go — submitted versions of a lite writing (0153): the lock
// facts, inserting a version, editing after 完成, and reading versions.

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/liteassign"
	"mindimprint/api/internal/store/sqlc"
)

// writingLockFacts reads what liteassign.Locked needs for one writing.
// ownerID is the atom's owner: the recipient row is keyed by her user id.
func writingLockFacts(ctx context.Context, q *sqlc.Queries, atomID, ownerID uuid.UUID) (liteassign.LockFacts, error) {
	var f liteassign.LockFacts
	n, err := q.CountWritingVersions(ctx, atomID)
	if err != nil {
		return f, err
	}
	f.HasVersion = n > 0
	row, err := q.GetLiteAssignmentForAtom(ctx, sqlc.GetLiteAssignmentForAtomParams{
		AtomID: pgtype.UUID{Bytes: atomID, Valid: true}, UserID: ownerID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return f, nil
	}
	if err != nil {
		return f, err
	}
	f.Homework = true
	f.DueAt = row.DueAt
	f.ReturnedAt = tsPtr(row.ReturnedAt)
	f.ReturnDueAt = tsPtr(row.ReturnDueAt)
	return f, nil
}

// refuseIfWritingLocked is the one place the three-step lock check lives:
// writingLockFacts → liteassign.Locked → httpx.ErrWritingLocked. It returns
// nil when the writing may still be written to, and the 403 writing_locked
// APIError (satisfying the error interface) when it may not. Finish, and
// later revise/discard/the write gate (Task 4), all call this instead of
// inlining the check, so the rule has exactly one place to change.
func refuseIfWritingLocked(ctx context.Context, q *sqlc.Queries, atomID, ownerID uuid.UUID, now time.Time) error {
	facts, err := writingLockFacts(ctx, q, atomID, ownerID)
	if err != nil {
		return err
	}
	if liteassign.Locked(facts, now) {
		return httpx.ErrWritingLocked()
	}
	return nil
}

// insertWritingVersion adds the next version. The caller holds the writing
// row lock (GetWritingForUpdate) in the same transaction, so two finishes
// cannot compute the same number; UNIQUE(atom_id, number) backs that up.
func insertWritingVersion(ctx context.Context, q *sqlc.Queries, atomID uuid.UUID, title, body string) (sqlc.WritingVersion, error) {
	n, err := q.NextWritingVersionNumber(ctx, atomID)
	if err != nil {
		return sqlc.WritingVersion{}, err
	}
	return q.CreateWritingVersion(ctx, sqlc.CreateWritingVersionParams{
		AtomID: atomID, Number: n, Title: title, Body: body,
		WordCount: int32(agent.CountWords(body)),
	})
}

// writingWriteGateForRow is the finished/revising/lock rule itself, evaluated
// against a writing row the caller already has: open → nil; finished, not
// revising → writing_finished; finished, revising → refuseIfWritingLocked
// (the one place the lock rule lives). Two shapes call this:
//
//   - writingWriteGate (below) — loadOwnedAtom's fast pre-check, reading
//     with a.d.Queries (no lock, not in any transaction). It exists to fail
//     obviously-closed writes early and to gate the last_activity_at touch;
//     it is NOT the authority for anything that also writes writing_draft.
//   - putWritingDraft / composeWritingDraft — the authority. Both re-run
//     this gate against a row taken with GetWritingForUpdate inside their
//     own transaction, immediately before the writing_draft upsert, using
//     the SAME tx's queries for the lock check. That is what actually
//     closes the race the pre-check cannot: without it, an upsert whose
//     pre-check ran while she was still revising could still commit AFTER a
//     concurrent discardWritingRevision or finishWritingAtom transaction had
//     already restored/replaced writing_draft and cleared revising_at,
//     silently reviving text she had just discarded (or superseding a fresh
//     version with a stale one). Taking GetWritingForUpdate first makes the
//     gate check and the write serialize with discard/finish's own
//     GetWritingForUpdate on the same row.
func writingWriteGateForRow(ctx context.Context, q *sqlc.Queries, wr sqlc.Writing, ownerID uuid.UUID) error {
	if wr.Status != "finished" {
		return nil
	}
	if !wr.RevisingAt.Valid {
		return httpx.ErrWritingFinished()
	}
	return refuseIfWritingLocked(ctx, q, wr.AtomID, ownerID, time.Now())
}

// writingWriteGate is the writing branch of loadOwnedAtom's write gate — see
// writingWriteGateForRow's comment for why this is a fast pre-check, not the
// authority, for any handler that itself writes writing_draft.
func (a *API) writingWriteGate(ctx context.Context, at sqlc.Atom) error {
	wr, err := a.d.Queries.GetWriting(ctx, at.ID)
	if err != nil {
		return err
	}
	return writingWriteGateForRow(ctx, a.d.Queries, wr, at.UserID)
}

// reviseWriting is POST /api/v1/writings/{id}/revise (修改): reopen a
// finished writing for editing. Status stays "finished" the whole time, so a
// homework this belongs to stays 已提交 — only revisingAt changes, which is
// what writingWriteGate and the lock check key off of.
func (a *API) reviseWriting(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedWritingAtomRow(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	wr, err := a.d.Queries.GetWriting(ctx, at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if wr.Status != "finished" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("writing_not_finished", "这篇写作还没有提交", nil))
		return
	}
	if err := refuseIfWritingLocked(ctx, a.d.Queries, at.ID, at.UserID, time.Now()); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	updated, err := a.d.Queries.SetWritingRevising(ctx, at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, writingDTOOf(updated, at.CreatedAt, at.LastActivityAt))
}

// discardWritingRevision is POST /api/v1/writings/{id}/revise/discard
// (放弃修改): the draft body and the title go back to the latest submitted
// version and revisingAt is cleared, so writingWriteGate closes writes again.
// A no-op 200 when she is not revising. Refused when locked, so edits made
// before the deadline stay in writing_draft until the teacher returns the
// homework — discard is itself a write, so it must not be able to bypass the
// lock it is trying to close.
func (a *API) discardWritingRevision(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedWritingAtomRow(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	tx, err := a.d.Pool.Begin(ctx)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := a.d.Queries.WithTx(tx)

	wr, err := qtx.GetWritingForUpdate(ctx, at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if !wr.RevisingAt.Valid {
		httpx.WriteJSON(w, http.StatusOK, writingDTOOf(wr, at.CreatedAt, at.LastActivityAt))
		return
	}
	if err := refuseIfWritingLocked(ctx, qtx, at.ID, at.UserID, time.Now()); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	latest, err := qtx.GetLatestWritingVersion(ctx, at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if _, err := qtx.UpsertWritingDraft(ctx, sqlc.UpsertWritingDraftParams{AtomID: at.ID, Body: latest.Body}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := qtx.RenameWriting(ctx, sqlc.RenameWritingParams{AtomID: at.ID, Title: latest.Title}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := qtx.ClearWritingRevising(ctx, at.ID); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := tx.Commit(ctx); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	updated, err := a.d.Queries.GetWriting(ctx, at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, writingDTOOf(updated, at.CreatedAt, at.LastActivityAt))
}
