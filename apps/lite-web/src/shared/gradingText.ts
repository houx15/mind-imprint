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
    const needle = quoteNeedle(q);
    if (needle === null) return;
    let from = 0;
    for (;;) {
      const m = matchFrom(text, normText, map, needle, from);
      if (m === null) return;
      if (!taken.some((r) => m.start < r.end && r.start < m.end)) {
        taken.push({ start: m.start, end: m.end, index });
        return;
      }
      from = m.next;
    }
  });
  return taken.sort((a, b) => a.start - b.start);
}

/** A quote normalized for matching, or `null` when there is nothing to match
 *  (missing, blank, or all punctuation). */
function quoteNeedle(q: string | null): string | null {
  const trimmed = q?.trim();
  if (!trimmed) return null;
  const needle = normalizeForMatch(trimmed);
  return needle === "" ? null : needle;
}

/** The first match of `needle` in `normText` at or after `from`, as a range
 *  of the original `text` (end extended over trailing punctuation, as
 *  described on `quoteRanges`), plus where to search next. */
function matchFrom(
  text: string,
  normText: string,
  map: readonly number[],
  needle: string,
  from: number,
): { start: number; end: number; next: number } | null {
  const p = normText.indexOf(needle, from);
  if (p < 0) return null;
  const start = map[p]!;
  let end = map[p + needle.length - 1]! + 1;
  while (end < text.length && PUNCT_STRIP.has(text[end]!)) end++;
  return { start, end, next: p + 1 };
}

/**
 * The first occurrence of one quote in `text`, matched the same way as
 * `quoteRanges` but on its own: another point quoting the same or an
 * overlapping sentence does not affect it. `index` is copied into the
 * result. `null` when the quote is blank, all punctuation, or not found.
 * The student's finished page highlights a clicked quote with this.
 */
export function rangeForQuote(text: string, quote: string | null, index: number): QuoteRange | null {
  const needle = quoteNeedle(quote);
  if (needle === null) return null;
  const { norm, map } = normalizedTextMap(text);
  const m = matchFrom(text, norm, map, needle, 0);
  return m === null ? null : { start: m.start, end: m.end, index };
}

/**
 * Indices of `quotes` that are not found in `text` at all (`rangeForQuote`
 * is null). A quote that overlaps another point's quote is found and is not
 * listed. A point with no quote is not listed.
 */
export function notFoundQuotes(text: string, quotes: readonly (string | null)[]): number[] {
  const out: number[] = [];
  quotes.forEach((q, i) => {
    if (q && q.trim() !== "" && rangeForQuote(text, q, i) === null) out.push(i);
  });
  return out;
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

/**
 * The student side's 「未在正文中标出」: which of `quotes` are not found in
 * her text (`notFoundQuotes`). An overlap with another point's quote is not
 * flagged here: she cannot act on it, and `rangeForQuote` still highlights
 * that quote when she clicks it. The teacher's editor keeps the
 * overlap-aware `unmarkedPointQuotes`.
 *
 * `null` when that cannot be determined yet, because the graded version's
 * body has not loaded. A caller whose body fetch FAILED passes `undefined` here too
 * (there is no separate "failed" value: `FinishedWritingPage` never
 * populates its body cache for a failed fetch, so "not loaded" and "failed
 * to load" are the same input at this layer) — both must read as
 * undetermined, never as "nothing is marked." Treating a missing body as
 * "all clear" would falsely flag every quoted point for one render before
 * the real text arrives, or keep flagging them forever after a background
 * preload failure that has nothing to do with whether her quotes are real.
 */
export function determinedUnmarkedQuotes(
  body: string | undefined,
  quotes: readonly (string | null)[],
): ReadonlySet<number> | null {
  if (body === undefined) return null;
  return new Set(notFoundQuotes(body, quotes));
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
