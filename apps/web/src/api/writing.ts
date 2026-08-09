import { z } from "zod";
import { API_BASE, apiFetch, ApiError } from "./client";
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

// WritingDocKind (Phase B) — which document a writing call targets. The proposal
// (写研究提案) and the essay (写正文) are distinct documents; the active one is
// derived from the studio status (see workspace/activeDoc.ts). Defaults to essay.
export type WritingDocKind = "proposal" | "essay";

// Silent-edit buffer autosave (Task 3's PUT /buffer) — student text only, no
// entitlement gate, no model call. Fire on every debounced keystroke. `doc`
// selects the document (Phase B).
export async function putBuffer(projectId: string, content: string, doc: WritingDocKind = "essay"): Promise<void> {
  await apiFetch<void>(`/api/v1/projects/${projectId}/buffer?doc=${doc}`, {
    method: "PUT",
    body: JSON.stringify({ content }),
  });
}

// Mints an immutable draft snapshot from the posted content (Task 3's POST
// /snapshots) — fails loud on schema drift, same posture as materials.ts. `doc`
// selects the document (Phase B).
export async function commitSnapshot(projectId: string, content: string, doc: WritingDocKind = "essay"): Promise<CommitSnapshotResult> {
  const raw = await apiFetch<unknown>(`/api/v1/projects/${projectId}/snapshots?doc=${doc}`, {
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
export type ReviewVoice = "board" | "sceptic" | "layperson" | "executioner";

// One row of the whole-draft 整稿体检 advice (agent.ReviewItem, snake_case on the
// wire). band/points name WHICH descriptor cell the paragraph evidences — never a
// grade (RL-3). fix is a suggested DIRECTION, not a rewrite.
export const ReviewItem = z.object({
  criterion_code: z.string(),
  criterion_name: z.string(),
  band: z.string(),
  evidence: z.string(),
  missing: z.string(),
  fix: z.string(),
  points: z.number().int().optional(),
});
export type ReviewItem = z.infer<typeof ReviewItem>;

export type DraftReviewResult = { items: ReviewItem[]; wordCount: number; inBand: boolean };

// runDraftReview — WA: the student-triggered "让印记体检整稿". Commits the current
// draft as an immutable snapshot, then runs the whole-draft review and collects
// the advice straight off the stream's final `review` frame (no projection
// refetch needed — the frame carries the persisted ReviewItem[]). Throws on the
// stream's error frame. Never touches the draft — the advice is the student's to
// act on (AI 克制: checks thinking/structure, doesn't rewrite).
export async function runDraftReview(projectId: string, content: string, voice: ReviewVoice = "board", doc: WritingDocKind = "essay"): Promise<DraftReviewResult> {
  const snap = await commitSnapshot(projectId, content, doc);
  let items: ReviewItem[] = [];
  for await (const ev of orderReview(projectId, snap.id, voice)) {
    if (ev.type === "error") throw new ApiError(ev.code, ev.message, 0);
    if (ev.type === "review") {
      const parsed = z.array(ReviewItem).safeParse(ev.items);
      if (parsed.success) items = parsed.data;
    }
  }
  return { items, wordCount: snap.wordCount, inBand: snap.inBand };
}

export async function* orderReview(projectId: string, snapshotId: string, voice: ReviewVoice = "board"): AsyncGenerator<StudioTurnEvent> {
  const q = voice === "board" ? "" : `?voice=${voice}`;
  const res = await fetch(`${API_BASE}/api/v1/projects/${projectId}/snapshots/${snapshotId}/review${q}`, {
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

// Runs a station's spot-check (S3 信源体检 / S4 论证体检 — N3f Task 7). Same
// SSE endpoint shape as orderReview above (fingerprint-keyed idempotent
// replay instead of snapshot-keyed), but the caller here only needs to know
// whether it succeeded: unlike orderReview, whose stream is the only place a
// caller could see the whole-draft work order it just produced, a
// spot-check's items are read straight off the refetched
// StudioProjection.spotChecks (SpotCheckPanel never reads this generator's
// own items). So this resolves once the stream reports "done" and REJECTS on
// an "error" frame — same error-envelope handling as orderReview and the
// same non-OK-response handling, just surfaced as a rejected Promise (mirrors
// apiFetch's throw-on-failure posture) instead of a yielded event, matching
// the plain `Promise<void>` shape SpotCheckPanel's caller wants.
export async function orderSpotCheck(projectId: string, contractId: string): Promise<void> {
  const res = await fetch(`${API_BASE}/api/v1/projects/${projectId}/contracts/${contractId}/spot-check`, {
    method: "POST",
    credentials: "include",
    headers: { Accept: "text/event-stream" },
  });
  if (!res.ok || !res.body) {
    let code = "internal_error", message = `HTTP ${res.status}`;
    try { const b = await res.json(); if (b?.error) { code = b.error.code ?? code; message = b.error.message ?? message; } } catch { /* non-JSON */ }
    throw new ApiError(code, message, res.status);
  }
  for await (const frame of parseSSE(res.body)) {
    const event = mapStudioFrame(frame);
    // M4 fix: this is NOT `res.status` (which is 200 here — the HTTP
    // request succeeded; it's the STREAM's own payload that reported
    // failure after the fact). Reusing `res.status` would silently lie to
    // any future caller that branches on `err.status` (e.g. `=== 400`),
    // since there is no real HTTP status backing this failure. `0` is the
    // sentinel for "no real HTTP status — the failure was reported in-band
    // over an already-200 stream", distinct from every real status code.
    if (event?.type === "error") throw new ApiError(event.code, event.message, 0);
  }
}

// Signs the S6 AI 使用申报单 (Task 9's POST .../declaration/sign) — a plain
// request, unlike orderReview/orderSpotCheck above: no request body, no
// stream, just the server recomputing + persisting the four counters and
// flipping declaration_signed in one transaction. Mirrors putBuffer's shape
// above (plain apiFetch POST, void response), not the SSE pair.
export async function signDeclaration(projectId: string): Promise<void> {
  await apiFetch<void>(`/api/v1/projects/${projectId}/declaration/sign`, {
    method: "POST",
  });
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
