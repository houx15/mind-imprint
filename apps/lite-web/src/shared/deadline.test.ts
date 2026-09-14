import { describe, expect, it } from "vitest";
import { beijingInputToISO, formatDeadline, isoToBeijingInput } from "./deadline";

describe("deadline", () => {
  it("treats the input as Beijing time", () => {
    expect(beijingInputToISO("2026-09-20T22:00")).toBe("2026-09-20T22:00:00+08:00");
  });
  it("rejects malformed input", () => {
    expect(beijingInputToISO("2026-09-20")).toBeNull();
    expect(beijingInputToISO("")).toBeNull();
  });
  it("formats in Beijing regardless of UTC instant", () => {
    expect(formatDeadline("2026-09-20T14:00:00Z")).toBe("9月20日 22:00");
    expect(formatDeadline("2026-09-20T16:30:00Z")).toBe("9月21日 00:30");
  });
  it("round-trips to the input format", () => {
    expect(isoToBeijingInput("2026-09-20T14:00:00Z")).toBe("2026-09-20T22:00");
  });
});
