import { describe, expect, it } from "vitest";
import { guessLang } from "./guessLang";

// Same cases as TestCreateWriting_GuessesLanguageWhenNotGiven (Go).
describe("guessLang", () => {
  it.each([
    ["Do you agree or disagree: it is better for students to work in groups?", "en"],
    ["People put in less effort when they believe their contribution cannot be seen.", "en"],
    ["学校应不应该取消期中考试", "zh"],
    ["关于 AI 的一篇议论文", "zh"],
    ["OK", "zh"],
  ])("%s → %s", (text, want) => {
    expect(guessLang(text)).toBe(want);
  });
});
