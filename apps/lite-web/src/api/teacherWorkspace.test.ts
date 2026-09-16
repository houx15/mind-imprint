import { describe, expect, it } from "vitest";
import { normalizeChoice, normalizeChoiceArticle } from "./teacherWorkspace";

describe("normalizeChoiceArticle", () => {
  it("returns undefined when the choice carries no article", () => {
    expect(normalizeChoiceArticle(undefined)).toBeUndefined();
  });

  it("keeps slug/zhTitle/reason/coverUrl when the cover signed", () => {
    const r = normalizeChoiceArticle({
      slug: "biden-creates-climate-corps",
      zhTitle: "美国气候队",
      reason: "一项就业计划如何同时服务气候目标",
      coverUrl: "https://cdn.example/cover.webp",
    });
    expect(r).toEqual({
      slug: "biden-creates-climate-corps",
      zhTitle: "美国气候队",
      reason: "一项就业计划如何同时服务气候目标",
      coverUrl: "https://cdn.example/cover.webp",
    });
  });

  it("normalizes an empty coverUrl to absent, not an empty string", () => {
    const r = normalizeChoiceArticle({ slug: "s", zhTitle: "t", reason: "r", coverUrl: "" });
    expect(r?.coverUrl).toBeUndefined();
  });

  it("normalizes a missing coverUrl the same way as an empty one", () => {
    const r = normalizeChoiceArticle({ slug: "s", zhTitle: "t", reason: "r" });
    expect(r?.coverUrl).toBeUndefined();
  });
});

describe("normalizeChoice", () => {
  it("leaves article undefined for an option that is not about an article", () => {
    const c = normalizeChoice({ id: "b", label: "个性化阅读" });
    expect(c).toEqual({ id: "b", label: "个性化阅读", slug: undefined, article: undefined });
  });

  it("carries the article through for an option that has a slug", () => {
    const c = normalizeChoice({
      id: "a",
      label: "美国气候队",
      slug: "biden-creates-climate-corps",
      article: { slug: "biden-creates-climate-corps", zhTitle: "美国气候队", reason: "理由", coverUrl: "" },
    });
    expect(c.slug).toBe("biden-creates-climate-corps");
    expect(c.article).toEqual({
      slug: "biden-creates-climate-corps",
      zhTitle: "美国气候队",
      reason: "理由",
      coverUrl: undefined,
    });
  });
});
