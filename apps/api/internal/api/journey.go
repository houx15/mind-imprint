package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/skills"
)

// reopenStation un-waives one station of a composed journey, by rail code
// (S0..S6). The journey is imposed at creation, but re-opening is the
// student's escape hatch (铁律 2): a waived station is never a permanent wall.
// Idempotent — re-opening a station that is not waived is a no-op 200.
func (a *API) reopenStation(w http.ResponseWriter, r *http.Request) {
	id, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	code := strings.ToUpper(strings.TrimSpace(r.PathValue("code")))
	if len(code) != 2 || code[0] != 'S' {
		httpx.WriteError(w, r, httpx.ErrBadRequest("bad_code", "环节编号不对", nil))
		return
	}
	idx, err := strconv.Atoi(code[1:])
	if err != nil || idx < 0 {
		httpx.WriteError(w, r, httpx.ErrBadRequest("bad_code", "环节编号不对", nil))
		return
	}
	sk, skOK := skills.ByID("writing-project")
	if !skOK {
		httpx.WriteError(w, r, httpx.ErrBadRequest("skill_missing", "流程未配置", nil))
		return
	}
	order, err := sk.TopoOrder()
	if err != nil || idx >= len(order) {
		httpx.WriteError(w, r, httpx.ErrBadRequest("bad_code", "环节编号不对", nil))
		return
	}
	contract := order[idx]

	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
	waived, err := store.LoadWaived(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if !waived[contract] {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"reopened": false}) // idempotent no-op
		return
	}
	next := make([]string, 0, len(waived))
	for c := range waived {
		if c != contract {
			next = append(next, c)
		}
	}
	if err := store.SetWaived(r.Context(), id, next); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	deps := agent.AgentDeps{Store: store, Skill: &sk}
	if _, err := agent.Replan(r.Context(), deps, id, sk, "reopened"); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	payload, _ := json.Marshal(map[string]any{"contract": contract, "code": code})
	if err := store.AppendEvent(r.Context(), agent.EventRow{
		ProjectID: id, Surface: "studio", Type: "journey_reopened", Payload: payload,
	}); err != nil {
		// Telemetry only — the re-open already succeeded.
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"reopened": true})
}
