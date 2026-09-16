import { describe, expect, it } from "vitest";
import { completedSteps, executionStatus } from "./stepProgress";

describe("plan execution progress", () => {
  it("counts saved work independently of approval status", () => {
    const steps = [
      { status: "tentative", progress: "done" as const },
      { status: "settled", progress: "done" as const },
      { status: "tentative", progress: "doing" as const },
      { status: "done", progress: "todo" as const },
      { status: "tentative", progress: "todo" as const },
    ];
    expect(completedSteps(steps)).toBe(2);
    expect(executionStatus(steps[3]!)).toBe("todo");
  });
  it("preserves ordinary project statuses and excludes skipped work", () => {
    expect(completedSteps([
      { status: "done" },
      { status: "awaiting_evidence" },
      { status: "cancelled", progress: "done" },
      { status: "skipped", progress: "done" },
    ])).toBe(1);
    expect(executionStatus({ status: "cancelled", progress: "done" })).toBe("cancelled");
    expect(completedSteps([])).toBe(0);
  });
});
