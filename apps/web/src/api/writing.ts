import { z } from "zod";
import { API_BASE, apiFetch } from "./client";
import { parseSSE } from "./sse";
import { mapStudioFrame, type StudioTurnEvent } from "./studioTurn";

// The commit response (apps/api/internal/api/writing.go's snapshotResp) is
// deliberately narrower than contracts' WritingSnapshot — it has no
// committedAt (that's derived server-side only when the FULL projection is
// re-read, studio.projectWriting) — so this is its own schema, not a reuse.
const SnapshotResp = z.object({
  id: z.string(),
  seq: z.number().int(),
  word_count: z.number().int(),
  in_band: z.boolean(),
});
export type CommitSnapshotResult = { id: string; seq: number; wordCount: number; inBand: boolean };

// Silent-edit buffer autosave (Task 3's PUT /buffer) — student text only, no
// entitlement gate, no model call. Fire on every debounced keystroke.
export async function putBuffer(projectId: string, content: string): Promise<void> {
  await apiFetch<void>(`/api/v1/projects/${projectId}/buffer`, {
    method: "PUT",
    body: JSON.stringify({ content }),
  });
}

// Mints an immutable draft snapshot from the posted content (Task 3's POST
// /snapshots) — fails loud on schema drift, same posture as materials.ts.
export async function commitSnapshot(projectId: string, content: string): Promise<CommitSnapshotResult> {
  const raw = await apiFetch<unknown>(`/api/v1/projects/${projectId}/snapshots`, {
    method: "POST",
    body: JSON.stringify({ content }),
  });
  const parsed = SnapshotResp.parse(raw);
  return { id: parsed.id, seq: parsed.seq, wordCount: parsed.word_count, inBand: parsed.in_band };
}

// Runs the student-triggered whole-draft 整稿体检 (Task 6's POST
// .../snapshots/{sid}/review, one snapshot/one review — idempotent replay
// server-side). Same SSE fetch/parse plumbing as studioTurn/submitProjectCard
// (mapStudioFrame is the single place the event vocabulary is decoded) — no
// request body, the snapshot is entirely path-addressed. The work-order's
// rendering shape (interventionId + disposition) comes from the refetched
// StudioProjection, not from these raw stream items — callers consume this
// generator to know when the review is done, not to read its payload.
export async function* orderReview(projectId: string, snapshotId: string): AsyncGenerator<StudioTurnEvent> {
  const res = await fetch(`${API_BASE}/api/v1/projects/${projectId}/snapshots/${snapshotId}/review`, {
    method: "POST",
    credentials: "include",
    headers: { Accept: "text/event-stream" },
  });
  if (!res.ok || !res.body) {
    let code = "internal_error", message = `HTTP ${res.status}`;
    try { const b = await res.json(); if (b?.error) { code = b.error.code ?? code; message = b.error.message ?? message; } } catch { /* non-JSON */ }
    yield { type: "error", code, message };
    return;
  }
  for await (const frame of parseSSE(res.body)) {
    const event = mapStudioFrame(frame);
    if (event) yield event;
  }
}

// Records (or clears) a student_written gate item — here always
// citations_matched on draft_polish (Task 4's POST .../gate/{contractId}/attest,
// 204 on success). Restricted server-side to the contract's own
// student_written item names.
export async function attestGate(projectId: string, contractId: string, item: string, confirmed: boolean): Promise<void> {
  await apiFetch<void>(`/api/v1/projects/${projectId}/gate/${contractId}/attest`, {
    method: "POST",
    body: JSON.stringify({ item, confirmed }),
  });
}
