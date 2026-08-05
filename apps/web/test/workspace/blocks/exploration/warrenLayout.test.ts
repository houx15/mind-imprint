import { describe, it, expect } from "vitest";
import type { ExplorationLead, QuestionEdge } from "@mind-imprint/contracts";
import {
  buildWarrenEdges,
  buildWarrenNodes,
  circlePositions,
  countPapersByRoot,
  NODE_THEMES,
  themeForId,
  themeIndexForId,
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

describe("warrenLayout · theme assignment", () => {
  it("is stable for a given id and always in range", () => {
    expect(themeForId("lead-abc")).toBe(themeForId("lead-abc"));
    for (const id of ["a", "lead-1", "中国碳排放", "zzzzzzzz"]) {
      const idx = themeIndexForId(id);
      expect(idx).toBeGreaterThanOrEqual(0);
      expect(idx).toBeLessThan(NODE_THEMES.length);
      expect(NODE_THEMES[idx]).toBe(themeForId(id));
    }
  });

  it("spreads distinct ids across more than one theme", () => {
    const used = new Set(["l0", "l1", "l2", "l3", "l4", "l5", "l6", "l7"].map((id) => themeForId(id).key));
    expect(used.size).toBeGreaterThan(1);
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
