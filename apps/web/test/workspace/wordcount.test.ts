import { describe, it, expect } from "vitest";
import { countWords } from "@/workspace/blocks/wordcount";

// Locks in BUG-02 (2026-08-25 findings): the writing-room live counter must
// count words the same CJK-aware way the backend (agent.CountWords) does, so
// the "字" number agrees with the 整稿体检 review and the word-budget band. The
// old `text.replace(/\s+/g,"").length` counted characters — ~5× too high for
// English against an "约 800 词" target.
describe("countWords (mirrors backend agent.CountWords)", () => {
  it("counts English by words, not characters", () => {
    expect(countWords("China alone accounts for a quarter of the greening")).toBe(9);
  });

  it("counts CJK by character", () => {
    expect(countWords("中国让地球更可持续")).toBe(9);
  });

  it("mixes CJK chars and Latin words", () => {
    // 中 国 的 在 (4 Han) + "GDP" (1) + "2024" (1) = 6
    expect(countWords("中国的 GDP 在 2024")).toBe(6);
  });

  it("ignores extra whitespace and newlines", () => {
    expect(countWords("  hello   world \n\n foo ")).toBe(3);
  });

  it("is zero for empty / whitespace-only", () => {
    expect(countWords("")).toBe(0);
    expect(countWords("   \n\t ")).toBe(0);
  });
});
