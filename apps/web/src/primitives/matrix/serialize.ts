import type { Anchor, MatrixState, MatrixRow } from "@mind-imprint/contracts";

// Column config arrives from the card spec's `params.cols` (e.g.
// perspective-matrix.json's 立场主张/依据/盲区). Field names mirror the JSON
// exactly: id / label / q. The primitive never hardcodes that vocabulary —
// it only knows the shape of a column. Deliberately independent of
// primitives/sort's Bucket and primitives/scale's Bucket even though the
// shape rhymes (see graph vs. compare precedent) — these primitive modules
// do not import from one another.
export type Col = { id: string; label: string; q: string };

let seq = 0;
const nid = () => `mx_${++seq}`;

// MatrixState -> Anchor[] — the wire carrier the studio card locks with. ONE
// ANCHOR PER CELL: quote = row.label (the row's persisted identity — this is
// what the server groups on and what the `perspectives` graph effect names
// the minted node with), dimension = column id, answer = cell text, author =
// "student" (RL-4 — this primitive has no AI-authored path at all). Rows
// with a blank `label` are dropped entirely — an unnamed perspective is not
// yet a row. Blank cells emit no anchor. Cell order follows the row's own
// `cells` key order (Matrix.tsx always initializes a new row's cells in
// `cols` order, so in practice this is cols order), rows in state order.
export function matrixStateToAnchors(state: MatrixState): Anchor[] {
  const out: Anchor[] = [];
  for (const row of state.rows) {
    if (row.label.trim() === "") continue;
    for (const [colId, text] of Object.entries(row.cells)) {
      if (text.trim() === "") continue;
      out.push({
        id: nid(),
        material_id: "",
        block_id: "",
        start: 0,
        end: 0,
        quote: row.label,
        dimension: colId,
        author: "student",
        question: "",
        answer: text,
      });
    }
  }
  return out;
}

// Anchor[] -> MatrixState — the inverse, used to rehydrate the primitive's
// working state from persisted anchors. Anchors are grouped by `quote` (the
// row label) in first-seen order; each group becomes a row `{id: nid(),
// label: quote, cells: {dimension: answer}, author: "student"}`. An anchor
// whose `dimension` is not a declared column id is dropped (defensive
// against config drift), as is an anchor with a blank `quote` (mirrors the
// forward direction's blank-label-rows-dropped rule; the server's
// firstIncompleteMatrixRow applies the same guard). Rows are rehydrated even
// when INCOMPLETE — a half-filled matrix must survive a reload, matching the
// server's own row rule (apps/api/internal/agent/card_effects.go
// completeMatrixRows / card_completion.go firstIncompleteMatrixRow), which
// only asks whether enough rows are complete, not that every row is.
export function anchorsToMatrixState(anchors: Anchor[], cols: Col[]): MatrixState {
  const colIds = new Set(cols.map((c) => c.id));
  const order: string[] = [];
  const cellsByLabel = new Map<string, Record<string, string>>();
  for (const a of anchors) {
    if (a.quote.trim() === "") continue;
    if (!colIds.has(a.dimension)) continue;
    if (!cellsByLabel.has(a.quote)) {
      cellsByLabel.set(a.quote, {});
      order.push(a.quote);
    }
    cellsByLabel.get(a.quote)![a.dimension] = a.answer;
  }
  const rows: MatrixRow[] = order.map((label) => ({
    id: nid(),
    label,
    cells: cellsByLabel.get(label)!,
    author: "student",
  }));
  return { rows };
}
