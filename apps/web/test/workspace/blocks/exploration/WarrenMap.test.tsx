import { describe, it, expect, vi } from "vitest";
import { routeNodeClick, UNFILED_NODE_ID } from "../../../../src/workspace/blocks/exploration/WarrenMap";

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
