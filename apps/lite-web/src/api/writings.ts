import { apiFetch } from "./client";

// api/writings.ts — the lite shell's 写作 CRUD client. Shapes read straight
// off the Go handlers (apps/api/internal/api/writings.go), not guessed:
// `writingDTO` (id/title/lang/stage/targetWords/status/createdAt/updatedAt/
// finishedAt) and createWriting's `{"id": "..."}` 201 body. Mirrors
// api/readings.ts's shape; the room-specific calls (turn/outline/snippets/
// compose/cards/summon) live in api/writingRoom.ts instead, matching the
// reading split (readings.ts vs readingRoom.ts).

export interface Writing {
  id: string;
  title: string;
  lang: string;
  /** One of 'outline' | 'snippets' | 'draft' | 'finished' — the three-stage
   *  MAP position, 结构 / 段落 / 成稿 (铁律②: a map, never a gate). The old
   *  'ideate' step was retired on 2026-08-27: migration 0100 moved every row
   *  onto 'outline', and the server now 400s on it. */
  stage: string;
  targetWords: number | null;
  /** Which skeleton she picked out of the fixed structure library; "" = none
   *  yet, which is what the 结构 page's empty state keys on. */
  structureKey: string;
  /** When the entry 设定 dialog was completed. null = never, and that is
   *  exactly what opens it — see WritingSetupModal. */
  setupAt: string | null;
  /** 'active' | 'finished' — independent of `stage` (a finished piece may
   *  still show any stage; see writing_stage.go's file comment). */
  status: string;
  createdAt: string;
  updatedAt: string;
  finishedAt: string | null;
}

/** A writing is finished when the server says so — same both-fields
 *  tolerance as readings.ts's `isFinished`. */
export function isWritingFinished(w: Writing): boolean {
  return w.status === "finished" || Boolean(w.finishedAt);
}

/** GET /api/v1/writings — 我的写作, newest-ordered by the server. */
export async function listWritings(): Promise<Writing[]> {
  const raw = await apiFetch<{ writings: Writing[] }>("/api/v1/writings");
  return raw.writings;
}

/**
 * POST /api/v1/writings — "type one sentence into a box and you're started."
 * `idea` must be non-empty (the server 400s on blank: unlike a reading, a
 * writing has nothing to talk about without one).
 *
 * `lang` here is PROVISIONAL. It used to be final, and the landing box
 * hardcoded "zh" for anything freely typed — so a student who typed her own
 * English essay idea silently lost every English-only affordance in the room
 * and had no way to discover why. The entry 设定 dialog now asks her outright
 * and overwrites this via `setWritingSetup` before she does anything, so a
 * wrong guess at creation time costs nothing.
 */
export async function createWriting(input: { idea: string; lang?: "zh" | "en" }): Promise<{ id: string }> {
  return apiFetch<{ id: string }>("/api/v1/writings", {
    method: "POST",
    body: JSON.stringify({ idea: input.idea, lang: input.lang ?? "zh" }),
  });
}

/** GET /api/v1/writings/{id} */
export async function getWriting(id: string): Promise<Writing> {
  return apiFetch<Writing>(`/api/v1/writings/${encodeURIComponent(id)}`);
}

/** PATCH /api/v1/writings/{id} — renames the writing. Title must be
 *  non-empty (the server 400s on blank). */
export async function renameWriting(id: string, title: string): Promise<Writing> {
  return apiFetch<Writing>(`/api/v1/writings/${encodeURIComponent(id)}`, {
    method: "PATCH",
    body: JSON.stringify({ title }),
  });
}
