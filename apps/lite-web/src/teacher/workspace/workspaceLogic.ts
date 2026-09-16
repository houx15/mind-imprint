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

/** Must equal liteworkspace.HistoryTextCapRunes. */
export const HISTORY_TEXT_CAP = 1000;

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
  /** The catalogue fields to show as a card, filled in server-side from
   *  `slug` (liteworkspace.ChoiceArticle) — not model output, so it carries
   *  no unverified prose. Absent on an option that is not about an article. */
  article?: ChoiceArticle;
}

export interface ChoiceArticle {
  slug: string;
  zhTitle: string;
  /** Absent when the cover could not be signed (OSS unset, or the key
   *  missing). A card with no cover renders without one, not with a broken
   *  `<img>`. */
  coverUrl?: string;
  reason: string;
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

/** Caps a HISTORY turn's text to HISTORY_TEXT_CAP runes, ending a cut string
 *  with "…". */
export function truncateHistoryText(text: string): string {
  const runes = [...text];
  if (runes.length <= HISTORY_TEXT_CAP) return text;
  return runes.slice(0, HISTORY_TEXT_CAP).join("") + "…";
}

/** Caps every turn to HISTORY_TEXT_CAP runes. `turns` here is HISTORY ONLY —
 *  the caller (threadLogic.ts's beginTurn) sends the current turn
 *  separately, never inside this array, so there is no "last turn"
 *  exception: every item is capped. Call this on an already-`trimTurns`-
 *  windowed array. */
export function truncateHistory(turns: Turn[]): Turn[] {
  return turns.map((t) => ({ ...t, text: truncateHistoryText(t.text) }));
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

/** Splits one `ask_choice` reply's options into the two blocks the panel
 *  renders: article options as stacked cards, plain options as a wrapping
 *  pill row (the compact row that shipped before Task 4 — the spec only
 *  turned ARTICLE options into cards, not every option into a form field).
 *  Order within each group is preserved from `choices`; `cards` always comes
 *  first because a card is the taller, more deliberate pick. */
export function splitChoices(choices: Choice[]): { cards: Choice[]; pills: Choice[] } {
  const cards: Choice[] = [];
  const pills: Choice[] = [];
  for (const c of choices) {
    (c.article ? cards : pills).push(c);
  }
  return { cards, pills };
}

/** Removes the optimistic teacher turn a failed round trip left on screen.
 *
 * A turn is appended before the request goes out so her sentence appears
 * immediately. When the request fails, 重试 sends it again and appends a second
 * copy — she saw her own sentence twice and the server received it twice, once
 * in `turns` and once as `text`. Rolling back on failure is what makes 重试 an
 * ordinary send.
 *
 * It removes a turn only when the one at `index` is still the last turn, is
 * hers, and still carries the text that was sent. Any other shape means the
 * conversation moved on (a class change clears it; the generation counter at
 * the call site already refuses a reply from an abandoned session) and the turn
 * at that index is no longer the one this failure is about. */
export function rollbackTurn(turns: Turn[], index: number, text: string): Turn[] {
  if (index !== turns.length - 1) return turns;
  const last = turns[index];
  if (!last || last.role !== "teacher" || last.text !== text) return turns;
  return turns.slice(0, index);
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
