import { z } from "zod";
import { apiFetch } from "./client";

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
