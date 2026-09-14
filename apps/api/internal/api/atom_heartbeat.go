package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/liteweek"
	"mindimprint/api/internal/store/sqlc"
)

// atom_heartbeat.go — the smallest honest way to know how long she was
// actually here.
//
// Nothing else in this codebase records DURATION: every existing timestamp
// (atom.created_at, atom.last_activity_at, reading/writing.updated_at,
// finished_at) is a point in time, so an end-of-session report asking "how
// many minutes did she spend on this" has nothing to answer from. The client
// posts a heartbeat every 60s while — and ONLY while — the reading or
// writing room's tab is visible, and this endpoint adds that (clamped)
// amount onto atom.active_seconds (Task 1).

// heartbeatCeiling bounds ONE heartbeat's contribution. The client posts
// every 60s while the tab is visible, so a well-behaved call is 60. A call
// claiming more means the tab slept, the machine suspended, or the client is
// lying — none of which is focus time. 120 leaves room for a late post
// without letting an abandoned tab accrue an hour.
const heartbeatCeiling int32 = 120

// clampHeartbeatSeconds bounds one heartbeat's contribution to
// [0, heartbeatCeiling]. Negative clamps to 0 — a heartbeat must never
// SUBTRACT from her recorded focus time.
func clampHeartbeatSeconds(n int32) int32 {
	if n < 0 {
		return 0
	}
	if n > heartbeatCeiling {
		return heartbeatCeiling
	}
	return n
}

// recordHeartbeat adds one clamped heartbeat to the atom's running total and
// to today's Beijing-date bucket, in one transaction so the two never
// disagree. `now` is a parameter so the midnight split is testable.
func (a *API) recordHeartbeat(ctx context.Context, atomID uuid.UUID, seconds int32, now time.Time) error {
	secs := clampHeartbeatSeconds(seconds)
	tx, err := a.d.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	qtx := a.d.Queries.WithTx(tx)
	if err := qtx.AddAtomActiveSeconds(ctx, sqlc.AddAtomActiveSecondsParams{ID: atomID, ActiveSeconds: secs}); err != nil {
		return err
	}
	if err := qtx.AddAtomActiveDay(ctx, sqlc.AddAtomActiveDayParams{
		AtomID:  atomID,
		Day:     pgtype.Date{Time: liteweek.Day(now), Valid: true},
		Seconds: secs,
	}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// heartbeatAtom is the shared body behind POST /api/v1/readings/{id}/heartbeat
// and POST /api/v1/writings/{id}/heartbeat.
//
// loadOwnedAtomRow, NOT loadOwnedAtom: a heartbeat on an already-finished
// atom must stay callable and answer 204 as a no-op, not the 403 the
// finished-write gate would otherwise produce. She may reopen a finished
// reading to re-read it, and that reopening is not focus time on the work —
// this mirrors finishReading/finishWritingAtom's own use of the ungated Row
// sibling for exactly this "must survive a finished atom" reason. The
// finished check and the last_activity_at bump below are then done by hand,
// covering only the not-finished path, since loadOwnedAtom's own gate+bump
// would refuse the finished case before we could turn it into a 204.
//
// No HasEntitlement check: a heartbeat consumes no LLM tokens, and gating it
// would silently stop counting time for a student whose entitlement lapses
// mid-session — she would still be doing the work, and the report would
// understate it.
func (a *API) heartbeatAtom(w http.ResponseWriter, r *http.Request, kind string) {
	at, ok := a.loadOwnedAtomRow(w, r, kind)
	if !ok {
		return
	}

	finished, err := a.atomIsFinished(r.Context(), at.ID, kind)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if finished {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	var req struct {
		Seconds int32 `json:"seconds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadJSON(err))
		return
	}

	if err := a.recordHeartbeat(r.Context(), at.ID, req.Seconds, time.Now()); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	// Best-effort, mirroring loadOwnedAtom's own touch: a heartbeat is, by
	// definition, her being here right now, so it bumps last_activity_at the
	// same way any other non-GET write to an open atom does. A failure here
	// must never turn a recorded heartbeat into a failed request.
	if _, terr := a.d.Queries.TouchAtom(r.Context(), at.ID); terr != nil {
		slog.Warn("lite atom heartbeat: touch last_activity_at failed",
			"err", terr, "atom_id", at.ID, "kind", kind, "request_id", httpx.RequestIDFromContext(r.Context()))
	}

	w.WriteHeader(http.StatusNoContent)
}

func (a *API) readingHeartbeat(w http.ResponseWriter, r *http.Request) {
	a.heartbeatAtom(w, r, "reading")
}

func (a *API) writingHeartbeat(w http.ResponseWriter, r *http.Request) {
	a.heartbeatAtom(w, r, "writing")
}
