/**
 * Sentence segmentation for click-to-select-a-sentence (#9). Splits a block's
 * text into sentence ranges by RUNE (code-point) offset — matching segment.ts
 * and the Go side (utf8.RuneCountInString) — so a click resolves to the one
 * sentence under the cursor instead of the whole paragraph.
 *
 * Handles mixed zh/en text: always breaks on CJK/full-width and ASCII sentence
 * terminators, but guards ASCII "." against decimals (3.5) and single-letter
 * abbreviations/initials (U.S., e.g.) so those don't over-split.
 */

export type SentenceRange = { start: number; end: number; text: string };

const TERMINATORS = new Set(["。", "！", "？", "!", "?", "…"]);
// Closing punctuation that belongs with the sentence it trails.
const TRAILERS = new Set(["”", "’", '"', "'", "）", ")", "》", "」", "』", "]", "】", "〉"]);

const isDigit = (c?: string) => !!c && c >= "0" && c <= "9";
const isAlpha = (c?: string) => !!c && /[A-Za-z]/.test(c);
const isUpper = (c?: string) => !!c && /[A-Z]/.test(c);
const isCJK = (c?: string) => !!c && /[一-鿿]/.test(c);
const isSpace = (c?: string) => c === " " || c === "\n" || c === "\t";

// Whether an ASCII "." at index i is a real sentence boundary (not a decimal or
// an abbreviation/initial). chars is the rune array.
function asciiDotIsBoundary(chars: string[], i: number): boolean {
  const prev = chars[i - 1];
  const next = chars[i + 1];
  if (isDigit(prev) && isDigit(next)) return false; // decimal: 3.5
  // Single capital letter preceded by a non-letter → initial/abbrev: "U.S.", "e.g."
  if (isAlpha(prev) && !isAlpha(chars[i - 2])) return false;
  // Only break when what follows starts a new sentence: end, space, CJK, or a
  // closing quote/bracket that belongs to this sentence (e.g. `He said "go."`).
  return next === undefined || isSpace(next) || isCJK(next) || TRAILERS.has(next);
}

export function segmentSentences(text: string): SentenceRange[] {
  const chars = Array.from(text);
  const n = chars.length;
  const out: SentenceRange[] = [];
  let start = 0;
  let i = 0;
  while (i < n) {
    const c = chars[i]!;
    const boundary = TERMINATORS.has(c) || (c === "." && asciiDotIsBoundary(chars, i));
    if (!boundary) {
      i++;
      continue;
    }
    // Consume consecutive terminators (?!, 。。) then trailing closers.
    let j = i + 1;
    while (j < n && (TERMINATORS.has(chars[j]!) || chars[j] === ".")) j++;
    while (j < n && TRAILERS.has(chars[j]!)) j++;
    const segEnd = j; // sentence ends here — trailing spaces belong to the gap
    let k = j;
    while (k < n && isSpace(chars[k])) k++;
    const seg = chars.slice(start, segEnd).join("");
    if (seg.trim().length > 0) out.push({ start, end: segEnd, text: seg });
    start = k;
    i = k;
  }
  if (start < n) {
    const seg = chars.slice(start, n).join("");
    if (seg.trim().length > 0) out.push({ start, end: n, text: seg });
  }
  if (out.length === 0) out.push({ start: 0, end: n, text });
  return out;
}

// The sentence range at (or immediately before) a rune offset — used to turn a
// click position into the sentence to select. Returns the last sentence whose
// start is ≤ offset, or the first sentence when offset precedes them all.
export function sentenceAtOffset(sentences: SentenceRange[], offset: number): SentenceRange | null {
  if (sentences.length === 0) return null;
  let sel = sentences[0]!;
  for (const s of sentences) {
    if (s.start <= offset) sel = s;
    else break;
  }
  return sel;
}
