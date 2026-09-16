import { describe, expect, it } from "vitest";
import { keptLabels } from "./AssignmentAIMode";

// The "text" patch key (set_material's pasted-article source) must show
// up in the kept-fields banner as the Chinese word, never the wire name —
// the same rule every other AssignmentDraft field already follows.
describe("keptLabels", () => {
  it("labels a kept text field in Chinese", () => {
    expect(keptLabels(["text"])).toBe("正文");
  });

  it("joins several kept fields with 、", () => {
    expect(keptLabels(["title", "text"])).toBe("标题、正文");
  });

  it("falls back to the raw key for a field with no label, rather than dropping it", () => {
    expect(keptLabels(["classId"])).toBe("classId");
  });
});
