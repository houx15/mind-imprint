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
  /** The article this option means, when it means one (liteworkspace.Choice's
   *  `slug`). Echoed back as `choiceSlug` when she taps it, which is how "use
   *  this article" survives a turn boundary — the server holds nothing between
   *  turns. Absent on an option that is not about an article. */
  slug?: string;
}

/** `current[key] !== snapshot[key]` is reference equality, which is wrong
 * for an array field like `userIds`: a rebuild with no real edit (e.g. a
 * fresh `[...roster]`) looks changed, and an in-place mutation looks
 * unchanged (see the "never mutate in place" rule at the call sites — this
 * function cannot see through a mutation, it can only compare values). When
 * both sides are arrays, compare length and elements instead of identity. */
function sameValue(a: unknown, b: unknown): boolean {
  if (Array.isArray(a) && Array.isArray(b)) {
    if (a.length !== b.length) return false;
    for (let i = 0; i < a.length; i++) {
      if (a[i] !== b[i]) return false;
    }
    return true;
  }
  return a === b;
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
    if (!sameValue(current[key], snapshot[key])) {
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

/** Identifies which session and class a turn was sent under. `classId`
 * alone is not enough to tell a turn's response is still wanted: a teacher
 * can switch class A → B → A while a turn for A is in flight, and the
 * response would land with the classId matching again even though it
 * belongs to an abandoned conversation. `gen` is a counter bumped on every
 * class change (or any other reset that should invalidate in-flight turns)
 * — comparing it alongside `classId` catches the "changed and changed back"
 * case that classId alone cannot. */
export interface TurnSession {
  gen: number;
  classId: string;
}

/** Whether a turn captured as `sent` is still the live session `now` — i.e.
 * whether its response is still wanted. Both fields must match: `gen` alone
 * would be enough on its own (a class change always bumps it), but naming
 * `classId` too keeps the check legible at the call site and independent of
 * anything else that might one day bump `gen` without also changing class. */
export function isCurrentTurn(sent: TurnSession, now: TurnSession): boolean {
  return sent.gen === now.gen && sent.classId === now.classId;
}
