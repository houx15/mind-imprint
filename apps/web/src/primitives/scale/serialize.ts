import type { Anchor, ScaleState, ScaleItem } from "@mind-imprint/contracts";

// Stop config arrives from the card spec's `params.buckets` (e.g.
// certainty-spectrum.json's 个人猜测 → 有据推断 → 强证据 → 科学共识 → 逻辑必然,
// an ORDERED axis rather than sort's unordered classification buckets).
// Field names mirror the JSON exactly: id / label / hint. The primitive
// never hardcodes the confidence vocabulary — it only knows the shape of a
// stop, and is deliberately independent of `primitives/sort`'s Bucket type
// even though the shape is identical (see graph vs. compare precedent).
export type Bucket = { id: string; label: string; hint: string };

let seq = 0;
const nid = () => `sc_${++seq}`;

// ScaleState -> Anchor[] — the wire carrier the studio card locks with. One
// anchor per item: quote = item.text, dimension = item.stop, answer =
// item.reason, author = "student" (RL-4 — this primitive has no AI-authored
// path at all). Items with a blank `text` are dropped — an empty row the
// student never filled is not data. Then, only when `state.rewrite` is
// non-blank, ONE extra anchor carries it on its own `dimension: "rewrite"` so
// it rides the card's `field_written_by{field:"rewrite"}` completion
// predicate (apps/api/internal/agent/card_completion.go fieldWrittenBy) —
// that anchor is never an item and must never be read back into `items`.
export function scaleStateToAnchors(state: ScaleState): Anchor[] {
  const out: Anchor[] = [];
  for (const item of state.items) {
    if (item.text.trim() === "") continue;
    out.push({
      id: nid(),
      material_id: "",
      block_id: "",
      start: 0,
      end: 0,
      quote: item.text,
      dimension: item.stop,
      author: "student",
      question: "",
      answer: item.reason,
    });
  }
  if (state.rewrite.trim() !== "") {
    out.push({
      id: nid(),
      material_id: "",
      block_id: "",
      start: 0,
      end: 0,
      quote: "",
      dimension: "rewrite",
      author: "student",
      question: "",
      answer: state.rewrite,
    });
  }
  return out;
}

// Anchor[] -> ScaleState — the inverse, used to rehydrate the primitive's
// working state from persisted anchors. An anchor whose `dimension` is a
// declared stop id becomes an item; the anchor with `dimension === "rewrite"`
// populates `state.rewrite` and is never treated as an item (mirroring how
// scaleStateToAnchors keeps it off the item list on the way out). Anchors
// matching neither are dropped (defensive against config drift). Order is
// the anchor order.
export function anchorsToScaleState(anchors: Anchor[], stops: Bucket[]): ScaleState {
  const stopIds = new Set(stops.map((s) => s.id));
  const items: ScaleItem[] = [];
  let rewrite = "";
  for (const a of anchors) {
    if (a.dimension === "rewrite") {
      rewrite = a.answer;
    } else if (stopIds.has(a.dimension)) {
      items.push({ id: nid(), text: a.quote, stop: a.dimension, reason: a.answer, author: "student" });
    }
  }
  return { items, rewrite };
}
