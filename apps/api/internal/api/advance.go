package api

// advance.go — Task 4: the first live caller of agent.AdvanceAll (Task 3).
// Gate state is DERIVED and fully recomputable from the graph (gate.go's
// CheckGate reads only the graph + recorded gate_state, never any other
// input), so re-deriving it on every gate-affecting write is always safe and
// never loses information even if a call here is skipped or fails.

import (
	"context"
	"log/slog"

	"github.com/google/uuid"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/skills"
)

// advanceGates runs agent.AdvanceAll for the project's skill, confirming
// every contract that is now genuinely finished. Gate state is DERIVED and
// fully recomputable from the graph, so a failure here must never fail the
// student's write — the next gate-affecting write recomputes it from
// scratch. This is why it returns nothing and logs instead.
func (a *API) advanceGates(ctx context.Context, projectID uuid.UUID) {
	sk, ok := skills.ByID("writing-project")
	if !ok {
		slog.Warn("advance gates: skill missing", "project_id", projectID.String())
		return
	}
	// Provider/Resolved/Sim are deliberately omitted (left zero-value):
	// AdvanceAll (planner.go) only ever touches deps.Store — it never calls a
	// model, so there is no key to resolve and no provider to construct.
	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
	deps := agent.AgentDeps{Store: store, Skill: &sk}
	if _, err := agent.AdvanceAll(ctx, deps, projectID, sk); err != nil {
		slog.Warn("advance gates failed", "err", err, "project_id", projectID.String())
	}
}

// attestReconLogged records evaluate_perspectives' student_written
// "recon_logged" item once the project's 检索日志 (source log, Slice 6b) has
// at least one entry. Opening and logging a source IS the recon — there is
// no separate control for it (spec §6.2) — and by the time logSourceOpen
// calls this, the source_log_entry it just looked up already proves that
// entry exists, so the condition holds by construction.
//
// Follows submitReflection's gate-merge block verbatim (ListGateStates ->
// take the recorded evaluate_perspectives state -> ensure Items is non-nil
// -> set recon_logged -> UpsertGateState), merging into any existing
// recorded state so a sources_per_perspective attestation (Task 11) is never
// clobbered. Unlike submitReflection, this is a side effect of opening a
// source, not the point of the request, so it logs on error instead of
// failing the request.
func (a *API) attestReconLogged(ctx context.Context, projectID uuid.UUID) {
	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
	recorded, err := store.ListGateStates(ctx, projectID)
	if err != nil {
		slog.Warn("attest recon_logged: list gate states failed", "err", err, "project_id", projectID.String())
		return
	}
	rec := recorded["evaluate_perspectives"]
	if rec.Items == nil {
		rec.Items = map[string]string{}
	}
	rec.Items["recon_logged"] = "solid"
	if err := store.UpsertGateState(ctx, projectID, "evaluate_perspectives", rec); err != nil {
		slog.Warn("attest recon_logged: upsert gate state failed", "err", err, "project_id", projectID.String())
	}
}
