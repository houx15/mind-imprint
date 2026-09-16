// writings/draftQuote.ts — whether a teacher's quoted point can still be
// found in the CURRENT draft while she is revising, and the exact substring
// to hand `ProseSurface` so it actually highlights.
//
// The finished page checks a quote against the (frozen) submitted version it
// was written about. The room shows her LIVE draft instead — she is revising
// precisely because that text may already have changed — so a quote that
// matched the version she was graded on can legitimately no longer match
// here. This reuses the exact same normalising match the finished page
// highlights with (`rangeForQuote`, shared/gradingText.ts, which mirrors
// apps/api/internal/quotematch) rather than a second copy of that rule, so
// "clickable in the room" and "would highlight" never disagree — see the
// doc comment on `draftQuoteMatch` for the bug this replaced.

import { rangeForQuote } from "../shared/gradingText";

/**
 * The literal substring of `draft` a teacher's `quote` matches — using the
 * same normalising rule the server accepted it with (`rangeForQuote`) — or
 * `null` when it cannot be found there at all (she has since changed that
 * text).
 *
 * 🚨 Pass THIS substring, never the raw `quote`, to `ProseSurface`'s
 * `highlight` prop. `ProseSurface` mirrors a plain textarea and matches
 * `highlight` with a literal `text.indexOf` — it does not normalise. A
 * teacher's quote is routinely NOT a byte-for-byte substring of the draft
 * (a restated 。 vs a comma, full-width vs half-width, different case), so
 * the first version of this file fed `ProseSurface` the raw quote: it could
 * pass this module's own "clickable" gate (normalising match) and still
 * highlight NOTHING (literal match fails) — reproduced with
 * draft="中国的碳排放全球第一　，后面接着写别的。",
 * quote="中国的碳排放全球第一。" (comma vs 。). `draftQuoteMatch`'s return
 * value, by construction, IS an exact substring of `draft` at the moment it
 * was computed, so it always literal-matches itself — see the combined test
 * in `draftQuote.test.ts` pinning this against `ProseSurface`'s
 * `splitOnHighlight`, which is the test that would have caught the bug.
 */
export function draftQuoteMatch(draft: string, quote: string | null): string | null {
  const range = rangeForQuote(draft, quote, 0);
  return range ? draft.slice(range.start, range.end) : null;
}

/**
 * Whether `quote` can still be found in `draft` at all (missing/blank quote,
 * or one she has since rewritten away, both `false`).
 */
export function quoteFoundInDraft(draft: string, quote: string | null): boolean {
  return draftQuoteMatch(draft, quote) !== null;
}

/**
 * Whether a teacher grading's quote is clickable in the writing room: found
 * in the current draft AND the room is actually showing that draft for her
 * to jump into (成稿 — `stage === "draft"`). Other stages (段落, 结构) have
 * no single draft surface to scroll a highlight into — `WritingRoomHost`
 * only wires `pendingHighlight` through to `ComposeStage` — so a quote there
 * renders as plain text even when the text technically still exists
 * somewhere in a draft she isn't looking at. This does not switch her to
 * 成稿; it only decides whether the quote is a button right now.
 */
export function quoteClickableInRoom(stage: string, draft: string, quote: string | null): boolean {
  return stage === "draft" && quoteFoundInDraft(draft, quote);
}
