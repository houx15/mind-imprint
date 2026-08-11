package api

// revision_checkpoint.go — Task 2 of the revision-recording plan (spec
// 2026-08-11-revision-recording). recordCheckpoint is the best-effort writer
// that snapshots one writing artifact (proposal / outline / snippets / claim /
// draft) as canonical JSON, hashes it, and inserts a revision_checkpoint row —
// the "checkpoints for writing text" half of the two-mechanism design (the
// other half, mutation events for exploration/sources, is a separate task).
// Best-effort: never blocks or fails the caller's request — on any read/
// insert error it logs and returns (mirrors the log-and-continue pattern
// used elsewhere, e.g. question_card.go's card_completed AppendEvent).

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

const (
	checkpointDraft    = "draft"
	checkpointOutline  = "outline"
	checkpointSnippets = "snippets"
	checkpointProposal = "proposal"
	checkpointClaim    = "claim"

	triggerAskFeedback = "ask_feedback"
	triggerFinish      = "finish"
	triggerAdvance     = "advance"
)

// checkpointContent reads the current state of one artifact as canonical JSON.
// Returns nil, nil when there is nothing to snapshot (skip silently) — an
// unrecognized artifactType, or (for draft) no snapshot committed yet.
func (a *API) checkpointContent(ctx context.Context, projectID uuid.UUID, artifactType string) ([]byte, error) {
	switch artifactType {
	case checkpointProposal:
		p, err := a.d.Queries.GetProjectProposal(ctx, projectID)
		if err != nil {
			return nil, err
		}
		return json.Marshal(map[string]any{
			"objective": p.Objective, "reason": p.Reason,
			"activities": p.Activities, "resources": p.Resources, "counterpoints": p.Counterpoints,
		})
	case checkpointOutline:
		nodes, err := a.d.Queries.ListOutlineNodes(ctx, projectID)
		if err != nil {
			return nil, err
		}
		return json.Marshal(nodes)
	case checkpointSnippets:
		rows, err := a.d.Queries.ListSnippets(ctx, projectID)
		if err != nil {
			return nil, err
		}
		return json.Marshal(rows)
	case checkpointClaim:
		state, err := a.loadEssayState(ctx, projectID)
		if err != nil {
			return nil, err
		}
		subQuestions := a.essaySubQuestions(state)
		if len(subQuestions) == 0 {
			// Nothing to snapshot (e.g. no proposal track yet) -> skip
			// silently, mirroring the draft case above. Marshaling a nil/
			// empty slice would otherwise produce JSON `null`, which the
			// ClaimCheckpointContent Zod contract (z.array(...)) rejects.
			return nil, nil
		}
		return json.Marshal(subQuestions)
	case checkpointDraft:
		// Never duplicate body text: reference the immutable snapshot by id.
		snap, err := a.d.Queries.GetLatestSnapshot(ctx, sqlc.GetLatestSnapshotParams{
			ProjectID: projectID, DocKind: string(agent.DocEssay),
		})
		if err != nil {
			return nil, err // no snapshot yet -> caller logs & skips
		}
		return json.Marshal(map[string]any{"snapshotId": snap.ID.String()})
	default:
		return nil, nil
	}
}

// recordCheckpoint is best-effort: it never panics or propagates an error to
// the caller. On any read/insert failure it logs and returns, so a checkpoint
// miss never blocks the request path (ask-feedback / finish / advance) it's
// attached to.
func (a *API) recordCheckpoint(ctx context.Context, projectID uuid.UUID, artifactType, trigger string, feedbackRef *uuid.UUID) {
	content, err := a.checkpointContent(ctx, projectID, artifactType)
	if err != nil {
		slog.Warn("revision: read artifact failed", "artifact", artifactType, "trigger", trigger, "err", err)
		return
	}
	if content == nil {
		return
	}
	sum := sha256.Sum256(content)
	fr := pgtype.UUID{Valid: false}
	if feedbackRef != nil {
		fr = pgtype.UUID{Bytes: *feedbackRef, Valid: true}
	}
	if _, err := a.d.Queries.InsertRevisionCheckpoint(ctx, sqlc.InsertRevisionCheckpointParams{
		ProjectID: projectID, ArtifactType: artifactType, Trigger: trigger,
		Content: content, ContentHash: hex.EncodeToString(sum[:]), FeedbackRef: fr,
	}); err != nil {
		slog.Warn("revision: insert checkpoint failed", "artifact", artifactType, "trigger", trigger, "err", err)
		return
	}
	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
	if err := store.AppendEvent(ctx, agent.EventRow{
		ProjectID: projectID, Surface: "studio", Type: "revision_checkpoint",
		Payload: mustJSON(map[string]any{"artifactType": artifactType, "trigger": trigger}),
	}); err != nil {
		slog.Warn("revision: append revision_checkpoint event failed", "artifact", artifactType, "err", err)
	}
}

// recordCheckpoints loops recordCheckpoint over multiple artifact types for
// one trigger (e.g. an ask-feedback turn snapshots several artifacts at once).
func (a *API) recordCheckpoints(ctx context.Context, projectID uuid.UUID, trigger string, feedbackRef *uuid.UUID, artifactTypes ...string) {
	for _, at := range artifactTypes {
		a.recordCheckpoint(ctx, projectID, at, trigger, feedbackRef)
	}
}

// -- GET /revision-checkpoints ----------------------------------------------

// listRevisionCheckpoints returns every revision checkpoint recorded for the
// project (both mechanisms funnel through the same revision_checkpoint table
// via recordCheckpoint), ordered by createdAt — read-only, no model call.
func (a *API) listRevisionCheckpoints(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	rows, err := a.d.Queries.ListRevisionCheckpoints(r.Context(), projectID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := make([]map[string]any, 0, len(rows))
	for _, c := range rows {
		var fr any
		if c.FeedbackRef.Valid {
			fr = uuid.UUID(c.FeedbackRef.Bytes).String()
		}
		out = append(out, map[string]any{
			"id": c.ID.String(), "artifactType": c.ArtifactType, "trigger": c.Trigger,
			"contentHash": c.ContentHash, "feedbackRef": fr, "createdAt": c.CreatedAt,
		})
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"checkpoints": out})
}

// -- Task 5: Mechanism-2 mutation events (exploration graph) ---------------
//
// recordCheckpoint (above) is the "checkpoints for writing text" half of the
// two-mechanism revision-recording design. This half covers mutations to the
// exploration graph (leads/edges/dig) — discrete events rather than periodic
// snapshots, since there's no meaningful "content" to hash for "a lead was
// deleted". Both halves share the same event table via AppendEvent.

// currentStage reads the project's studio stage best-effort — "" on any
// error, never blocking the caller. Reuses loadEssayState's own
// GetStudioState-then-unmarshal (essay_statement.go), which itself never
// errors except on a corrupt stored JSON blob (a missing row just falls back
// to agent.DefaultStudioState()).
func (a *API) currentStage(ctx context.Context, projectID uuid.UUID) string {
	state, err := a.loadEssayState(ctx, projectID)
	if err != nil {
		return ""
	}
	return string(state.Stage)
}

// emitMutation appends a Mechanism-2 event for an exploration-graph mutation
// (lead/edge/dig). Every payload is auto-stamped with `stage` (unless the
// caller already set one) — the "at what phase was this node/source added"
// material-provenance signal. Best-effort + additive, mirroring
// recordCheckpoint's own AppendEvent call: an emit failure only logs, it
// must never fail or change the caller's response.
func (a *API) emitMutation(ctx context.Context, projectID uuid.UUID, eventType string, payload map[string]any) {
	if payload == nil {
		payload = map[string]any{}
	}
	if _, ok := payload["stage"]; !ok {
		payload["stage"] = a.currentStage(ctx, projectID)
	}
	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
	if err := store.AppendEvent(ctx, agent.EventRow{
		ProjectID: projectID, Surface: "studio", Type: eventType, Payload: mustJSON(payload),
	}); err != nil {
		slog.Warn("revision: emit mutation event failed", "type", eventType, "err", err)
	}
}
