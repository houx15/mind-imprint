import { describe, expect, it } from "vitest";
import { assignedPromptOf, isAssignedWriting } from "./writings";

// isAssignedWriting locks the language and target in the setup dialog and the
// room. Wrong one way, she cannot set her own writing's length; wrong the other
// way, she edits a target the server refuses with 409.
describe("isAssignedWriting", () => {
  it("is true only for a non-blank teacher prompt", () => {
    expect(isAssignedWriting({ assignedPrompt: "写一篇关于雨的记叙文" })).toBe(true);
    expect(isAssignedWriting({ assignedPrompt: null })).toBe(false);
    expect(isAssignedWriting({})).toBe(false);
    expect(isAssignedWriting({ assignedPrompt: "  " })).toBe(false);
  });
});

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
