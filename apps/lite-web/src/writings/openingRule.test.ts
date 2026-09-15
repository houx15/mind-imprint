import { describe, expect, it } from "vitest";
import { coachOpeningNeeded } from "./openingRule";

const open = { setupAt: "2026-09-15T00:00:00Z", status: "active", finishedAt: null };
const student = { role: "student" };
const ai = { role: "ai" };

describe("coachOpeningNeeded", () => {
  it("opens a new writing that has no line from 印记 yet", () => expect(coachOpeningNeeded(open, [student])).toBe(true));
  it("does not open before setup is done", () => expect(coachOpeningNeeded({ ...open, setupAt: null }, [])).toBe(false));
  it("does not open twice", () => expect(coachOpeningNeeded(open, [student, ai])).toBe(false));

  // 修改 reopens a finished writing. It already has a draft and at least one
  // version, so an opening line would be a model call that restarts a
  // conversation about a piece she has already submitted.
  it("does not open a finished writing she is revising", () =>
    expect(coachOpeningNeeded({ ...open, status: "finished", finishedAt: "2026-09-15T06:00:00Z" }, [student])).toBe(false));
  it("does not open when only finishedAt says it is finished", () =>
    expect(coachOpeningNeeded({ ...open, finishedAt: "2026-09-15T06:00:00Z" }, [])).toBe(false));
});
