import { describe, it, expect } from "vitest";
import { SOLO_LABELS } from "../src/rubric";

describe("SOLO_LABELS", () => {
  it("unchanged", () => {
    expect(SOLO_LABELS).toEqual({ L1: "萌芽", L2: "发展中", L3: "熟练", L4: "卓越" });
  });
});
