import { describe, expect, it } from "vitest";
import { VisualDesignSpecV1 } from "@mind-imprint/contracts";
import { htmlToReferenceSpec, mergeVisionAnalysis, whiteboardToReferenceSpec } from "./referenceSpec";
import { applyPatch, manualPatch } from "./visualPatch";

const design = VisualDesignSpecV1.parse({
  schema: "visual-pbl-design-output", version: "1.0", designId: "design.test", revision: 1,
  artifactType: "webpage", title: "测试设计", brief: { topic: "主题", audience: ["学生"], goal: "展示", constraints: [] }, rationale: "测试",
  theme: { palette: { background: ["#FFFFFF"], surface: ["#F8FAFC"], text: ["#172033"], accent: ["#2563EB"] }, headingFont: "system-ui", bodyFont: "system-ui", baseFontSize: 16, spacingScale: [8, 16, 24], defaultRadius: 8 },
  pages: [{ id: "page.home", name: "主页", viewport: { width: 1000, height: 800, device: "desktop" }, background: "#FFFFFF", nodes: [{ id: "node.hero", type: "section", name: "首屏", parentId: null, order: 0, bounds: { x: 0, y: 0, width: 1000, height: 300 }, style: { background: "#FFFFFF" }, content: { text: "原文" }, interactionIds: [], author: "ai" }] }],
  interactions: [], referenceInfluence: [], assumptions: [],
});

describe("schema-driven visual workflow", () => {
  it("extracts HTML layout, visual tokens, content and interactions into ReferenceSpecV1", () => {
    const spec = htmlToReferenceSpec(`<html><head><title>作品集</title><style>body{background:#F6F0E5;font-family:Inter;font-size:18px}.hero{display:grid;gap:24px;color:#173B2C}</style></head><body><header class="hero"><h1>我的项目</h1><a href="https://example.com/work">查看作品</a></header><main><section><h2>研究过程</h2><button onclick="openPanel()">展开</button></section></main></body></html>`, "portfolio.html");
    expect(spec.schema).toBe("visual-pbl-design-input");
    expect(spec.palette.background).toContain("#F6F0E5");
    expect(spec.typography.families).toContain("Inter");
    expect(spec.pages[0]?.nodes.some((node) => node.content?.text === "我的项目")).toBe(true);
    expect(spec.interactions.some((interaction) => interaction.action === "open-url")).toBe(true);
  });

  it("keeps whiteboard coordinates and stable ids", () => {
    const spec = whiteboardToReferenceSpec({ viewport: "desktop", width: 960, height: 720, nodes: [{ id: "wire.hero", kind: "section", x: 40, y: 50, width: 600, height: 240, text: "首屏" }] });
    expect(spec?.pages[0]?.nodes[0]).toMatchObject({ id: "wire.hero", bounds: { x: 40, y: 50, width: 600, height: 240 }, author: "student" });
  });

  it("merges semantic screenshot analysis into executable reference nodes", () => {
    const base = whiteboardToReferenceSpec({ viewport: "desktop", width: 1000, height: 800, nodes: [{ id: "wire.base", kind: "section", x: 0, y: 0, width: 960, height: 720, text: "截图" }] })!;
    const merged = mergeVisionAnalysis(base, { summary: "双栏作品集", pageType: "作品集", palette: ["#F5F5F5", "#E74C3C"], fonts: ["无衬线体"], fontSizes: [32, 16], spacing: [16, 24], patterns: ["侧边栏+内容区"], components: [{ id: "sidebar", type: "sidebar", name: "侧栏", x: 0, y: 0, width: 240, height: 1000, background: "#F5F5F5", color: "#333333" }, { id: "hero", type: "hero", name: "首屏", text: "作品集", x: 240, y: 0, width: 760, height: 420, background: "#FFFFFF", color: "#333333" }] });
    expect(merged.status).toBe("parsed");
    expect(merged.pages[0]?.nodes).toHaveLength(2);
    expect(merged.reusablePatterns).toContain("侧边栏+内容区");
  });

  it("applies manual and AI operations through the same PatchSpecV1 path", () => {
    const patch = manualPatch(design, "node.hero", "改成深绿色", { operationId: "op.color", action: "set-style", nodeId: "node.hero", style: { background: "#173B2C" } });
    const next = applyPatch(design, patch);
    expect(next.revision).toBe(2);
    expect(next.pages[0]?.nodes[0]?.style.background).toBe("#173B2C");
  });
});
