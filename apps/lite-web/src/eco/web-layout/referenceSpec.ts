import { ReferenceSpecV1, type ReferenceSpecV1 as ReferenceSpec } from "@mind-imprint/contracts";
import type { WireframeDocument, WireframeNode } from "./types";

const emptyInsets = { top: 0, right: 0, bottom: 0, left: 0 };
const clean = (value: string, max = 240) => value.replace(/\s+/g, " ").trim().slice(0, max);
const stable = (value: string, fallback: string) => {
  const normalized = value.normalize("NFKD").replace(/[^A-Za-z0-9._:-]+/g, "-").replace(/^-+|-+$/g, "").slice(0, 100);
  return normalized || fallback;
};
const unique = <T,>(items: T[]) => [...new Set(items)];
const colors = (source: string) => unique(Array.from(source.matchAll(/#[0-9a-f]{3,8}\b|rgba?\([^;{}]+\)|hsla?\([^;{}]+\)/gi), (match) => match[0])).slice(0, 20);
const numbers = (source: string, property: string) => unique(Array.from(source.matchAll(new RegExp(`${property}\\s*:\\s*(\\d+(?:\\.\\d+)?)px`, "gi")), (match) => Number(match[1]))).filter(Boolean).slice(0, 30);

function palette(found: string[]) {
  const values = found.length ? found : ["#F7F5F0", "#FFFFFF", "#1E293B", "#2563EB"];
  return {
    background: values.slice(0, 3),
    surface: values.slice(1, 5),
    text: values.filter((color) => /^#(?:0|1|2|3|4|5)/i.test(color)).slice(0, 4).concat("#1E293B").slice(0, 4),
    accent: values.slice(-4),
  };
}

function nodeStyle(element: Element, cssText: string) {
  const inline = element.getAttribute("style") || "";
  const classText = typeof element.className === "string" ? element.className : "";
  const source = `${inline} ${cssText}`;
  const background = source.match(/background(?:-color)?\s*:\s*(#[0-9a-f]{3,8}|rgba?\([^;{}]+\)|hsla?\([^;{}]+\))/i)?.[1];
  const color = source.match(/(?:^|[;{])\s*color\s*:\s*(#[0-9a-f]{3,8}|rgba?\([^;{}]+\)|hsla?\([^;{}]+\))/i)?.[1];
  const fontSize = Number(source.match(/font-size\s*:\s*(\d+(?:\.\d+)?)px/i)?.[1]);
  const fontWeight = Number(source.match(/font-weight\s*:\s*(\d{3})/i)?.[1]);
  const radius = Number(source.match(/border-radius\s*:\s*(\d+(?:\.\d+)?)px/i)?.[1]);
  return {
    ...(background ? { background } : {}),
    ...(color ? { color } : {}),
    ...(radius ? { borderRadius: radius } : {}),
    ...(fontSize ? { typography: { fontFamily: "system-ui", fontSize, fontWeight: fontWeight || 400, lineHeight: 1.5 } } : {}),
    ...(classText.includes("overflow") ? { overflow: "hidden" as const } : {}),
  };
}

function interactionFor(element: Element, nodeId: string, index: number) {
  const href = element.getAttribute("href");
  const onclick = element.getAttribute("onclick");
  if (!href && !onclick && !["BUTTON", "A", "DETAILS", "INPUT"].includes(element.tagName)) return null;
  const id = `reference.interaction.${index}`;
  if (href && /^https?:\/\//i.test(href)) return { id, sourceNodeId: nodeId, trigger: "click" as const, action: "open-url" as const, href, description: "点击后打开外部链接", confidence: 0.95 };
  return { id, sourceNodeId: nodeId, trigger: element.tagName === "INPUT" ? "focus" as const : "click" as const, action: "custom" as const, description: clean(onclick || `${element.tagName.toLowerCase()} 元素的交互行为`, 300), confidence: onclick ? 0.85 : 0.65 };
}

export function htmlToReferenceSpec(source: string, fileName: string): ReferenceSpec {
  const doc = new DOMParser().parseFromString(source, "text/html");
  const cssText = Array.from(doc.querySelectorAll("style"), (style) => style.textContent || "").join("\n");
  const pageWidth = Number(cssText.match(/(?:max-)?width\s*:\s*(\d{3,4})px/i)?.[1]) || 1440;
  const candidates = Array.from(doc.querySelectorAll("body > header, body > nav, main > section, body > section, main > article, body > footer"));
  const roots = candidates.length ? candidates.slice(0, 24) : Array.from(doc.body?.children || []).filter((element) => !["SCRIPT", "STYLE", "LINK", "META", "NOSCRIPT"].includes(element.tagName)).slice(0, 24);
  const nodes: ReferenceSpec["pages"][number]["nodes"] = [];
  const interactions: ReferenceSpec["interactions"] = [];
  let y = 0;
  roots.forEach((element, rootIndex) => {
    const tag = element.tagName.toLowerCase();
    const label = clean(element.querySelector("h1,h2,h3")?.textContent || element.getAttribute("aria-label") || element.id || `${tag} ${rootIndex + 1}`, 100);
    const explicitHeight = Number((element.getAttribute("style") || "").match(/height\s*:\s*(\d+)px/i)?.[1]);
    const height = Math.max(120, Math.min(900, explicitHeight || (tag === "header" || tag === "nav" ? 88 : element.querySelector("img,video") ? 440 : 280)));
    const rootId = `reference.node.${stable(element.id || label, `section-${rootIndex + 1}`)}.${rootIndex}`;
    nodes.push({ id: rootId, type: tag === "header" ? "header" : tag === "nav" ? "nav" : tag === "footer" ? "footer" : "section", name: label, parentId: null, order: rootIndex, bounds: { x: 0, y, width: pageWidth, height }, layout: { mode: /grid/i.test(`${element.className} ${element.getAttribute("style")}`) ? "grid" : "flex", ...( /grid/i.test(`${element.className} ${element.getAttribute("style")}`) ? { columns: 3 } : { direction: "column" as const }), gap: 24, padding: { top: 32, right: 48, bottom: 32, left: 48 } }, style: nodeStyle(element, cssText), content: { text: label }, interactionIds: [], author: "imported", confidence: 0.75 });
    const children = Array.from(element.querySelectorAll(":scope h1, :scope h2, :scope h3, :scope p, :scope img, :scope a, :scope button, :scope input")).slice(0, 14);
    children.forEach((child, childIndex) => {
      const type = child.tagName === "IMG" ? "image" : child.tagName === "BUTTON" ? "button" : child.tagName === "A" ? "link" : child.tagName === "INPUT" ? "input" : "text";
      const nodeId = `${rootId}.${type}.${childIndex}`;
      const interaction = interactionFor(child, nodeId, interactions.length);
      if (interaction) interactions.push(interaction);
      const column = childIndex % 3;
      const row = Math.floor(childIndex / 3);
      nodes.push({ id: nodeId, type, name: clean(child.textContent || child.getAttribute("alt") || child.getAttribute("placeholder") || `${type} ${childIndex + 1}`, 100), parentId: rootId, order: childIndex, bounds: { x: 48 + column * ((pageWidth - 120) / 3), y: y + 52 + row * 72, width: (pageWidth - 160) / 3, height: type === "image" ? 150 : 52, zIndex: 1 }, style: nodeStyle(child, cssText), content: { ...(type === "image" ? { assetRef: child.getAttribute("src") || "reference-image", alt: child.getAttribute("alt") || "" } : { text: clean(child.textContent || child.getAttribute("placeholder") || "", 500) }) }, interactionIds: interaction ? [interaction.id] : [], author: "imported", confidence: 0.7 });
    });
    y += height;
  });
  if (!nodes.length) nodes.push({ id: "reference.node.page-content", type: "section", name: "页面内容", parentId: null, order: 0, bounds: { x: 0, y: 0, width: pageWidth, height: 720 }, layout: { mode: "flow", direction: "column", gap: 24 }, style: {}, content: { text: clean(doc.body?.textContent || fileName, 500) }, interactionIds: [], author: "imported", confidence: 0.35 });
  const foundColors = colors(`${source} ${cssText}`);
  const families = unique(Array.from(cssText.matchAll(/font-family\s*:\s*([^;{}]+)/gi), (match) => clean(match[1] || "system-ui", 100).replace(/["']/g, ""))).slice(0, 20);
  const gaps = unique([...numbers(cssText, "gap"), ...numbers(cssText, "margin"), ...numbers(cssText, "padding")]);
  const result = {
    schema: "visual-pbl-design-input" as const, version: "1.0" as const,
    referenceId: `reference.html.${stable(fileName, "upload")}.${Date.now()}`,
    source: { id: `source.html.${stable(fileName, "upload")}`, type: "html" as const, name: fileName, inlineHtml: source.slice(0, 2_000_000) },
    status: candidates.length ? "parsed" as const : "partially-parsed" as const,
    pageType: clean(doc.querySelector('meta[property="og:type"]')?.getAttribute("content") || "网页", 100),
    summary: `“${clean(doc.title || fileName, 100)}”：解析到 ${nodes.length} 个视觉节点、${interactions.length} 个交互、${roots.length} 个主要区块。`,
    palette: palette(foundColors),
    typography: { families: families.length ? families : ["system-ui"], sizes: numbers(cssText, "font-size"), weights: unique(Array.from(cssText.matchAll(/font-weight\s*:\s*(\d{3})/gi), (match) => Number(match[1]))), hierarchy: Array.from(doc.querySelectorAll("h1,h2,h3"), (node) => clean(node.textContent || "", 120)).filter(Boolean).slice(0, 30) },
    spacing: { scale: gaps.length ? gaps : [4, 8, 16, 24, 32, 48], commonMargins: [emptyInsets], commonGaps: gaps.slice(0, 30) },
    pages: [{ id: "reference.page.home", name: clean(doc.title || fileName, 100), viewport: { width: pageWidth, height: Math.max(720, y), device: "desktop" as const }, background: foundColors[0], nodes }],
    interactions,
    reusablePatterns: roots.map((element) => `${element.tagName.toLowerCase()}：${clean(element.querySelector("h1,h2,h3")?.textContent || element.className || "内容区", 120)}`).slice(0, 100),
    protectedContent: Array.from(doc.querySelectorAll("h1,h2"), (node) => clean(node.textContent || "", 120)).filter(Boolean).slice(0, 30),
    limitations: ["静态 HTML 无法执行远程脚本；动态渲染状态与后端行为需要人工复核。"], confidence: candidates.length ? 0.82 : 0.58,
  };
  return ReferenceSpecV1.parse(result);
}

export async function imageToReferenceSpec(source: string, fileName: string, mimeType: "image/png" | "image/jpeg" | "image/webp"): Promise<ReferenceSpec> {
  const image = await new Promise<HTMLImageElement>((resolve, reject) => { const element = new Image(); element.onload = () => resolve(element); element.onerror = () => reject(new Error("图片无法读取")); element.src = source; });
  const canvas = document.createElement("canvas"); canvas.width = 120; canvas.height = 120;
  const context = canvas.getContext("2d", { willReadFrequently: true });
  if (!context) throw new Error("浏览器无法分析截图");
  context.drawImage(image, 0, 0, canvas.width, canvas.height);
  const pixels = context.getImageData(0, 0, canvas.width, canvas.height).data;
  const buckets = new Map<string, number>(); const rowEnergy = Array.from({ length: 12 }, () => 0);
  for (let y = 0; y < 120; y += 1) for (let x = 0; x < 120; x += 1) {
    const at = (y * 120 + x) * 4; const r = pixels[at] || 0; const g = pixels[at + 1] || 0; const b = pixels[at + 2] || 0;
    const hex = `#${[r, g, b].map((value) => Math.min(255, Math.round(value / 32) * 32).toString(16).padStart(2, "0")).join("")}`.toUpperCase();
    buckets.set(hex, (buckets.get(hex) || 0) + 1);
    if (x > 0) { const prev = at - 4; rowEnergy[Math.floor(y / 10)]! += Math.abs(r - (pixels[prev] || 0)) + Math.abs(g - (pixels[prev + 1] || 0)) + Math.abs(b - (pixels[prev + 2] || 0)); }
  }
  const foundColors = [...buckets.entries()].sort((a, b) => b[1] - a[1]).slice(0, 10).map(([color]) => color);
  const breaks = rowEnergy.map((energy, index) => ({ energy, index })).filter(({ energy }, index) => index === 0 || energy > rowEnergy[index - 1]! * 1.3).slice(0, 7);
  const viewportWidth = 1440; const ratio = image.naturalHeight / image.naturalWidth; const viewportHeight = Math.max(720, Math.min(6000, Math.round(viewportWidth * ratio)));
  const sectionHeight = viewportHeight / Math.max(2, breaks.length);
  const nodes = Array.from({ length: Math.max(2, breaks.length) }, (_, index) => ({ id: `reference.image.section.${index + 1}`, type: "section" as const, name: index === 0 ? "截图首屏" : `截图视觉区 ${index + 1}`, parentId: null, order: index, bounds: { x: 0, y: index * sectionHeight, width: viewportWidth, height: sectionHeight }, layout: { mode: "free" as const }, style: { background: foundColors[index % foundColors.length] || "#FFFFFF" }, content: { text: "由截图的色彩和边缘变化推断的视觉分区" }, interactionIds: [], author: "imported" as const, confidence: 0.58 }));
  return ReferenceSpecV1.parse({ schema: "visual-pbl-design-input", version: "1.0", referenceId: `reference.image.${stable(fileName, "upload")}.${Date.now()}`, source: { id: `source.image.${stable(fileName, "upload")}`, type: "webpage-screenshot", name: fileName, assetRef: `local-upload:${stable(fileName, "image")}`, mimeType }, status: "partially-parsed", pageType: "网页截图", summary: `截图 ${image.naturalWidth}×${image.naturalHeight}；提取 ${foundColors.length} 个主色和 ${nodes.length} 个视觉分区。`, palette: palette(foundColors), typography: { families: [], sizes: [], weights: [], hierarchy: [] }, spacing: { scale: [8, 16, 24, 32, 48], commonMargins: [], commonGaps: [] }, pages: [{ id: "reference.page.screenshot", name: fileName, viewport: { width: viewportWidth, height: viewportHeight, device: "desktop" }, background: foundColors[0], nodes }], interactions: [], reusablePatterns: ["截图色带分区", "视觉密度分层"], protectedContent: [], limitations: ["本地像素分析不等同于 OCR；文字语义和真实点击行为由生成 AI 结合用户说明推断。"], confidence: 0.62 });
}

export interface VisionReferenceAnalysis {
  summary?: string;
  pageType?: string;
  palette?: string[];
  fonts?: string[];
  fontSizes?: number[];
  spacing?: number[];
  patterns?: string[];
  interactions?: string[];
  components?: Array<{ id?: string; type?: string; name?: string; text?: string; x?: number; y?: number; width?: number; height?: number; background?: string; color?: string }>;
}

export function mergeVisionAnalysis(reference: ReferenceSpec, vision: VisionReferenceAnalysis): ReferenceSpec {
  const page = reference.pages[0]!;
  const typeMap: Record<string, ReferenceSpec["pages"][number]["nodes"][number]["type"]> = { hero: "section", section: "section", nav: "nav", header: "header", footer: "footer", card: "card", grid: "grid", text: "text", image: "image", button: "button", shape: "shape" };
  const components = (vision.components || []).slice(0, 30);
  const nodes = components.length ? components.map((component, index) => {
    const normalized = [component.x, component.y, component.width, component.height].every((value) => typeof value === "number" && value! >= 0 && value! <= 1000);
    const rawX = component.x! / 1000 * page.viewport.width; const rawY = component.y! / 1000 * page.viewport.height;
    const bounds = normalized ? { x: Math.min(page.viewport.width - 1, rawX), y: Math.min(page.viewport.height - 1, rawY), width: Math.max(1, Math.min(component.width! / 1000 * page.viewport.width, page.viewport.width - rawX)), height: Math.max(1, Math.min(component.height! / 1000 * page.viewport.height, page.viewport.height - rawY)), zIndex: index } : page.nodes[index % page.nodes.length]?.bounds ?? { x: 0, y: index * 180, width: page.viewport.width, height: 180 };
    return { id: `reference.vision.${stable(component.id || component.name || "node", `node-${index}`)}.${index}`, type: typeMap[(component.type || "section").toLowerCase()] || "custom" as const, name: clean(component.name || `${component.type || "视觉组件"} ${index + 1}`, 120), parentId: null, order: index, bounds, layout: { mode: "free" as const }, style: { ...(component.background ? { background: component.background } : {}), ...(component.color ? { color: component.color } : {}) }, content: { text: clean(component.text || component.name || "", 500) }, interactionIds: [], author: "ai" as const, confidence: 0.78 };
  }) : page.nodes;
  const foundPalette = (vision.palette || []).filter((color) => /^#[0-9a-f]{3,8}$/i.test(color));
  return ReferenceSpecV1.parse({ ...reference, status: "parsed", pageType: clean(vision.pageType || reference.pageType || "网页截图", 160), summary: clean(vision.summary || reference.summary, 4000), palette: foundPalette.length ? palette(foundPalette) : reference.palette, typography: { ...reference.typography, families: (vision.fonts || reference.typography.families).slice(0, 20), sizes: (vision.fontSizes || reference.typography.sizes).filter((value) => value > 0 && value <= 512).slice(0, 50) }, spacing: { ...reference.spacing, scale: (vision.spacing || reference.spacing.scale).filter((value) => value >= 0).slice(0, 30) }, pages: [{ ...page, nodes }], reusablePatterns: unique([...reference.reusablePatterns, ...(vision.patterns || [])]).slice(0, 100), limitations: ["截图中的链接目标和动态状态仍需在真实网页中复核。"], confidence: components.length ? 0.84 : 0.7 });
}

function whiteboardType(node: WireframeNode) {
  return ({ section: "section", text: "text", image: "image", button: "button", divider: "divider", pen: "shape" } as const)[node.kind];
}

export function whiteboardToReferenceSpec(document: WireframeDocument): ReferenceSpec | null {
  if (!document.nodes.length) return null;
  const scaleX = document.width / 960; const scaleY = document.height / 720;
  return ReferenceSpecV1.parse({ schema: "visual-pbl-design-input", version: "1.0", referenceId: "reference.whiteboard.current", source: { id: "source.whiteboard.current", type: "whiteboard", name: "用户白板", format: "structured" }, status: "parsed", pageType: "用户线框图", summary: `用户绘制了 ${document.nodes.length} 个元素；坐标、尺寸和顺序应作为生成约束。`, palette: palette([]), typography: { families: ["system-ui"], sizes: [16, 24, 48], weights: [400, 600, 700], hierarchy: document.nodes.filter((node) => node.text).map((node) => node.text!).slice(0, 30) }, spacing: { scale: [8, 16, 24, 32, 48], commonMargins: [], commonGaps: [] }, pages: [{ id: "reference.page.whiteboard", name: "白板页面", viewport: { width: document.width, height: document.height, device: document.viewport === "phone" ? "mobile" : "desktop" }, background: "#FFFFFF", nodes: document.nodes.map((node, index) => ({ id: stable(node.id, `wire-${index}`), type: whiteboardType(node), name: node.text || `${node.kind} ${index + 1}`, parentId: null, order: index, bounds: { x: Math.max(0, node.x * scaleX), y: Math.max(0, node.y * scaleY), width: Math.max(1, node.width * scaleX), height: Math.max(1, node.height * scaleY), zIndex: index }, layout: node.kind === "section" ? { mode: "free" } : undefined, style: {}, content: node.kind === "image" ? { assetRef: "whiteboard-placeholder", alt: node.text || "图片占位" } : { text: node.text || node.kind }, interactionIds: [], author: "student", confidence: 1 })) }], interactions: [], reusablePatterns: ["用户白板空间结构"], protectedContent: document.nodes.map((node) => node.text).filter((text): text is string => Boolean(text)), limitations: [], confidence: 1 });
}
