import { describe, it, expect } from "vitest";
import { radarGeometry } from "./radarGeometry";

describe("radarGeometry", () => {
  it("produces one axis/dot/label per level and 4 rings", () => {
    const names = ["D1","D2","D3","D4","D5","D6","D7","D8","D9"];
    const g = radarGeometry([1,2,3,4,1,2,3,4,2], names);
    expect(g.axes).toHaveLength(9);
    expect(g.dots).toHaveLength(9);
    expect(g.labels).toHaveLength(9);
    expect(g.rings).toHaveLength(4);
    expect(g.polygonPoints.split(" ")).toHaveLength(9);
  });
  it("dot radius grows with level (L4 farther from center than L1)", () => {
    const g = radarGeometry([1,1,1,1,1,1,1,1,4], new Array(9).fill("x"));
    const center = { x: 140, y: 128 };
    const dist = (d: { cx: number; cy: number }) => Math.hypot(d.cx - center.x, d.cy - center.y);
    expect(dist(g.dots[8]!)).toBeGreaterThan(dist(g.dots[0]!));
  });
});
