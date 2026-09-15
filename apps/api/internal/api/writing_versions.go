package api

// writing_versions.go — submitted versions of a lite writing (0153): the lock
// facts, inserting a version, editing after 完成, and reading versions.

import (
	"context"
	"errors"
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
