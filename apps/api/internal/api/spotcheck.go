package api

// spotcheck.go — N3f Task 4. POST /projects/{id}/contracts/{contractId}/
// spot-check: the endpoint that lets a student order S3/S4's station
// spot-check (evaluate_sources' 信源体检 / build_argument's 论证体检). Same
// move as orderReview (writing.go:267-433) one level up: a station has no
// committed snapshot to anchor idempotency on, so agent.SpotCheckFingerprint
// generalizes the snapshot id to a content hash of exactly what the check
// reads (agent.SpotCheckTargets via studio.SpotCheckTargets).

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/skills"
	"mindimprint/api/internal/store/sqlc"
	"mindimprint/api/internal/studio"
)

// orderSpotCheck runs a station's spot-check. One (station, fingerprint)
// pair, one check: if spot_check_item interventions already anchor this
// exact fingerprint, stream them back — NO second model call. `contractId`
// must be one of the two stations that have a spot-check defined
// (agent.SpotCheckSources / agent.SpotCheckArgument); anything else is a 404
// so this route can never mint interventions against an undefined station.
func (a *API) orderSpotCheck(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	u, _ := UserFromContext(r.Context())
	station := r.PathValue("contractId")
	if station != agent.SpotCheckSources && station != agent.SpotCheckArgument {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}

	d, err := studio.Load(r.Context(), a.d.Queries, projectID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	targets := studio.SpotCheckTargets(d, station, a.d.SpecByID)
	if len(targets) == 0 {
		httpx.WriteError(w, r, httpx.ErrBadRequest("nothing_to_check", "还没有可以体检的内容。", nil))
		return
	}
	fingerprint := agent.SpotCheckFingerprint(targets)

	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
	existing := spotCheckItemsFor(r.Context(), a.d.Queries, projectID, station, fingerprint)

	// Entitlement gate BEFORE the stream — only when a model call will happen.
	if len(existing) == 0 {
		entitled, eerr := HasEntitlement(r.Context(), u)
		if eerr != nil {
			httpx.WriteError(w, r, eerr)
			return
		}
		if !entitled {
			httpx.WriteError(w, r, httpx.ErrNotEntitled())
			return
		}
	}

	sse, err := gateway.NewSSEWriter(w)
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrInternal())
		return
	}
	em := &studioEmitter{sse: sse}
	stop, hbDone := startHeartbeat(r.Context(), em)
	defer func() { close(stop); <-hbDone }()

	if len(existing) > 0 { // idempotent replay — no second model call.
		_ = em.Review(mustJSON(existing))
		_ = em.Done()
		return
	}

	sk, _ := skills.ByID("writing-project")
	resolved, rerr := a.d.ChatResolver(r.Context())
	if rerr != nil {
		_ = em.ErrorEnvelope("internal_error", "体检失败，请重试")
		_ = em.Done()
		return
	}
	items, usage, perr := agent.ProposeSpotCheck(r.Context(), a.d.Provider, resolved, station, targets)
	// Record the call cost even if enforcement then rejected the output — a
	// rejected call still cost money.
	if usage.InputTokens > 0 || usage.OutputTokens > 0 {
		if err := store.RecordLLMCall(r.Context(), agent.LLMCallRow{
			ProjectID: projectID, Surface: "studio", Purpose: "spot_check",
			Resolved: resolved, PromptTokens: int32(usage.InputTokens), CompletionTokens: int32(usage.OutputTokens),
		}); err != nil {
			slog.Warn("spot check: record llm call", "err", err)
		}
	}
	if perr != nil {
		// Enforcement rejection or parse failure: persist NOTHING, stream an
		// error envelope + done.
		slog.Warn("spot check: proposal rejected", "err", perr, "station", station)
		_ = em.ErrorEnvelope("spot_check_rejected", "这次体检没通过内部校验，请再试一次")
		_ = em.Done()
		return
	}

	// Persist each item as a spot_check_item intervention anchored to
	// {station, fingerprint}.
	anchor := mustJSON(map[string]string{"station": station, "fingerprint": fingerprint})
	persisted := make([]agent.SpotCheckItem, 0, len(items))
	for _, it := range items {
		if err := store.InsertSpotCheckIntervention(r.Context(), agent.SpotCheckInterventionRow{
			ProjectID: projectID, Anchor: anchor, Body: string(mustJSON(it)),
		}); err != nil {
			slog.Warn("spot check: persist item", "err", err)
			continue
		}
		persisted = append(persisted, it)
	}

	// Mark the station's human gate item solid ONLY if at least one item
	// actually persisted. If every insert failed, the gate must NOT be marked
	// solid: a solid gate with zero visible items makes the gate and the UI
	// disagree, and a phantom "already ordered" record would make every later
	// attempt a silent no-op with nothing to show for it.
	if len(persisted) > 0 {
		itemName := map[string]string{
			agent.SpotCheckSources:  "source_quality_spot_check",
			agent.SpotCheckArgument: "warrant_quality_spot_check",
		}[station]
		// Guard on the item actually being in the contract's Gate.Human, so a
		// skill-config edit can never leave this writing a name no gate reads.
		if c, ok := sk.Contracts[station]; ok {
			for _, hi := range c.Gate.Human {
				if hi == itemName {
					recorded, gerr := store.ListGateStates(r.Context(), projectID)
					if gerr != nil {
						slog.Warn("spot check: list gate states", "err", gerr)
						break
					}
					rec := recorded[station]
					if rec.Items == nil {
						rec.Items = map[string]string{}
					}
					rec.Items[itemName] = "solid"
					if err := store.UpsertGateState(r.Context(), projectID, station, rec); err != nil {
						slog.Warn("spot check: record gate item", "err", err)
					}
					break
				}
			}
		}
		if err := store.AppendEvent(r.Context(), agent.EventRow{
			ProjectID: projectID, Surface: "studio", Type: "spot_check_ordered",
			Payload: mustJSON(map[string]any{"station": station, "items": len(persisted)}),
		}); err != nil {
			slog.Warn("spot check: append event", "err", err)
		}
		a.advanceGates(r.Context(), projectID)
	}
	// Ordering a spot-check is student activity that costs a model call — the
	// roster's 最近活跃 column depends on last_active_at (see the "exact roster
	// lie" comment at projectcards.go:317); best-effort, never fails the
	// stream.
	if err := a.d.Queries.TouchProject(r.Context(), projectID); err != nil {
		slog.Warn("spot check: touch project", "err", err)
	}
	_ = em.Review(mustJSON(persisted))
	_ = em.Done()
}

// spotCheckItemsFor returns the persisted spot-check items anchored to
// (station, fingerprint), reconstructed from their intervention rows (empty
// if none — the not-yet-ordered state, which is what makes a spot-check
// idempotent: seeing none here is exactly the signal to call the model,
// seeing any is the signal to replay them instead).
func spotCheckItemsFor(ctx context.Context, q *sqlc.Queries, projectID uuid.UUID, station, fingerprint string) []agent.SpotCheckItem {
	ivs, err := q.ListInterventionsByProject(ctx, projectID)
	if err != nil {
		return nil
	}
	out := []agent.SpotCheckItem{}
	for _, iv := range ivs {
		if iv.Type != "spot_check_item" {
			continue
		}
		var anchor struct {
			Station     string `json:"station"`
			Fingerprint string `json:"fingerprint"`
		}
		if err := json.Unmarshal(iv.Anchor, &anchor); err != nil {
			continue
		}
		if anchor.Station != station || anchor.Fingerprint != fingerprint {
			continue
		}
		var it agent.SpotCheckItem
		if err := json.Unmarshal([]byte(iv.Body), &it); err != nil {
			continue
		}
		out = append(out, it)
	}
	return out
}
