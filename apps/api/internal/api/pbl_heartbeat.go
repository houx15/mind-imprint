package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	"mindimprint/api/internal/httpx"
)

// pblFinishedStatuses are the project states that count as done (same set
// that queues interest harvesting). A heartbeat on a done project is a 204
// no-op: reopening a finished project is not time spent on the work.
var pblFinishedStatuses = map[string]bool{"review": true, "keeping": true, "archived": true}

// postPblHeartbeat handles POST /api/v1/pbl/projects/{id}/heartbeat — the
// project room's half of the minute count readings and writings already have.
func (a *API) postPblHeartbeat(w http.ResponseWriter, r *http.Request) {
	atomID, ok := a.loadOwnedPblProject(w, r)
	if !ok {
		return
	}
	// loadOwnedPblProject already parsed {id} successfully, so this cannot fail.
	projectID, _ := uuid.Parse(r.PathValue("id"))
	proj, err := a.d.Queries.GetPblProject(r.Context(), projectID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if pblFinishedStatuses[proj.Status] {
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
	if err := a.recordHeartbeat(r.Context(), atomID, req.Seconds, time.Now()); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if _, terr := a.d.Queries.TouchAtom(r.Context(), atomID); terr != nil {
		slog.Warn("pbl heartbeat: touch last_activity_at failed",
			"err", terr, "atom_id", atomID, "request_id", httpx.RequestIDFromContext(r.Context()))
	}
	w.WriteHeader(http.StatusNoContent)
}
