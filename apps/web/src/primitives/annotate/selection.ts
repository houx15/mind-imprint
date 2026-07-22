/**
 * Text selection → span. Lets a student select a sentence in the article
 * herself (L2/L3 of the guidance ladder), instead of the AI always circling
 * it for her (spec §7.1, §8).
 *
 * `Anchor.start`/`Anchor.end` are RUNE (Unicode code point) indices, matching
 * `segmentBlock` (./segment.ts) and the Go side (`utf8.RuneCountInString`).
 * `Range.startOffset`/`endOffset` are UTF-16 code-unit offsets WITHIN a text
 * node, so they must be converted per node — never summed raw — before they
 * are accumulated into block-level offsets.
 */

export type CreatedSpan = { blockId: string; start: number; end: number; text: string };

function runeLen(s: string): number {
  return Array.from(s).length;
}

function findBlockId(node: Node | null): { blockId: string; blockEl: Element } | null {
  let el: Element | null = node instanceof Element ? node : node?.parentElement ?? null;
  while (el) {
    const blockId = el.getAttribute("data-block-id");
    if (blockId) return { blockId, blockEl: el };
    el = el.parentElement;
  }
  return null;
}

/**
 * Pure over a `Range` — no dependency on `window.getSelection()` — so it is
 * directly testable with jsdom-constructed ranges. `selectionToSpan` below is
 * the thin wrapper that reads the live selection.
 */
export function rangeToSpan(range: Range): CreatedSpan | null {
  if (range.collapsed) return null;

  const startBlock = findBlockId(range.startContainer);
  const endBlock = findBlockId(range.endContainer);
  if (!startBlock || !endBlock || startBlock.blockId !== endBlock.blockId) return null;

  // Walk the block's descendant text nodes in document order, accumulating
  // rune length, so offsets accumulate across sibling runs (e.g. a selection
  // starting inside a <mark> and continuing into a plain <span>) rather than
  // being computed within a single node.
  const walker = document.createTreeWalker(startBlock.blockEl, NodeFilter.SHOW_TEXT);
  let acc = 0;
  let start: number | null = null;
  let end: number | null = null;
  let fullText = "";
  let node: Node | null;
  while ((node = walker.nextNode())) {
    const data = (node as Text).data;
    if (node === range.startContainer) {
      start = acc + runeLen(data.slice(0, range.startOffset));
    }
    if (node === range.endContainer) {
      end = acc + runeLen(data.slice(0, range.endOffset));
    }
    fullText += data;
    acc += runeLen(data);
  }

  if (start == null || end == null) return null;
  if (end <= start) return null;

  const text = Array.from(fullText).slice(start, end).join("");
  return { blockId: startBlock.blockId, start, end, text };
}

/** Thin wrapper: reads `window.getSelection()`, delegates to `rangeToSpan`. */
export function selectionToSpan(): CreatedSpan | null {
  const sel = window.getSelection();
  if (!sel || sel.rangeCount === 0) return null;
  return rangeToSpan(sel.getRangeAt(0));
}
