import { describe, expect, it } from "vitest";
import { normalizeClassRecommendations } from "./teacherLibrary";

const article = (over: Record<string, unknown> = {}): Record<string, unknown> => ({
  slug: "coral-reefs",
  title: "Coral reefs are bleaching",
  zhTitle: "珊瑚礁正在白化",
  reason: "海水升温一度，珊瑚会怎样",
  field: "science",
  tags: [{ id: "biology", zh: "生物", field: "science" }],
  coverUrl: "https://cdn.example/c.jpg",
  levels: [{ tier: 2, name: "基础", lexile: 820, words: 600, minutes: 5 }],
  finished: false,
  why: ["生物"],
  readCount: 3,
  ...over,
});

describe("normalizeClassRecommendations", () => {
  it("keeps the article shape plus why and readCount", () => {
    const r = normalizeClassRecommendations({ articles: [article()], tier: 3 });
    expect(r.tier).toBe(3);
    expect(r.articles[0]).toMatchObject({ slug: "coral-reefs", zhTitle: "珊瑚礁正在白化", why: ["生物"], readCount: 3 });
    expect(r.articles[0]!.levels).toHaveLength(1);
  });
  it("defaults a missing why to [] and a missing readCount to 0", () => {
    const raw = article();
    delete raw.why;
    delete raw.readCount;
    const r = normalizeClassRecommendations({ articles: [raw], tier: 2 });
    expect(r.articles[0]!.why).toEqual([]);
    expect(r.articles[0]!.readCount).toBe(0);
  });
  it("treats a negative or fractional readCount as 0 and drops non-string reasons", () => {
    const r = normalizeClassRecommendations({
      articles: [article({ readCount: -2, why: ["生物", 4, ""] }), article({ slug: "b", readCount: 1.5 })],
    });
    expect(r.articles[0]!.readCount).toBe(0);
    expect(r.articles[0]!.why).toEqual(["生物"]);
    expect(r.articles[1]!.readCount).toBe(0);
  });
  it("maps a tier outside 1..5 to null", () => {
    expect(normalizeClassRecommendations({ articles: [], tier: 0 }).tier).toBeNull();
    expect(normalizeClassRecommendations({ articles: [], tier: "3" }).tier).toBeNull();
    expect(normalizeClassRecommendations({ articles: [] }).tier).toBeNull();
  });
  it("drops articles without a slug, and levels with a bad tier", () => {
    const r = normalizeClassRecommendations({
      articles: [article({ slug: "" }), article({ levels: [{ tier: 9 }, { tier: 1, name: "入门" }] })],
      tier: 2,
    });
    expect(r.articles).toHaveLength(1);
    expect(r.articles[0]!.levels.map((l) => l.tier)).toEqual([1]);
  });
  it("returns an empty list for a malformed body", () => {
    expect(normalizeClassRecommendations(null)).toEqual({ articles: [], tier: null });
    expect(normalizeClassRecommendations({ articles: "x" })).toEqual({ articles: [], tier: null });
  });
});
