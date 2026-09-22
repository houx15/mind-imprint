import { describe, expect, it } from "vitest";
import { parseShowcaseInterests } from "./showcaseDraft";

// Keywords cross a strict API boundary. Duplication and Unicode length must agree
// with the server without truncating what the student entered.
describe("showcase keyword input", () => {
  it("accepts common Chinese separators and removes repeated keywords in original order", () => {
    expect(parseShowcaseInterests(" 科幻、动漫，科幻\n植物, ")).toEqual({ interests: ["科幻", "动漫", "植物"], error: "" });
  });
  it("reports too many distinct terms without silently dropping any", () => {
    const result = parseShowcaseInterests(Array.from({length: 13}, (_, i) => `兴趣${i}`).join("、"));
    expect(result.interests).toHaveLength(13);
    expect(result.error).toContain("12");
  });
  it("uses Unicode code points for the per-term boundary", () => {
    expect(parseShowcaseInterests("🌱".repeat(60)).error).toBe("");
    const result = parseShowcaseInterests("🌱".repeat(61));
    expect(result.error).toContain("60");
    expect(result.interests[0]).toBe("🌱".repeat(61));
  });
});
