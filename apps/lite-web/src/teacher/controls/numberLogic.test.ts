import { describe, expect, it } from "vitest";
import { stepNumber } from "./numberLogic";

describe("stepNumber", () => {
  it("steps within the range and stops at its ends", () => {
    expect(stepNumber("600", 50, 50, 5000)).toBe("650");
    expect(stepNumber("5000", 50, 50, 5000)).toBeNull();
    expect(stepNumber("50", -50, 50, 5000)).toBeNull();
    expect(stepNumber("4990", 50, 50, 5000)).toBe("5000");
  });
  it("starts an empty box at the minimum", () => {
    expect(stepNumber("", 1, 0, 10)).toBe("0");
    expect(stepNumber("", -50, 50, 5000)).toBe("50");
    expect(stepNumber("", 1)).toBe("0");
  });
  it("moves an out-of-range value to the nearer end, only in the direction pressed", () => {
    expect(stepNumber("12", -1, 0, 10)).toBe("10");
    expect(stepNumber("12", 1, 0, 10)).toBeNull();
    expect(stepNumber("30", 50, 50, 5000)).toBe("50");
    expect(stepNumber("30", -50, 50, 5000)).toBeNull();
  });
});
