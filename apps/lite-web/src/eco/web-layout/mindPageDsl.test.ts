import { describe, expect, it } from "vitest";
import { pageFromMindPage, parseMindPage, serializeMindPage } from "./mindPageDsl";
import type { DesignDirection, PageDocument, WireframeDocument } from "./types";

const wireframe: WireframeDocument = {
  viewport: "desktop",
  width: 960,
  height: 720,
  nodes: [
    { id: "hero", kind: "section", x: 96, y: 72, width: 768, height: 300, text: "个人首屏" },
    { id: "headline", kind: "text", x: 130, y: 110, width: 410, height: 70, text: "你好，我是陈墨" },
    { id: "portrait", kind: "image", x: 590, y: 105, width: 220, height: 210, text: "人物照片" },
  ],
};

const direction: DesignDirection = {
  key: "story",
  name: "叙事主页",
  reason: "强调个人表达",
  headline: "AI 生成的标题",
  description: "AI 生成的介绍",
  cta: "联系我",
  layout: "essay",
};

const fallback: PageDocument = {
  layout: "essay",
  title: "默认页面",
  siteName: "陈墨",
  navItems: ["关于", "作品"],
  blocks: [],
};

describe("MindPage DSL", () => {
  it("round-trips normalized coordinates, content and parent relationships", () => {
    const parsed = parseMindPage(serializeMindPage(wireframe));
    expect(parsed.viewport).toBe("desktop");
    expect(parsed.elements.find((element) => element.id === "headline")).toMatchObject({
      parent: "hero",
      x: 135,
      content: "你好，我是陈墨",
    });
  });

  it("uses DSL structure while preserving the chosen visual layout", () => {
    const page = pageFromMindPage(serializeMindPage(wireframe), direction, fallback);
    const firstBlock = page.blocks[0];
    expect(firstBlock).toBeDefined();
    if (!firstBlock) throw new Error("expected generated block");
    expect(page.layout).toBe("essay");
    expect(page.blocks).toHaveLength(1);
    expect(firstBlock).toMatchObject({ id: "hero", name: "个人首屏" });
    expect(firstBlock.items.map((item) => item.id)).toEqual(["headline", "portrait"]);
    expect(firstBlock.items[0]).toMatchObject({ type: "heading", text: "你好，我是陈墨", span: 6 });
    expect(firstBlock.items[1]).toMatchObject({ type: "image", span: 3 });
  });
});
