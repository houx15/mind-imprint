import type { AnnotateState, Author } from "@mind-imprint/contracts";

export type Run = { text: string; spanId: string | null; author: Author | null };

/**
 * Split a block's text into plain/marked runs given the spans that touch it.
 * A span with no `range` marks the whole block. Slice 1 assumes non-overlapping
 * spans within a block (first-wins by sort order on overlap).
 */
export function segmentBlock(blockId: string, text: string, spans: AnnotateState["spans"]): Run[] {
  const touching = spans
    .filter((s) => s.block_ref === blockId)
    .map((s) => ({
      id: s.id,
      author: s.author,
      start: s.range ? s.range.start : 0,
      end: s.range ? s.range.end : text.length,
    }))
    .sort((a, b) => a.start - b.start);

  if (touching.length === 0) {
    return [{ text, spanId: null, author: null }];
  }

  const runs: Run[] = [];
  let cursor = 0;
  for (const span of touching) {
    if (span.start > cursor) {
      runs.push({ text: text.slice(cursor, span.start), spanId: null, author: null });
    }
    if (span.end > cursor) {
      const from = Math.max(span.start, cursor);
      runs.push({ text: text.slice(from, span.end), spanId: span.id, author: span.author });
      cursor = span.end;
    }
  }
  if (cursor < text.length) {
    runs.push({ text: text.slice(cursor), spanId: null, author: null });
  }
  return runs;
}
