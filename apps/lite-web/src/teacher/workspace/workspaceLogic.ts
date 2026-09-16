// teacher/workspace/workspaceLogic.ts — pure logic for the teacher workspace
// turn loop: applying a patch onto a draft that may have moved under the
// teacher's hand while the turn was in flight, and the two bounds that must
// match the Go side (apps/api/internal/liteworkspace/workspace.go).
//
// No React, no network. `AssignmentDraft` is the concrete type this runs
// against; the functions stay generic so they carry no knowledge of its
// field names.

/** Must equal liteworkspace.TurnsWindow. A drift means the client sends more
 *  turns than the server bounds the prompt to. */
export const TURNS_WINDOW = 8;

/** Must equal liteworkspace.MaxChoices. */
export const MAX_CHOICES = 4;

export interface Turn {
  role: "teacher" | "ai";
  text: string;
}

export interface Choice {
  id: string;
  label: string;
}

/** A turn takes several seconds; the teacher can edit the card while it is in
 * flight. `snapshot` is the draft as it was when the turn started, `current`
 * is the draft now. For each key the patch names: if the teacher changed it
 * since the snapshot, that edit wins — the patched value is dropped and the
 * key recorded in `kept` — otherwise the patch applies. An empty patch is
 * the identity. */
export function applyPatch<T extends object>(
  current: T,
  snapshot: T,
  patch: Partial<T>,
): { next: T; kept: (keyof T)[] } {
  const next = { ...current };
  const kept: (keyof T)[] = [];
  for (const key of Object.keys(patch) as (keyof T)[]) {
    if (current[key] !== snapshot[key]) {
      kept.push(key);
      continue;
    }
    next[key] = patch[key] as T[keyof T];
  }
  return { next, kept };
}

/** Keeps the most recent TURNS_WINDOW turns. */
export function trimTurns(turns: Turn[]): Turn[] {
  if (turns.length <= TURNS_WINDOW) return turns;
  return turns.slice(turns.length - TURNS_WINDOW);
}

/** Drops blank labels first, then caps at MAX_CHOICES — a blank must never
 * consume a slot that a real choice could have filled. */
export function clampChoices(choices: Choice[]): Choice[] {
  const out: Choice[] = [];
  for (const c of choices) {
    if (c.label.trim() === "") continue;
    out.push(c);
    if (out.length === MAX_CHOICES) break;
  }
  return out;
}
