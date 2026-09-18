import { describe, expect, it } from "vitest";
import { normalizeField, normalizeHarvestStatus } from "./interestQuiz";

describe("interestQuiz response normalizers", () => {
  it("accepts all seven interest fields without collapsing science into formal", () => {
    const fields = [
      "formal",
      "science",
      "making",
      "society",
      "humanities",
      "arts",
      "self",
    ] as const;
    expect(fields.map(normalizeField)).toEqual(fields);
  });

  it("falls back safely for an unknown field", () => {
    expect(normalizeField("unknown")).toBe("self");
    expect(normalizeField(undefined)).toBe("self");
  });

  it("keeps the four harvest outcomes distinct", () => {
    const statuses = ["too_thin", "completed", "empty", "unavailable"] as const;
    expect(statuses.map(normalizeHarvestStatus)).toEqual(statuses);
  });

  it("treats a missing or future harvest status as unavailable", () => {
    expect(normalizeHarvestStatus(undefined)).toBe("unavailable");
    expect(normalizeHarvestStatus("failed")).toBe("unavailable");
  });
});
