import type { Anchor, SortState, SortItem } from "@mind-imprint/contracts";

// Bucket config arrives from the card spec's `params.buckets` (e.g.
// fact-opinion-value.json). Field names mirror the JSON exactly: id / label
// / hint. The primitive never hardcodes the 事实/观点/价值判断 vocabulary —
// it only knows the shape of a bucket.
export type Bucket = { id: string; label: string; hint: string };

let seq = 0;
const nid = () => `so_${++seq}`;

// SortState -> Anchor[] — the wire carrier the studio card locks with. One
// anchor per item: quote = item.text, dimension = item.bucket, answer =
// item.reason, author = "student" (RL-4 — this primitive has no AI-authored
// path at all). Items with a blank `text` are dropped — an empty row the
// student never filled is not data.
export function sortStateToAnchors(state: SortState): Anchor[] {
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
      dimension: item.bucket,
      author: "student",
      question: "",
      answer: item.reason,
    });
  }
  return out;
}

// Anchor[] -> SortState — the inverse, used to rehydrate the primitive's
// working state from persisted anchors. An anchor whose `dimension` is not a
// declared bucket id is dropped (defensive against config drift, mirroring
// how anchorsToGraphState iterates slots rather than raw anchors). Order is
// the anchor order.
export function anchorsToSortState(anchors: Anchor[], buckets: Bucket[]): SortState {
  const bucketIds = new Set(buckets.map((b) => b.id));
  const items: SortItem[] = [];
  for (const a of anchors) {
    if (!bucketIds.has(a.dimension)) continue;
    items.push({ id: nid(), text: a.quote, bucket: a.dimension, reason: a.answer, author: "student" });
  }
  return { items };
}
