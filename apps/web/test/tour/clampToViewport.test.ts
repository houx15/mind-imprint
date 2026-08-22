import { describe, it, expect } from "vitest";
import { clampToViewport } from "@/tour/viewport";

const viewport = { vw: 1024, vh: 768 };

describe("clampToViewport", () => {
  it("leaves a position with room to spare unchanged", () => {
    const pos = { top: 100, left: 100 };
    const size = { w: 360, h: 200 };
    expect(clampToViewport(pos, size, viewport)).toEqual(pos);
  });

  it("pulls a right-overflowing box back so left + w fits within the margin", () => {
    const pos = { top: 100, left: 900 }; // 900 + 360 = 1260, way past 1024
    const size = { w: 360, h: 200 };
    const clamped = clampToViewport(pos, size, viewport, 8);
    expect(clamped.left + size.w).toBeLessThanOrEqual(viewport.vw - 8);
    expect(clamped.top).toBe(100); // top had room; only left needed correcting
  });

  it("pulls a bottom-overflowing box back so top + h fits within the margin", () => {
    const pos = { top: 700, left: 100 }; // 700 + 200 = 900, past 768
    const size = { w: 360, h: 200 };
    const clamped = clampToViewport(pos, size, viewport, 8);
    expect(clamped.top + size.h).toBeLessThanOrEqual(viewport.vh - 8);
    expect(clamped.left).toBe(100);
  });

  it("clamps a position that overflows both the right and bottom edges", () => {
    const pos = { top: 750, left: 1000 };
    const size = { w: 360, h: 200 };
    const clamped = clampToViewport(pos, size, viewport, 8);
    expect(clamped.left + size.w).toBeLessThanOrEqual(viewport.vw - 8);
    expect(clamped.top + size.h).toBeLessThanOrEqual(viewport.vh - 8);
  });

  it("never returns a coordinate below the margin, even off-screen to the top-left", () => {
    const pos = { top: -500, left: -500 };
    const size = { w: 360, h: 200 };
    const clamped = clampToViewport(pos, size, viewport, 8);
    expect(clamped.top).toBeGreaterThanOrEqual(8);
    expect(clamped.left).toBeGreaterThanOrEqual(8);
  });

  it("never returns a coordinate below the margin even in the degenerate case where the box exceeds the viewport", () => {
    const pos = { top: 100, left: 100 };
    const size = { w: 2000, h: 2000 }; // bigger than the viewport itself
    const clamped = clampToViewport(pos, size, viewport, 8);
    expect(clamped.top).toBeGreaterThanOrEqual(8);
    expect(clamped.left).toBeGreaterThanOrEqual(8);
  });
});
