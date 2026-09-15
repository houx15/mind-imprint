import { describe, expect, it } from "vitest";
import { GRADINGS_CHANGED_EVENT, gradingsChangedAtom } from "./gradingsChanged";

// FinishedWritingPage refetches only when the event names its own writing;
// an event for another writing, or one with no detail, must be ignored.
describe("gradingsChangedAtom", () => {
  it("reads the writing id from the event", () => {
    expect(gradingsChangedAtom(new CustomEvent(GRADINGS_CHANGED_EVENT, { detail: { atomId: "w1" } }))).toBe("w1");
  });
  it("is null for an event without a writing id", () => {
    expect(gradingsChangedAtom(new Event(GRADINGS_CHANGED_EVENT))).toBeNull();
    expect(gradingsChangedAtom(new CustomEvent(GRADINGS_CHANGED_EVENT, { detail: { atomId: 7 } }))).toBeNull();
  });
});
