import { describe, expect, it } from "vitest";
import { assignedPromptOf } from "./writings";

// assignedPromptOf decides whether the writing room renders 题目：… at all. A
// wrong answer either hides the teacher's prompt or renders an empty line.
describe("assignedPromptOf", () => {
  it("returns the teacher's prompt, trimmed", () => {
    expect(assignedPromptOf({ assignedPrompt: "  写一篇关于雨的记叙文 " })).toBe("写一篇关于雨的记叙文");
  });

  it("treats null, a missing field and a blank prompt as no prompt", () => {
    expect(assignedPromptOf({ assignedPrompt: null })).toBeNull();
    expect(assignedPromptOf({})).toBeNull();
    expect(assignedPromptOf({ assignedPrompt: "   " })).toBeNull();
  });
});
