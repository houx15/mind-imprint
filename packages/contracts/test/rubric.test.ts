import { describe, it, expect } from "vitest";
import { FULL_RUBRIC, SOLO_LABELS, CT_RUBRIC, assertRubricComplete } from "../src/rubric";

describe("FULL_RUBRIC", () => {
  it("has 10 dimensions D1..D10 in order", () => {
    expect(FULL_RUBRIC.map((d) => d.id)).toEqual(["D1","D2","D3","D4","D5","D6","D7","D8","D9","D10"]);
  });
  it("every dim has a name, framework, and all 4 SOLO anchors", () => {
    for (const d of FULL_RUBRIC) {
      expect(d.name.length).toBeGreaterThan(0);
      expect(d.framework.length).toBeGreaterThan(0);
      for (const lvl of ["L1","L2","L3","L4"] as const) {
        expect(d.anchors[lvl].length).toBeGreaterThan(0);
      }
    }
  });
  it("SOLO_LABELS unchanged", () => {
    expect(SOLO_LABELS).toEqual({ L1: "萌芽", L2: "发展中", L3: "熟练", L4: "卓越" });
  });
});

describe("Rubric container (C2)", () => {
  it("CT_RUBRIC has id 'ct'", () => {
    expect(CT_RUBRIC.id).toBe("ct");
  });

  it("CT_RUBRIC has exactly 10 dimensions", () => {
    expect(CT_RUBRIC.dimensions.length).toBe(10);
  });

  it("every dimension in CT_RUBRIC has non-empty L1-L4 anchors", () => {
    for (const d of CT_RUBRIC.dimensions) {
      for (const lvl of ["L1","L2","L3","L4"] as const) {
        expect(d.anchors[lvl].trim()).not.toBe("");
      }
    }
  });

  it("assertRubricComplete does not throw for CT_RUBRIC", () => {
    expect(() => assertRubricComplete(CT_RUBRIC)).not.toThrow();
  });

  it("assertRubricComplete throws for a rubric with a blank anchor", () => {
    const badRubric = {
      id: "bad",
      name: "Bad Rubric",
      dimensions: [{
        id: "D1",
        name: "Test",
        framework: "Test Framework",
        anchors: { L1: "fine", L2: "", L3: "fine", L4: "fine" },
      }],
    };
    expect(() => assertRubricComplete(badRubric)).toThrow();
  });
});
