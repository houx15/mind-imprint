// writings/draftQuote.ts — whether a teacher's quoted point can still be
// found in the CURRENT draft while she is revising.
//
// The finished page checks a quote against the (frozen) submitted version it
// was written about. The room shows her LIVE draft instead — she is revising
// precisely because that text may already have changed — so a quote that
// matched the version she was graded on can legitimately no longer match
// here. This reuses the exact same normalising match the finished page
// highlights with (`rangeForQuote`, shared/gradingText.ts, which mirrors
// apps/api/internal/quotematch) rather than a second copy of that rule, so
// "clickable in the room" and "would highlight" never disagree.

import { rangeForQuote } from "../shared/gradingText";

/**
 * Whether `quote` can still be found in `draft` — whitespace/punctuation
 * stripped, case folded, substring match (same rule the server used to
 * accept the quote in the first place). `false` for a missing/blank quote,
 * or one she has since rewritten away.
 */
export function quoteFoundInDraft(draft: string, quote: string | null): boolean {
  return rangeForQuote(draft, quote, 0) !== null;
}
