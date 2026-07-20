import { describe, it, expect } from "vitest";
import { OnboardingFx, CreateProjectBody, OnboardingSubmitBody } from "../src/studioState";

describe("N1 contracts", () => {
  it("OnboardingFx parses the new fields", () => {
    const ob = OnboardingFx.parse({
      restatePrompt: "P", rubricRows: [], planSteps: [],
      assignmentText: "讨论 X", studentRestate: "我的理解", studentWeakPicks: [0, 2],
    });
    expect(ob.assignmentText).toBe("讨论 X");
    expect(ob.studentWeakPicks).toEqual([0, 2]);
  });
  it("CreateProjectBody requires a non-empty prompt", () => {
    expect(() => CreateProjectBody.parse({ prompt: "" })).toThrow();
    expect(CreateProjectBody.parse({ prompt: "x" }).title).toBeUndefined();
  });
  it("OnboardingSubmitBody shape", () => {
    const b = OnboardingSubmitBody.parse({ restate: "r", weakPicks: [1] });
    expect(b.weakPicks).toEqual([1]);
  });
});
