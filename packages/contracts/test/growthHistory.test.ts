import { describe, expect, it } from "vitest";
import { GrowthHistory } from "../src/growthHistory";

describe("GrowthHistory", () => {
  it("parses a representative payload with a nullable sublabel", () => {
    const parsed = GrowthHistory.parse({
      entries: [
        {
          surface: "course",
          scopeId: "00000000-0000-0000-0000-0000000000c1",
          label: "信息素养",
          sublabel: "回看",
          createdAt: "2026-07-18T00:00:00Z",
          report: { dimensions: [], narrative: "n", generatedAt: "2026-07-18T00:00:00Z" },
        },
        {
          surface: "project",
          scopeId: "11111111-1111-1111-1111-111111111111",
          label: "中国可持续",
          sublabel: null,
          createdAt: "2026-07-17T00:00:00Z",
          report: { dimensions: [], narrative: "n", generatedAt: "2026-07-17T00:00:00Z" },
        },
      ],
    });
    expect(parsed.entries).toHaveLength(2);
    expect(parsed.entries[1]!.sublabel).toBeNull();
  });

  it("rejects an unknown surface", () => {
    expect(() =>
      GrowthHistory.parse({ entries: [{ surface: "email", scopeId: "x", label: "x", sublabel: null, createdAt: "x", report: { dimensions: [], narrative: "", generatedAt: "" } }] }),
    ).toThrow();
  });
});
