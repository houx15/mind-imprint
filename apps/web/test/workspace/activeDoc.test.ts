import { describe, it, expect } from "vitest";
import { activeDocForStage } from "../../src/workspace/activeDoc";

describe("activeDocForStage", () => {
  it("maps the proposal statuses to the proposal doc", () => {
    expect(activeDocForStage("proposal_writing")).toBe("proposal");
    expect(activeDocForStage("proposal_review")).toBe("proposal");
  });

  it("maps body writing to the essay doc", () => {
    expect(activeDocForStage("body_writing")).toBe("essay");
  });

  it("defaults every other status to the essay doc", () => {
    expect(activeDocForStage("topic_discussion")).toBe("essay");
    expect(activeDocForStage("proposal_forming")).toBe("essay");
    expect(activeDocForStage("plan_generation")).toBe("essay");
    expect(activeDocForStage("retrospective")).toBe("essay");
  });
});
