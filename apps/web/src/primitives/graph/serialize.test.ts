import { expect, test } from "vitest";
import { graphStateToAnchors, anchorsToGraphState } from "./serialize";
import type { GraphState } from "@mind-imprint/contracts";

const slots = [
  { id: "claim", role: "核心主张", needSrc: false, q: "" },
  { id: "evidence", role: "支撑证据", needSrc: true, q: "" },
];

test("round-trips text + source through anchors", () => {
  const state: GraphState = {
    nodes: [
      { id: "claim", type: "claim", text: "核心判断一句话说清楚。", author: "student" },
      { id: "evidence", type: "evidence", text: "证据如何支撑主张。", author: "student" },
    ],
    edges: [
      { id: "e1", from: "evidence", to: "m_nasa", type: "cites" },
      { id: "e2", from: "evidence", to: "claim", type: "supports" },
    ],
  };
  const anchors = graphStateToAnchors(state);
  // two text anchors + one source anchor (supports edge is not an anchor)
  expect(anchors.filter((a) => a.answer !== "")).toHaveLength(2);
  expect(anchors.filter((a) => a.material_id !== "")).toHaveLength(1);
  expect(anchors.every((a) => a.author === "student")).toBe(true);

  const back = anchorsToGraphState(anchors, slots);
  expect(back.nodes.map((n) => n.type).sort()).toEqual(["claim", "evidence"]);
  expect(back.edges.find((e) => e.type === "cites")?.to).toBe("m_nasa");
  expect(back.edges.some((e) => e.type === "supports" && e.from === "evidence" && e.to === "claim")).toBe(true);
});

test("emits the exact per-field anchor shape the backend keys completion + mint on", () => {
  const state: GraphState = {
    nodes: [{ id: "claim", type: "claim", text: "核心判断一句话说清楚。", author: "student" }],
    edges: [{ id: "e1", from: "claim", to: "m_nasa", type: "cites" }],
  };
  const anchors = graphStateToAnchors(state);
  const text = anchors.find((a) => a.answer !== "");
  const source = anchors.find((a) => a.material_id !== "");
  expect(text).toEqual(
    expect.objectContaining({
      dimension: "claim",
      answer: "核心判断一句话说清楚。",
      author: "student",
      material_id: "",
      block_id: "",
      start: 0,
      end: 0,
      quote: "",
      question: "",
    }),
  );
  expect(source).toEqual(
    expect.objectContaining({
      dimension: "claim",
      material_id: "m_nasa",
      answer: "",
      author: "student",
      block_id: "",
      start: 0,
      end: 0,
      quote: "",
      question: "",
    }),
  );
  // ids are unique and non-empty (the backend keys anchors by id).
  expect(text!.id).not.toBe("");
  expect(source!.id).not.toBe("");
  expect(text!.id).not.toBe(source!.id);
});

test("a slot with only a short (<12 rune) or missing answer round-trips to no node", () => {
  // anchorsToGraphState treats any non-empty trimmed answer as a text hit —
  // the 12-rune completion gate lives in canLock / the backend, not here.
  // This test locks the *inverse* behavior: an anchor with a whitespace-only
  // answer must NOT produce a node (the `.trim() !== ""` guard).
  const anchors = [
    { id: "a1", material_id: "", block_id: "", start: 0, end: 0, quote: "", dimension: "claim", author: "student" as const, question: "", answer: "   " },
  ];
  const back = anchorsToGraphState(anchors, slots);
  expect(back.nodes).toHaveLength(0);
});
