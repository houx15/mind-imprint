import { z } from "zod";
import { OutlineNode } from "./outlineNode";
import { Snippet } from "./snippet";

// Revision-recording (spec 2026-08-11-revision-recording): two mechanisms
// share one event/table design. Mechanism 1 — periodic checkpoints — snapshots
// one writing artifact's current content as canonical JSON (apps/api's
// checkpointContent, revision_checkpoint.go). Mechanism 2 — discrete mutation
// events (leads/edges/dig/sources) — has no "content" to snapshot; see
// REVISION_EVENT_TYPES below. Go validates only the outer envelope (id,
// status, field_values as object, event_trace as array); these schemas are
// the inner-structure source of truth for both mechanisms' payloads.

export const RevisionCheckpointArtifactType = z.enum([
  "draft",
  "outline",
  "snippets",
  "proposal",
  "claim",
]);
export type RevisionCheckpointArtifactType = z.infer<typeof RevisionCheckpointArtifactType>;

export const RevisionCheckpointTrigger = z.enum(["ask_feedback", "finish", "advance"]);
export type RevisionCheckpointTrigger = z.infer<typeof RevisionCheckpointTrigger>;

// draft: never duplicates body text — references the immutable prose snapshot
// by id (recordCheckpoint / checkpointContent, checkpointDraft branch).
export const DraftCheckpointContent = z.object({ snapshotId: z.string().uuid() });
export type DraftCheckpointContent = z.infer<typeof DraftCheckpointContent>;

// proposal: json.Marshal of the flat ProjectProposal row (workspace.sql.go /
// sqlc.ProjectProposal) — all five fields are plain (never-null) strings.
export const ProposalCheckpointContent = z.object({
  objective: z.string(),
  reason: z.string(),
  activities: z.string(),
  resources: z.string(),
  counterpoints: z.string().optional().default(""),
});
export type ProposalCheckpointContent = z.infer<typeof ProposalCheckpointContent>;

// claim: json.Marshal of essaySubQuestions(state) — []agent.SubQuestion,
// {id, text} per proposal_track.go — the essay's claims/sub-questions.
export const ClaimCheckpointContent = z.array(
  z.object({ id: z.string(), text: z.string() })
);
export type ClaimCheckpointContent = z.infer<typeof ClaimCheckpointContent>;

// outline / snippets: json.Marshal of ListOutlineNodes / ListSnippets rows —
// reuse the existing shared row schemas (single source of truth) rather than
// redefining them here.
export const OutlineCheckpointContent = z.array(OutlineNode);
export type OutlineCheckpointContent = z.infer<typeof OutlineCheckpointContent>;

export const SnippetsCheckpointContent = z.array(Snippet);
export type SnippetsCheckpointContent = z.infer<typeof SnippetsCheckpointContent>;

// Union of every checkpoint artifact's content shape. The artifact type
// (which member applies) is carried out-of-band by the revision_checkpoint
// row's own `artifactType` column, not by a discriminant field inside the
// content JSON itself — so this is a plain union, not discriminated.
export const RevisionCheckpointContent = z.union([
  DraftCheckpointContent,
  ProposalCheckpointContent,
  ClaimCheckpointContent,
  OutlineCheckpointContent,
  SnippetsCheckpointContent,
]);
export type RevisionCheckpointContent = z.infer<typeof RevisionCheckpointContent>;

// Mechanism-2 mutation events + the Mechanism-1 revision_checkpoint marker
// event itself, exactly matching the Go's `Type: "..."` / emitMutation call
// sites (revision_checkpoint.go, exploration.go, workspace_library.go,
// evidence_map.go) — verified by grep, not just this list's authorship.
export const REVISION_EVENT_TYPES = [
  "revision_checkpoint",
  "lead_added",
  "lead_removed",
  "edge_added",
  "edge_removed",
  "lead_adopted",
  "source_attached",
  "dig_performed",
  "source_added",
  "source_dropped",
  "source_reclassified",
] as const;
