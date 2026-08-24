import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import type { ExplorationLead, QuestionEdge } from "@mind-imprint/contracts";
import { circlePositions } from "../../../../src/workspace/blocks/exploration/warrenLayout";
import {
  computeUnfiledSlot,
  routeNodeClick,
  UNFILED_NODE_ID,
  WarrenMap,
} from "../../../../src/workspace/blocks/exploration/WarrenMap";

// React Flow can't render in jsdom (needs ResizeObserver + a measured
// container). Mirrors the shim in ExplorationView.test.tsx: each node renders
// through the REAL nodeTypes component (WarrenNodeView/UnfiledNodeView), so
// the card's actual JSX — including the 已读 badge and its data-tour anchor —
// lands in the DOM for assertions.
vi.mock("@xyflow/react", async () => {
  const React = await import("react");
  const Position = { Left: "left", Right: "right", Top: "top", Bottom: "bottom" };
  type AnyProps = Record<string, any>;
  return {
    __esModule: true,
    Position,
    ReactFlow: ({ nodes = [], nodeTypes = {} }: AnyProps) =>
      React.createElement(
        "div",
        { "data-testid": "rf-mock" },
        ...nodes.map((n: AnyProps) => {
          const C = nodeTypes[n.type];
          return React.createElement(
            "div",
            { key: n.id, "data-testid": "rf-node" },
            C ? React.createElement(C, { id: n.id, data: n.data }) : null,
          );
        }),
      ),
    ReactFlowProvider: ({ children }: AnyProps) => React.createElement(React.Fragment, null, children),
    Background: () => null,
    Controls: () => null,
    Handle: () => null,
    BaseEdge: () => null,
    EdgeLabelRenderer: ({ children }: AnyProps) => React.createElement(React.Fragment, null, children),
    getBezierPath: () => ["", 0, 0],
    applyNodeChanges: (_changes: any, nodes: any) => nodes,
  };
});

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

// noop handlers for the props WarrenMap needs but these tests don't exercise.
const noopMapProps = {
  projectId: "p1",
  countByRoot: new Map<string, number>(),
  edges: [] as QuestionEdge[],
  onZoom: vi.fn(),
  unfiledCount: 0,
  onOpenUnfiled: vi.fn(),
  busyEdgeIds: new Set<string>(),
  onConfirmEdge: vi.fn(),
  onDismissEdge: vi.fn(),
  onRelabelEdge: vi.fn(),
  onCreateEdge: vi.fn(),
  onDeleteLead: vi.fn(),
} as const;

describe("WarrenMap node-click routing", () => {
  it("routes the 未归类 sentinel to onOpenUnfiled, not onZoom", () => {
    const onZoom = vi.fn();
    const onOpen = vi.fn();
    routeNodeClick(UNFILED_NODE_ID, onZoom, onOpen);
    expect(onOpen).toHaveBeenCalledTimes(1);
    expect(onZoom).not.toHaveBeenCalled();
  });
  it("routes a real root to onZoom", () => {
    const onZoom = vi.fn();
    const onOpen = vi.fn();
    routeNodeClick("root-1", onZoom, onOpen);
    expect(onZoom).toHaveBeenCalledWith("root-1");
    expect(onOpen).not.toHaveBeenCalled();
  });
});

// The 未归类 node used to sit at a hardcoded {x:0,y:320} — circlePositions'
// ellipse regularly puts a root right on the +y axis (e.g. the demo's 4-root
// layout), which collided with that hardcoded spot. computeUnfiledSlot must
// instead derive the slot from the roots' ACTUAL bounding box, so it clears
// every root regardless of count/spread.
const NODE_W = 208;
const NODE_H = 104;

describe("WarrenMap · computeUnfiledSlot", () => {
  it("for the demo's 4-root circle layout, lands below every root — never equal to one", () => {
    const rootIds = ["r1", "r2", "r3", "r4"];
    const circle = circlePositions(rootIds);
    const positions = rootIds.map((id) => circle.get(id)!);
    const slot = computeUnfiledSlot(positions);

    // Strictly below the lowest root's bottom edge (not just its top).
    const maxBottom = Math.max(...positions.map((p) => p.y + NODE_H));
    expect(slot.y).toBeGreaterThan(maxBottom);

    // Never coincides with any root's own position (the exact collision the
    // old hardcoded {x:0,y:320} could hit).
    for (const p of positions) {
      expect(`${slot.x},${slot.y}`).not.toBe(`${p.x},${p.y}`);
    }

    // No rectangle overlap either (belt-and-suspenders on top of the y check).
    const overlaps = (a: { x: number; y: number }, b: { x: number; y: number }) =>
      a.x < b.x + NODE_W && a.x + NODE_W > b.x && a.y < b.y + NODE_H && a.y + NODE_H > b.y;
    for (const p of positions) expect(overlaps(slot, p)).toBe(false);
  });

  it("stays clear for other root counts too (1, 2, 7)", () => {
    for (const n of [1, 2, 7]) {
      const rootIds = Array.from({ length: n }, (_v, i) => `r${i}`);
      const circle = circlePositions(rootIds);
      const positions = rootIds.map((id) => circle.get(id)!);
      const slot = computeUnfiledSlot(positions);
      const maxBottom = Math.max(...positions.map((p) => p.y + NODE_H));
      expect(slot.y).toBeGreaterThan(maxBottom);
    }
  });

  it("centers under the roots' bounding box (x)", () => {
    const positions = [
      { x: -100, y: 0 },
      { x: 100, y: 0 },
    ];
    const slot = computeUnfiledSlot(positions);
    // bbox spans [-100, 100+208]; its center minus half the node's own width.
    expect(slot.x).toBe(Math.round((-100 + (100 + NODE_W)) / 2 - NODE_W / 2));
  });

  it("falls back to a fixed slot with zero roots", () => {
    expect(computeUnfiledSlot([])).toEqual({ x: 0, y: NODE_H + 150 });
  });
});

describe("WarrenMap · 已读 read badge", () => {
  it("renders the 已读 badge on a root with a done contained reference (data-driven, no tour anchor)", async () => {
    const roots = [
      lead({ id: "r-done", text: "有已读来源的问题" }),
      lead({ id: "r-unread", text: "没有已读来源的问题" }),
    ];
    const { container } = render(
      <WarrenMap {...noopMapProps} roots={roots} readByRoot={new Set(["r-done"])} />,
    );
    await screen.findByText("有已读来源的问题");

    const cards = Array.from(container.querySelectorAll<HTMLElement>('[data-tour="warren-question"]'));
    expect(cards.length).toBe(2);
    const doneCard = cards.find((c) => c.textContent?.includes("有已读来源的问题"));
    const unreadCard = cards.find((c) => c.textContent?.includes("没有已读来源的问题"));
    expect(doneCard).toBeTruthy();
    expect(unreadCard).toBeTruthy();

    // The badge itself still shows on the data-done root...
    expect(doneCard!.textContent).toContain("已读✓");
    expect(unreadCard!.textContent).not.toContain("已读✓");

    // ...but WITHOUT justReadRootIds (no demo override), NO node carries the
    // tour anchor — a plain data-done badge is not "just read" by the tour.
    expect(doneCard!.querySelector('[data-tour="warren-node-read"]')).toBeNull();
    expect(container.querySelector('[data-tour="warren-node-read"]')).toBeNull();
  });

  it("shows no badge anywhere when readByRoot is omitted (normal-graph regression guard)", async () => {
    const roots = [lead({ id: "r1", text: "普通问题" })];
    const { container } = render(<WarrenMap {...noopMapProps} roots={roots} />);
    await screen.findByText("普通问题");
    expect(container.querySelector('[data-tour="warren-node-read"]')).toBeNull();
  });

  // P7 cross-seam fix: the `warren-node-read` tour anchor must resolve to
  // EXACTLY the demo override root, never any other badged (data-done) root —
  // otherwise the tour spotlight is ambiguous when multiple roots are 已读.
  it("with a demo justReadRootIds override AND other data-done roots, the anchor lands on ONLY the override node", async () => {
    const roots = [
      lead({ id: "r-just-read", text: "刚被标记已读的问题" }),
      lead({ id: "r-data-done-1", text: "数据已读问题一" }),
      lead({ id: "r-data-done-2", text: "数据已读问题二" }),
      lead({ id: "r-unread", text: "还没读的问题" }),
    ];
    const { container } = render(
      <WarrenMap
        {...noopMapProps}
        roots={roots}
        readByRoot={new Set(["r-just-read", "r-data-done-1", "r-data-done-2"])}
        justReadRootIds={new Set(["r-just-read"])}
      />,
    );
    await screen.findByText("刚被标记已读的问题");

    const cards = Array.from(container.querySelectorAll<HTMLElement>('[data-tour="warren-question"]'));
    const cardFor = (text: string) => cards.find((c) => c.textContent?.includes(text))!;

    // All three data-done-or-override roots still show the 已读 badge...
    expect(cardFor("刚被标记已读的问题").textContent).toContain("已读✓");
    expect(cardFor("数据已读问题一").textContent).toContain("已读✓");
    expect(cardFor("数据已读问题二").textContent).toContain("已读✓");
    expect(cardFor("还没读的问题").textContent).not.toContain("已读✓");

    // ...but the tour anchor resolves to EXACTLY one element, on the override node.
    const anchors = container.querySelectorAll('[data-tour="warren-node-read"]');
    expect(anchors.length).toBe(1);
    expect(cardFor("刚被标记已读的问题").querySelector('[data-tour="warren-node-read"]')).not.toBeNull();
    expect(cardFor("数据已读问题一").querySelector('[data-tour="warren-node-read"]')).toBeNull();
    expect(cardFor("数据已读问题二").querySelector('[data-tour="warren-node-read"]')).toBeNull();
  });

  it("an empty justReadRootIds override places no anchor, even with data-done roots present", async () => {
    const roots = [lead({ id: "r-done", text: "数据已读问题" })];
    const { container } = render(
      <WarrenMap
        {...noopMapProps}
        roots={roots}
        readByRoot={new Set(["r-done"])}
        justReadRootIds={new Set()}
      />,
    );
    await screen.findByText("数据已读问题");
    expect(container.querySelector('[data-tour="warren-node-read"]')).toBeNull();
  });
});
