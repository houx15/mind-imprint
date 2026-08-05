import { describe, it, expect } from "vitest";
import type { ExplorationLead, QuestionEdge } from "@mind-imprint/contracts";
import {
  buildMindmapEdges,
  buildMindmapNodes,
  buildWarrenEdges,
  buildWarrenNodes,
  circlePositions,
  countPapersByRoot,
  dagreMindmapLayout,
  depthTint,
  mixToward,
  NODE_THEMES,
  radialLayout,
  rootOrdinal,
  themeForOrdinal,
  themeForRoot,
} from "@/workspace/blocks/exploration/warrenLayout";

function lead(overrides: Partial<ExplorationLead> & { id: string }): ExplorationLead {
  return {
    text: overrides.id,
    status: "open",
    origin: "manual",
    sourceReferenceId: null,
    connectedReferenceId: null,
    position: 0,
    parentLeadId: null,
    createdAt: "2026-08-01T00:00:00Z",
    ...overrides,
  };
}

describe("warrenLayout · countPapersByRoot", () => {
  it("counts only descendant leads that carry a connectedReferenceId (papers), any depth", () => {
    const leads: ExplorationLead[] = [
      lead({ id: "r1" }),
      // a sub-question (no paper) under r1 — must NOT count
      lead({ id: "q1", parentLeadId: "r1" }),
      // a paper under that sub-question — counts (grandchild depth)
      lead({ id: "p1", parentLeadId: "q1", status: "connected", connectedReferenceId: "ref-a" }),
      // a paper directly under r1 — counts
      lead({ id: "p2", parentLeadId: "r1", status: "connected", connectedReferenceId: "ref-b" }),
      // a second root with no descendants
      lead({ id: "r2" }),
    ];
    const counts = countPapersByRoot(leads);
    expect(counts.get("r1")).toBe(2);
    expect(counts.get("r2")).toBe(0);
    // only roots are keyed
    expect([...counts.keys()].sort()).toEqual(["r1", "r2"]);
  });
});

describe("warrenLayout · theme assignment (by ordinal)", () => {
  it("maps every ordinal in-range and wraps modulo the palette (incl. negatives)", () => {
    for (let i = 0; i < NODE_THEMES.length * 2; i++) {
      expect(themeForOrdinal(i)).toBe(NODE_THEMES[i % NODE_THEMES.length]);
    }
    // wrap-around: ordinal 0 and ordinal palette-length share a theme
    expect(themeForOrdinal(NODE_THEMES.length)).toBe(themeForOrdinal(0));
    // defensive on a negative ordinal (e.g. a not-found root)
    expect(NODE_THEMES).toContain(themeForOrdinal(-1));
  });

  it("gives ADJACENT roots visibly different themes (the anti-collision guarantee)", () => {
    for (let i = 0; i + 1 < NODE_THEMES.length; i++) {
      expect(themeForOrdinal(i).key).not.toBe(themeForOrdinal(i + 1).key);
    }
  });

  it("rootOrdinal is the root's index among top-level leads; themeForRoot resolves it", () => {
    const leads: ExplorationLead[] = [
      lead({ id: "r0" }),
      lead({ id: "child", parentLeadId: "r0" }), // not a root — must not shift ordinals
      lead({ id: "r1" }),
      lead({ id: "r2" }),
    ];
    expect(rootOrdinal("r0", leads)).toBe(0);
    expect(rootOrdinal("r1", leads)).toBe(1);
    expect(rootOrdinal("r2", leads)).toBe(2);
    expect(themeForRoot("r0", leads)).toBe(themeForOrdinal(0));
    expect(themeForRoot("r2", leads)).toBe(themeForOrdinal(2));
    // an unknown id falls back to ordinal 0 (still in-range, never throws)
    expect(rootOrdinal("nope", leads)).toBe(0);
  });
});

describe("warrenLayout · buildWarrenNodes theme spread", () => {
  it("themes roots by their array position so neighbours differ", () => {
    const roots = [lead({ id: "a" }), lead({ id: "b" }), lead({ id: "c" })];
    const nodes = buildWarrenNodes(roots, new Map());
    expect(nodes[0]!.theme).toBe(themeForOrdinal(0));
    expect(nodes[1]!.theme).toBe(themeForOrdinal(1));
    expect(nodes[2]!.theme).toBe(themeForOrdinal(2));
    expect(nodes[0]!.theme.key).not.toBe(nodes[1]!.theme.key);
  });
});

describe("warrenLayout · circlePositions", () => {
  it("puts a single node at the origin", () => {
    const m = circlePositions(["only"]);
    expect(m.get("only")).toEqual({ x: 0, y: 0 });
  });

  it("is deterministic and lays every id out distinctly for many nodes", () => {
    const ids = ["a", "b", "c", "d"];
    const first = circlePositions(ids);
    const second = circlePositions(ids);
    for (const id of ids) expect(first.get(id)).toEqual(second.get(id));
    const pts = ids.map((id) => `${first.get(id)!.x},${first.get(id)!.y}`);
    expect(new Set(pts).size).toBe(ids.length);
  });
});

describe("warrenLayout · buildWarrenNodes", () => {
  it("prefers a saved (dragged) position over the circle fallback", () => {
    const roots = [lead({ id: "r1" }), lead({ id: "r2" })];
    const nodes = buildWarrenNodes(roots, new Map([["r1", 3]]), { r1: { x: 42, y: -7 } });
    const n1 = nodes.find((n) => n.id === "r1")!;
    expect(n1.position).toEqual({ x: 42, y: -7 });
    expect(n1.paperCount).toBe(3);
    // r2 has no saved slot → falls back to a circle position (defined, non-null)
    const n2 = nodes.find((n) => n.id === "r2")!;
    expect(n2.position).toBeDefined();
    expect(n2.paperCount).toBe(0);
  });
});

describe("warrenLayout · buildWarrenEdges", () => {
  const edge = (over: Partial<QuestionEdge> & { id: string }): QuestionEdge => ({
    fromLeadId: "r1",
    toLeadId: "r2",
    label: "支持",
    status: "confirmed",
    ...over,
  });

  it("keeps only edges whose endpoints are both roots", () => {
    const rootIds = new Set(["r1", "r2"]);
    const edges = [edge({ id: "e1" }), edge({ id: "e2", toLeadId: "not-a-root" })];
    const built = buildWarrenEdges(edges, rootIds);
    expect(built.map((e) => e.id)).toEqual(["e1"]);
    expect(built[0]).toMatchObject({ source: "r1", target: "r2", label: "支持", status: "confirmed" });
  });
});

// ---------------------- GVb · Level-2 mindmap helpers ----------------------

describe("warrenLayout · mixToward / depthTint", () => {
  it("mixToward blends a color toward a target by t", () => {
    expect(mixToward("#000000", "#ffffff", 0.5)).toBe("#808080");
    expect(mixToward("#000000", "#ffffff", 0)).toBe("#000000");
    expect(mixToward("#000000", "#ffffff", 1)).toBe("#ffffff");
  });

  it("depthTint returns the base theme at depth 0 and lightens the fill deeper (border/label unchanged)", () => {
    const base = NODE_THEMES[0]!;
    expect(depthTint(base, 0)).toBe(base);
    const d2 = depthTint(base, 2);
    expect(d2.border).toBe(base.border);
    expect(d2.label).toBe(base.label);
    // a lighter fill than the root
    expect(d2.fillFrom).not.toBe(base.fillFrom);
  });
});

describe("warrenLayout · radialLayout", () => {
  it("puts the root at the origin and its children on a distinct ring", () => {
    const children = new Map<string, ExplorationLead[]>([
      ["r", [lead({ id: "a", parentLeadId: "r" }), lead({ id: "b", parentLeadId: "r" })]],
    ]);
    const pos = radialLayout("r", children);
    expect(pos.get("r")).toMatchObject({ x: 0, y: 0, depth: 0 });
    expect(pos.get("a")!.depth).toBe(1);
    expect(pos.get("b")!.depth).toBe(1);
    // the two children get distinct positions
    const a = pos.get("a")!;
    const b = pos.get("b")!;
    expect(`${a.x},${a.y}`).not.toBe(`${b.x},${b.y}`);
  });

  it("assigns increasing depth down a chain (grandchild deeper than child)", () => {
    const children = new Map<string, ExplorationLead[]>([
      ["r", [lead({ id: "p1", parentLeadId: "r" })]],
      ["p1", [lead({ id: "p2", parentLeadId: "p1" })]],
    ]);
    const pos = radialLayout("r", children);
    expect(pos.get("p1")!.depth).toBe(1);
    expect(pos.get("p2")!.depth).toBe(2);
  });
});

describe("warrenLayout · dagreMindmapLayout", () => {
  // Node footprints must match the pure fn's constants / MindmapNodeView.
  const sizeOf = (rootId: string, id: string) =>
    id === rootId ? { w: 216, h: 108 } : { w: 176, h: 88 };
  const overlaps = (
    a: { x: number; y: number; w: number; h: number },
    b: { x: number; y: number; w: number; h: number },
  ) => a.x < b.x + b.w && a.x + a.w > b.x && a.y < b.y + b.h && a.y + a.h > b.y;

  it("anchors the root at the origin and places every descendant", () => {
    const children = new Map<string, ExplorationLead[]>([
      ["r", [lead({ id: "a", parentLeadId: "r" }), lead({ id: "b", parentLeadId: "r" })]],
      ["a", [lead({ id: "c", parentLeadId: "a" })]],
    ]);
    const pos = dagreMindmapLayout("r", children);
    // root top-left anchored at (0,0)
    expect(pos.get("r")).toMatchObject({ x: 0, y: 0, depth: 0 });
    // all four nodes placed with the right depths
    expect([...pos.keys()].sort()).toEqual(["a", "b", "c", "r"]);
    expect(pos.get("a")!.depth).toBe(1);
    expect(pos.get("b")!.depth).toBe(1);
    expect(pos.get("c")!.depth).toBe(2);
  });

  it("is deterministic — same input yields identical positions", () => {
    const mk = () =>
      new Map<string, ExplorationLead[]>([
        ["r", [lead({ id: "a", parentLeadId: "r" }), lead({ id: "b", parentLeadId: "r" }), lead({ id: "d", parentLeadId: "r" })]],
      ]);
    const first = dagreMindmapLayout("r", mk());
    const second = dagreMindmapLayout("r", mk());
    for (const id of ["r", "a", "b", "d"]) expect(first.get(id)).toEqual(second.get(id));
  });

  it("lays a left-to-right tree with NO overlapping node rectangles", () => {
    const kids = Array.from({ length: 6 }, (_v, i) => lead({ id: `k${i}`, parentLeadId: "r" }));
    const children = new Map<string, ExplorationLead[]>([["r", kids]]);
    const pos = dagreMindmapLayout("r", children);
    const rects = [...pos.entries()].map(([id, p]) => ({ id, x: p.x, y: p.y, ...sizeOf("r", id) }));
    for (let i = 0; i < rects.length; i++) {
      for (let j = i + 1; j < rects.length; j++) {
        expect(overlaps(rects[i]!, rects[j]!)).toBe(false);
      }
    }
    // LR: children sit to the RIGHT of the root (greater x)
    for (const k of kids) expect(pos.get(k.id)!.x).toBeGreaterThan(pos.get("r")!.x);
  });
});

describe("warrenLayout · buildMindmapNodes / buildMindmapEdges", () => {
  const leads: ExplorationLead[] = [
    lead({ id: "r", text: "根问题" }),
    lead({ id: "p", text: "一篇论文", parentLeadId: "r", status: "connected", connectedReferenceId: "ref-1" }),
    lead({ id: "q", text: "子问题", parentLeadId: "r" }),
    // a lead OUTSIDE this root's subtree — must not appear
    lead({ id: "other", text: "别的根" }),
  ];

  it("marks the root as a question, a connected lead as a paper, and excludes out-of-subtree leads", () => {
    const nodes = buildMindmapNodes("r", leads);
    const ids = nodes.map((n) => n.id).sort();
    expect(ids).toEqual(["p", "q", "r"]);
    expect(nodes.find((n) => n.id === "r")).toMatchObject({ isRoot: true, kind: "question", depth: 0 });
    expect(nodes.find((n) => n.id === "p")).toMatchObject({ isRoot: false, kind: "paper" });
    expect(nodes.find((n) => n.id === "q")).toMatchObject({ kind: "question" });
    expect(nodes.some((n) => n.id === "other")).toBe(false);
  });

  it("prefers a saved (dragged) position over the dagre slot", () => {
    const nodes = buildMindmapNodes("r", leads, { p: { x: 11, y: 22 } });
    expect(nodes.find((n) => n.id === "p")!.position).toEqual({ x: 11, y: 22 });
  });

  it("tints descendants in the ROOT's theme family — same border/label, lighter fill", () => {
    // "r" is the first (and only) root here → ordinal 0 → themeForOrdinal(0).
    const base = themeForOrdinal(0);
    const nodes = buildMindmapNodes("r", leads);
    const root = nodes.find((n) => n.id === "r")!;
    const paper = nodes.find((n) => n.id === "p")!; // depth 1 descendant
    expect(root.theme.border).toBe(base.border);
    // descendant keeps the family's border + label hue…
    expect(paper.theme.border).toBe(base.border);
    expect(paper.theme.label).toBe(base.label);
    // …but a lighter fill than the root (depth tint)
    expect(paper.theme.fillFrom).not.toBe(base.fillFrom);
  });

  it("excludes pruned leads from the mindmap", () => {
    const withPruned = [...leads, lead({ id: "gone", text: "剪掉的", parentLeadId: "r", status: "pruned" })];
    const nodes = buildMindmapNodes("r", withPruned);
    expect(nodes.some((n) => n.id === "gone")).toBe(false);
  });

  it("builds provenance edges (parent → child) only within the subtree", () => {
    const edges = buildMindmapEdges("r", leads);
    const pairs = edges.map((e) => `${e.source}->${e.target}`).sort();
    expect(pairs).toEqual(["r->p", "r->q"]);
  });
});
