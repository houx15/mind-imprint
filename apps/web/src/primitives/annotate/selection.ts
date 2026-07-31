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
 * A `Range` boundary's `container`/`offset` are only text-node offsets when
 * `container` IS a text node. When the browser collapses a selection onto an
 * ELEMENT instead (real browsers do this for a triple-click "select whole
 * paragraph" — e.g. `container` = the `<p data-block-id>` itself, `offset` =
 * a child-node index), the offset indexes `childNodes`, not characters.
 * Resolves either shape down to a concrete `(Text, charOffset)` pair:
 *   - `offset < childNodes.length` → descend into that child, land on its
 *     FIRST text node at offset 0 (the boundary is "just before" it).
 *   - `offset === childNodes.length` ("after the last child") → land on the
 *     LAST text node inside the last child, at its end.
 * Returns null (a no-op, never a guess) when no text node can be found —
 * e.g. an empty element — so this never fabricates a boundary.
 */
// TreeWalker.nextNode() never yields its OWN root (only descendants) — so a
// child that is ITSELF a leaf Text node (no wrapping <span>, e.g. a block
// with exactly one un-highlighted run appended directly) must be special-
// cased rather than handed to a TreeWalker, which would find nothing to
// descend into and wrongly report "no text here".
function firstTextNodeWithin(node: Node): Text | null {
  if (node.nodeType === Node.TEXT_NODE) return node as Text;
  return document.createTreeWalker(node, NodeFilter.SHOW_TEXT).nextNode() as Text | null;
}

function lastTextNodeWithin(node: Node): Text | null {
  if (node.nodeType === Node.TEXT_NODE) return node as Text;
  const walker = document.createTreeWalker(node, NodeFilter.SHOW_TEXT);
  let last: Text | null = null;
  let n: Text | null;
  while ((n = walker.nextNode() as Text | null)) last = n;
  return last;
}

function resolveTextBoundary(container: Node, offset: number): { node: Text; offset: number } | null {
  if (container.nodeType === Node.TEXT_NODE) return { node: container as Text, offset };

  const children = container.childNodes;
  if (offset < children.length) {
    const first = firstTextNodeWithin(children[offset]!);
    return first ? { node: first, offset: 0 } : null;
  }
  if (children.length === 0) return null;
  const last = lastTextNodeWithin(children[children.length - 1]!);
  return last ? { node: last, offset: last.data.length } : null;
}

/**
 * Pure over a `Range` — no dependency on `window.getSelection()` — so it is
 * directly testable with jsdom-constructed ranges. `selectionToSpan` below is
 * the thin wrapper that reads the live selection.
 */
export function rangeToSpan(range: Range): CreatedSpan | null {
  if (range.collapsed) return null;

  const startBoundary = resolveTextBoundary(range.startContainer, range.startOffset);
  const endBoundary = resolveTextBoundary(range.endContainer, range.endOffset);
  if (!startBoundary || !endBoundary) return null;

  const startBlock = findBlockId(startBoundary.node);
  const endBlock = findBlockId(endBoundary.node);
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
    if (node === startBoundary.node) {
      start = acc + runeLen(data.slice(0, startBoundary.offset));
    }
    if (node === endBoundary.node) {
      end = acc + runeLen(data.slice(0, endBoundary.offset));
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

/**
 * Resolve a viewport point (a click's clientX/clientY) to the RUNE offset within
 * its block — for click-to-select-a-sentence (#9). Uses the standard
 * `caretPositionFromPoint` (or WebKit's `caretRangeFromPoint`), then accumulates
 * rune length across the block's text nodes the same way `rangeToSpan` does.
 * Returns null when the point isn't inside an annotated block.
 */
export function pointToRuneOffset(x: number, y: number): { blockId: string; offset: number } | null {
  let node: Node | null = null;
  let nodeOffset = 0;
  const doc = document as Document & {
    caretPositionFromPoint?: (x: number, y: number) => { offsetNode: Node; offset: number } | null;
    caretRangeFromPoint?: (x: number, y: number) => Range | null;
  };
  if (typeof doc.caretPositionFromPoint === "function") {
    const pos = doc.caretPositionFromPoint(x, y);
    if (pos) {
      node = pos.offsetNode;
      nodeOffset = pos.offset;
    }
  } else if (typeof doc.caretRangeFromPoint === "function") {
    const range = doc.caretRangeFromPoint(x, y);
    if (range) {
      node = range.startContainer;
      nodeOffset = range.startOffset;
    }
  }
  if (!node) return null;

  const boundary = resolveTextBoundary(node, nodeOffset);
  if (!boundary) return null;
  const block = findBlockId(boundary.node);
  if (!block) return null;

  const walker = document.createTreeWalker(block.blockEl, NodeFilter.SHOW_TEXT);
  let acc = 0;
  let n: Node | null;
  while ((n = walker.nextNode())) {
    if (n === boundary.node) {
      return { blockId: block.blockId, offset: acc + runeLen((n as Text).data.slice(0, boundary.offset)) };
    }
    acc += runeLen((n as Text).data);
  }
  return { blockId: block.blockId, offset: acc };
}
