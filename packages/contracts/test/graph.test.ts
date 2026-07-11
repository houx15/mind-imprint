import { describe, it, expect } from "vitest";
import { GraphNode, GraphEdge, GraphNodeType, NodeRefKind } from "../src/graph";

describe("workspace graph (hybrid)", () => {
  it("light node types are the argument-graph participants", () => {
    for (const t of ["claim", "evidence", "plan", "gate_state", "note"]) {
      expect(GraphNodeType.safeParse(t).success).toBe(true);
    }
  });
  it("a graph node carries project, type, body, author", () => {
    expect(GraphNode.safeParse({
      id: "n1", project_id: "p1", type: "claim",
      body: { text: "..." }, author: "student", created_at: "2026-07-11T00:00:00Z",
    }).success).toBe(true);
  });
  it("edges are polymorphic across heavy + light nodes", () => {
    expect(NodeRefKind.safeParse("card_instance").success).toBe(true);
    expect(GraphEdge.safeParse({
      id: "e1", project_id: "p1", type: "supports",
      from_kind: "card_instance", from_id: "c1", to_kind: "graph_node", to_id: "n1",
      created_at: "2026-07-11T00:00:00Z",
    }).success).toBe(true);
  });
});
