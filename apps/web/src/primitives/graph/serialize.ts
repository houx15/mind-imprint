import type { Anchor, GraphState, GraphNodeUnit } from "@mind-imprint/contracts";

// Slot config arrives from the card spec's `params.slots` (e.g. toulmin.json).
// Field names mirror the Go struct (`apps/api/internal/cards/loader.go` Slot)
// exactly: id / role / needSrc / q. The primitive never hardcodes the five
// Toulmin roles — it only knows the shape of a slot.
export type Slot = { id: string; role: string; needSrc: boolean; q: string };

let seq = 0;
const nid = () => `ga_${++seq}`;

// GraphState -> Anchor[] — the wire carrier the studio card locks with.
// One text anchor per node (dimension = node.type, answer = node.text,
// material_id = ""), plus one source anchor per `cites` edge (dimension =
// the *from* node's type, material_id = edge.to, answer = ""). The `supports`
// edge is structural (re-derived by anchorsToGraphState on load) and is
// never itself serialized to an anchor — the backend has no use for it and
// EvaluateCompletion/GraphEffects only ever look at dimension + answer/material_id.
export function graphStateToAnchors(state: GraphState): Anchor[] {
  const out: Anchor[] = [];
  for (const n of state.nodes) {
    out.push({
      id: nid(),
      material_id: "",
      block_id: "",
      start: 0,
      end: 0,
      quote: "",
      dimension: n.type,
      author: "student",
      question: "",
      answer: n.text,
    });
  }
  const typeOf = new Map(state.nodes.map((n) => [n.id, n.type]));
  for (const e of state.edges) {
    if (e.type !== "cites") continue; // supports is structural, re-derived on load
    out.push({
      id: nid(),
      material_id: e.to,
      block_id: "",
      start: 0,
      end: 0,
      quote: "",
      dimension: typeOf.get(e.from) ?? e.from,
      author: "student",
      question: "",
      answer: "",
    });
  }
  return out;
}

// Anchor[] -> GraphState — the inverse, used to rehydrate the primitive's
// working state from persisted anchors. Text anchors (non-empty `answer`)
// become nodes; source anchors (non-empty `material_id`) become `cites`
// edges. When both a `claim` and an `evidence` node exist, a synthetic
// `supports` edge (evidence -> claim) is added — the structural fact that
// evidence backs the claim, not something the student free-draws.
export function anchorsToGraphState(anchors: Anchor[], slots: Slot[]): GraphState {
  const nodes: GraphNodeUnit[] = [];
  const edges: GraphState["edges"] = [];
  for (const slot of slots) {
    const text = anchors.find((a) => a.dimension === slot.id && a.answer.trim() !== "");
    if (!text) continue;
    nodes.push({ id: slot.id, type: slot.id, text: text.answer, author: "student" });
    for (const src of anchors.filter((a) => a.dimension === slot.id && a.material_id !== "")) {
      edges.push({ id: nid(), from: slot.id, to: src.material_id, type: "cites" });
    }
  }
  const hasEvidence = nodes.some((n) => n.type === "evidence");
  const hasClaim = nodes.some((n) => n.type === "claim");
  if (hasEvidence && hasClaim) {
    edges.push({ id: nid(), from: "evidence", to: "claim", type: "supports" });
  }
  return { nodes, edges };
}
