import { describe, expect, it } from "vitest";
import {
  PatchSpecV1,
  ReferenceSourceV1,
  ReferenceSpecV1,
  VisualDesignSpecV1,
} from "../src/visualDesign";

const palette = {
  background: ["#ffffff"],
  surface: ["#f5f5f5"],
  text: ["#151515"],
  accent: ["#3159d8"],
};

const hero = {
  id: "page.home.hero",
  type: "section" as const,
  name: "首屏",
  parentId: null,
  order: 0,
  bounds: { x: 0, y: 0, width: 1440, height: 700 },
  layout: { mode: "grid" as const, columns: 2, gap: 48 },
  style: { background: "#ffffff" },
  interactionIds: [],
  author: "imported" as const,
  confidence: 0.94,
};

const button = {
  id: "page.home.hero.cta",
  type: "button" as const,
  name: "查看作品",
  parentId: hero.id,
  order: 0,
  bounds: { x: 80, y: 520, width: 160, height: 48 },
  style: {
    background: "#3159d8",
    color: "#ffffff",
    typography: { fontFamily: "Inter, sans-serif", fontSize: 16, fontWeight: 600, lineHeight: 1.5 },
  },
  content: { text: "查看作品" },
  interactionIds: ["interaction.hero.cta"],
  author: "imported" as const,
  confidence: 0.88,
};

const interaction = {
  id: "interaction.hero.cta",
  sourceNodeId: button.id,
  trigger: "click" as const,
  action: "scroll-to" as const,
  targetNodeId: hero.id,
  description: "点击后滚动到作品区",
  confidence: 0.81,
};

const page = {
  id: "page.home",
  name: "主页",
  viewport: { width: 1440, height: 2200, device: "desktop" as const },
  background: "#ffffff",
  nodes: [hero, button],
};

const reference = {
  schema: "visual-pbl-design-input" as const,
  version: "1.0" as const,
  referenceId: "reference.portfolio",
  source: { id: "source.portfolio", type: "html" as const, name: "portfolio.html", assetRef: "references/portfolio.html" },
  status: "parsed" as const,
  pageType: "个人作品集",
  summary: "大图首屏与两列作品区",
  palette,
  typography: { families: ["Inter, sans-serif"], sizes: [16, 64], weights: [400, 700], hierarchy: ["64px 首屏标题", "16px 正文"] },
  spacing: { scale: [8, 16, 24, 48], commonMargins: [{ top: 0, right: 80, bottom: 0, left: 80 }], commonGaps: [24, 48] },
  pages: [page],
  interactions: [interaction],
  reusablePatterns: ["大图首屏", "两列网格"],
  protectedContent: ["原站品牌名称和图片"],
  limitations: [],
  confidence: 0.9,
};

describe("视觉 PBL 设计输入 schema", () => {
  it("统一接受 URL、HTML、网页截图、白板和参考图片", () => {
    const sources = [
      { id: "s.url", type: "web-url", name: "站点", url: "https://example.com" },
      { id: "s.html", type: "html", name: "页面", inlineHtml: "<main></main>" },
      { id: "s.shot", type: "webpage-screenshot", name: "截图", assetRef: "a.png", mimeType: "image/png" },
      { id: "s.board", type: "whiteboard", name: "白板", format: "structured" },
      { id: "s.image", type: "reference-image", name: "参考图", assetRef: "a.webp", mimeType: "image/webp" },
    ];
    for (const source of sources) expect(ReferenceSourceV1.safeParse(source).success).toBe(true);
  });

  it("接受包含节点、字体、间距、配色和按钮逻辑的解析结果", () => {
    expect(ReferenceSpecV1.parse(reference).pages[0]?.nodes[1]?.id).toBe(button.id);
  });

  it("拒绝越出画布的节点和不存在的交互目标", () => {
    const bad = structuredClone(reference);
    bad.pages[0]!.nodes[1]!.bounds.x = 1400;
    bad.interactions[0]!.targetNodeId = "missing.node";
    const result = ReferenceSpecV1.safeParse(bad);
    expect(result.success).toBe(false);
    if (!result.success) expect(result.error.issues.length).toBeGreaterThanOrEqual(2);
  });

  it("拒绝重复稳定 ID 和父节点循环", () => {
    const duplicated = structuredClone(reference);
    duplicated.pages[0]!.nodes.push({ ...structuredClone(button), parentId: null });
    expect(ReferenceSpecV1.safeParse(duplicated).success).toBe(false);

    const cyclic = structuredClone(reference);
    cyclic.pages[0]!.nodes[0]!.parentId = button.id;
    expect(ReferenceSpecV1.safeParse(cyclic).success).toBe(false);
  });
});

describe("视觉 PBL 设计输出 schema", () => {
  it("可直接描述画布主题、页面、节点和参考影响", () => {
    const design = {
      schema: "visual-pbl-design-output",
      version: "1.0",
      designId: "design.portfolio",
      revision: 1,
      artifactType: "webpage",
      title: "个人作品集",
      brief: { topic: "个人作品集", audience: ["潜在合作方"], goal: "展示作品", constraints: [] },
      rationale: "采用参考页的大图首屏，但替换其品牌内容。",
      theme: { palette, headingFont: "Inter, sans-serif", bodyFont: "Inter, sans-serif", baseFontSize: 16, spacingScale: [8, 16, 24, 48], defaultRadius: 12 },
      pages: [{ ...page, nodes: [{ ...hero, author: "ai" }, { ...button, author: "ai" }] }],
      interactions: [{ ...interaction, confidence: undefined }],
      referenceInfluence: [{ referenceId: reference.referenceId, borrowedPatterns: ["大图首屏"], rejectedPatterns: ["原品牌内容"], affectedNodeIds: [hero.id] }],
      assumptions: ["用户希望突出代表作品"],
    };
    expect(VisualDesignSpecV1.safeParse(design).success).toBe(true);
  });
});

describe("视觉 PBL 局部修改 schema", () => {
  it("接受同一修改单中的布局、样式和尺寸操作", () => {
    const patch = {
      schema: "visual-pbl-local-patch",
      version: "1.0",
      patchId: "patch.2",
      designId: "design.portfolio",
      baseRevision: 1,
      scope: { type: "section", id: hero.id },
      instruction: "首屏改为深色并缩短高度",
      summary: "调整首屏配色和高度",
      operations: [
        { operationId: "op.style", action: "set-style", nodeId: hero.id, style: { background: "#111111", color: "#ffffff" } },
        { operationId: "op.size", action: "resize-node", nodeId: hero.id, bounds: { height: 620 } },
        { operationId: "op.layout", action: "set-layout", nodeId: hero.id, layout: { mode: "flex", direction: "row", gap: 40 } },
      ],
      referenceIds: [reference.referenceId],
      author: "ai",
    };
    expect(PatchSpecV1.safeParse(patch).success).toBe(true);
  });

  it("拒绝空操作列表和重复 operationId", () => {
    const base = {
      schema: "visual-pbl-local-patch",
      version: "1.0",
      patchId: "patch.bad",
      designId: "design.portfolio",
      baseRevision: 1,
      scope: { type: "node", id: hero.id },
      instruction: "修改",
      summary: "修改",
      referenceIds: [],
      author: "ai",
    };
    expect(PatchSpecV1.safeParse({ ...base, operations: [] }).success).toBe(false);
    expect(PatchSpecV1.safeParse({
      ...base,
      operations: [
        { operationId: "op.same", action: "resize-node", nodeId: hero.id, bounds: { width: 900 } },
        { operationId: "op.same", action: "resize-node", nodeId: hero.id, bounds: { height: 600 } },
      ],
    }).success).toBe(false);
  });
});
