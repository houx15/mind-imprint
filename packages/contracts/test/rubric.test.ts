import { describe, it, expect } from "vitest";
import { DUALAXIS_MODEL, assertModelComplete, type DualAxis } from "../src/rubric";

describe("DUALAXIS_MODEL", () => {
  it("parses the canonical config: 6 depth dims, 6 autonomy signals, 6 lenses, 1 standard", () => {
    expect(DUALAXIS_MODEL.depth).toHaveLength(6);
    expect(DUALAXIS_MODEL.autonomy).toHaveLength(6);
    expect(DUALAXIS_MODEL.lenses).toHaveLength(6);
    expect(DUALAXIS_MODEL.standards).toHaveLength(1);
    expect(DUALAXIS_MODEL.standards[0]!.components).toHaveLength(4);
    expect(DUALAXIS_MODEL.standards[0]!.alignmentItems).toHaveLength(5);
  });

  it("carries the axiom verbatim", () => {
    expect(DUALAXIS_MODEL.axiom).toBe(
      "两轴永不合成总分；单次会话为事件级证据，不构成人级档位判定",
    );
  });
});

describe("assertModelComplete", () => {
  it("passes for the complete canonical model", () => {
    expect(() => assertModelComplete(DUALAXIS_MODEL)).not.toThrow();
  });

  it("throws when a depth dim is missing an anchor", () => {
    const broken: DualAxis = {
      ...DUALAXIS_MODEL,
      depth: DUALAXIS_MODEL.depth.map((d, i) =>
        i === 0 ? { ...d, anchors: { ...d.anchors, L4: "" } } : d,
      ),
    };
    expect(() => assertModelComplete(broken)).toThrow();
  });

  it("throws when an autonomy signal is missing its event", () => {
    const broken: DualAxis = {
      ...DUALAXIS_MODEL,
      autonomy: DUALAXIS_MODEL.autonomy.map((a, i) => (i === 0 ? { ...a, event: "" } : a)),
    };
    expect(() => assertModelComplete(broken)).toThrow();
  });

  it("throws when a lens is missing its guide", () => {
    const broken: DualAxis = {
      ...DUALAXIS_MODEL,
      lenses: DUALAXIS_MODEL.lenses.map((l, i) => (i === 0 ? { ...l, guide: "" } : l)),
    };
    expect(() => assertModelComplete(broken)).toThrow();
  });
});
