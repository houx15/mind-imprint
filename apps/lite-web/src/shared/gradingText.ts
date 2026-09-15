// shared/gradingText.ts — where a grading's quotes sit in her text, for
// highlighting on the teacher's grading view and the student's finished page.
//
// A model or a teacher can restate a real sentence with a different trailing
// 。 or a half-width comma without inventing anything — the same case
// apps/api/internal/quotematch exists for. This module matches quotes the
// same way the server does (`Normalize`: strip whitespace/common punctuation,
// lowercase, then substring-contains) so a quote the server accepted here is
// also found and highlighted, not silently dropped over a punctuation detail.

import { splitSentences } from "../writings/sentences";

/** Same character set as quotematch.Normalize's switch, split into the two
 *  groups this module needs separately: WS is never kept in a highlighted
 *  span's trailing edge, PUNCT is (see quoteRanges). */
const WS_STRIP = new Set([" ", "\t", "\n", "\r", "　"]);
const PUNCT_STRIP = new Set(["。", "，", "、", "；", "：", "！", "？", "…", ".", ",", ";", ":", "!", "?"]);
const isStripped = (ch: string): boolean => WS_STRIP.has(ch) || PUNCT_STRIP.has(ch);

/** A string with whitespace/punctuation dropped and case folded — matches
 *  quotematch.Normalize exactly, character set included. */
function normalizeForMatch(s: string): string {
  let out = "";
  for (let i = 0; i < s.length; i++) {
    const ch = s[i]!;
    if (!isStripped(ch)) out += ch.toLowerCase();
  }
  return out;
}

/** `text` normalized the same way, plus `map[i]` = the original index of the
 *  normalized string's `i`-th character (every kept character is real text,
 *  so this only ever points at content, never at a stripped character).
 *
 *  A character's lowercase form is not always one UTF-16 unit long — `"İ"`
 *  (U+0130) lowercases to two (`"i"` + a combining dot above). Pushing only
 *  one `map` entry per *original* character would leave `map` one entry
 *  short of `norm`'s actual length from that point on, shifting every later
 *  match's offsets by one. So this pushes one entry per unit of the
 *  *lowered* output, all pointing at the same original index. */
function normalizedTextMap(text: string): { norm: string; map: number[] } {
  let norm = "";
  const map: number[] = [];
  for (let i = 0; i < text.length; i++) {
    const ch = text[i]!;
    if (isStripped(ch)) continue;
    const lower = ch.toLowerCase();
    norm += lower;
    for (let k = 0; k < lower.length; k++) map.push(i);
  }
  return { norm, map };
}

export interface QuoteRange {
  start: number;
  end: number;
  /** Position of the quote in the list passed in (the point's index). */
  index: number;
}

/**
 * First non-overlapping occurrence of each quote in `text`, matched the way
 * the server matches a grading's quotes (normalized: whitespace/punctuation
 * dropped, case folded — see quotematch.Normalize). A quote that is blank,
 * normalizes to nothing (all punctuation, e.g. "……"), not found, or only
 * overlaps an already-taken range gets no range — never a wrong one.
 *
 * A match's `end` also absorbs the trailing run of *punctuation* (not
 * whitespace) immediately after the matched content in `text`, so a quote
 * that matched up to a sentence's content but dropped its own trailing 。
 * still highlights the whole sentence. Whitespace is deliberately excluded
 * from that extension: a quote that spans a paragraph break already has the
 * break *inside* its matched span (between two matched characters), but a
 * quote that simply ends at a sentence must not bleed into the blank line
 * that happens to follow it.
 */
export function quoteRanges(text: string, quotes: readonly (string | null)[]): QuoteRange[] {
  const { norm: normText, map } = normalizedTextMap(text);
  const taken: QuoteRange[] = [];
  quotes.forEach((q, index) => {
    const trimmed = q?.trim();
    if (!trimmed) return;
    const needle = normalizeForMatch(trimmed);
    if (needle === "") return;
    let from = 0;
    for (;;) {
      const p = normText.indexOf(needle, from);
      if (p < 0) return;
      const start = map[p]!;
      let end = map[p + needle.length - 1]! + 1;
      while (end < text.length && PUNCT_STRIP.has(text[end]!)) end++;
      if (!taken.some((r) => start < r.end && r.start < end)) {
        taken.push({ start, end, index });
        return;
      }
      from = p + 1;
    }
  });
  return taken.sort((a, b) => a.start - b.start);
}

export interface TextSegment {
  text: string;
  index: number | null;
}

/** The text cut into plain and highlighted segments; joined, they are the text. */
export function highlightSegments(text: string, ranges: readonly QuoteRange[]): TextSegment[] {
  const out: TextSegment[] = [];
  let at = 0;
  for (const r of ranges) {
    if (r.start > at) out.push({ text: text.slice(at, r.start), index: null });
    out.push({ text: text.slice(r.start, r.end), index: r.index });
    at = r.end;
  }
  if (at < text.length || out.length === 0) out.push({ text: text.slice(at), index: null });
  return out;
}

/**
 * Which of `quotes` (the same array passed to `quoteRanges`, so index `i`
 * here lines up with a point's own index) got no highlight range — either
 * because it wasn't found in the text at all, or because it only overlapped
 * an earlier point's already-taken range. `quoteRanges` silently drops both
 * cases (never a wrong range); this is how a point card finds out its own
 * quote is one of them, to show 「未在正文中标出」 instead of nothing.
 * A point with no quote at all (`null`/blank) is not "unmarked" — there is
 * nothing for it to have missed.
 */
export function unmarkedPointQuotes(quotes: readonly (string | null)[], ranges: readonly QuoteRange[]): number[] {
  const marked = new Set(ranges.map((r) => r.index));
  const out: number[] = [];
  quotes.forEach((q, i) => {
    if (q && q.trim() !== "" && !marked.has(i)) out.push(i);
  });
  return out;
}

/** Sentences a teacher can pick as a point's quote. Only exact substrings are
 *  offered, because the server refuses any other quote. */
export function pickableSentences(text: string): string[] {
  return splitSentences(text)
    .map((s) => s.trim())
    .filter((s) => s !== "" && text.includes(s));
}

// Shared with GradingPage (Task 12) and FinishedWritingPage (Task 14) so a
// highlighted quote reads the same in both places — defined once here
// instead of twice, per the ruling against duplicating these two constants.
export const MARK_STYLE = { background: "color-mix(in srgb, var(--mk-warning) 22%, transparent)", color: "inherit" };
export const PIECE_CLS = "whitespace-pre-wrap font-mk-piece text-mk-report-piece text-mk-ink";
