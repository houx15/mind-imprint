import { describe, expect, it } from "vitest";
import { asLink, deriveTitle } from "./ReadingsLanding";

/**
 * 一行字是不是一个链接 —— 判错两个方向的代价都是真的：
 * 判不出来，她粘的链接被当成一篇一句话的文章存下来（产品负责人 2026-09-22
 * 报的「粘贴链接识别不了」）；判过头，她粘的一句中文被拿去抓网页。
 */
describe("asLink", () => {
  it("带 scheme 的照旧", () => {
    expect(asLink("https://www.bbc.com/news/articles/abc")).toBe("https://www.bbc.com/news/articles/abc");
    expect(asLink("  http://example.com/a?b=1  ")).toBe("http://example.com/a?b=1");
  });

  it("从地址栏复制出来的、没有 scheme 的那一份", () => {
    expect(asLink("www.bbc.com/news/articles/abc")).toBe("https://www.bbc.com/news/articles/abc");
    expect(asLink("en.wikipedia.org/wiki/Artificial_intelligence")).toBe(
      "https://en.wikipedia.org/wiki/Artificial_intelligence",
    );
  });

  it("一段正文不是链接", () => {
    expect(asLink("")).toBeNull();
    expect(asLink("第3段。")).toBeNull();
    expect(asLink("Many high school seniors are applying to colleges.")).toBeNull();
    expect(asLink("看这个 https://example.com")).toBeNull();
    // 一个词、一个句号结尾的中文，都不该被当成域名。
    expect(asLink("人工智能")).toBeNull();
    expect(asLink("ChatGPT")).toBeNull();
  });
});

describe("deriveTitle", () => {
  it("链接不做标题", () => {
    expect(deriveTitle("https://example.com/a")).toBe("");
    expect(deriveTitle("www.bbc.com/news/abc")).toBe("");
  });

  it("正文取第一行", () => {
    expect(deriveTitle("\n  人工智能与升学申请  \n后面还有")).toBe("人工智能与升学申请");
  });
});
