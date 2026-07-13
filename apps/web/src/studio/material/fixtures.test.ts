import { describe, it, expect } from "vitest";
import { Author } from "@mind-imprint/contracts";
import { SOURCE_FIXTURES } from "./fixtures";

describe("SOURCE_FIXTURES", () => {
  it("has at least two entries", () => {
    expect(SOURCE_FIXTURES.length).toBeGreaterThanOrEqual(2);
  });

  it("has at least one article source with a block and a real AI span (tag + note)", () => {
    const article = SOURCE_FIXTURES.find((s) => s.view === "article");
    expect(article).toBeTruthy();
    expect(article!.blocks.length).toBeGreaterThanOrEqual(1);
    const aiSpans = article!.annotate.spans.filter((sp) => sp.author === "ai");
    expect(aiSpans.length).toBeGreaterThanOrEqual(1);
    for (const span of aiSpans) {
      expect(span.tag.trim().length).toBeGreaterThan(0);
      expect(span.note.trim().length).toBeGreaterThan(0);
    }
  });

  it("has at least one summary source with a takeaway", () => {
    const summary = SOURCE_FIXTURES.find((s) => s.view === "summary");
    expect(summary).toBeTruthy();
    expect(summary!.takeaway?.trim().length).toBeGreaterThan(0);
  });

  it("every span's author is a valid Author", () => {
    for (const source of SOURCE_FIXTURES) {
      for (const span of source.annotate.spans) {
        expect(Author.safeParse(span.author).success).toBe(true);
      }
    }
  });

  it("every annotate.material_id matches its source id", () => {
    for (const source of SOURCE_FIXTURES) {
      expect(source.annotate.material_id).toBe(source.id);
    }
  });
});
