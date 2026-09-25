export interface ReferenceSection {
  name: string;
  purpose: string;
  layout: string;
  evidence: string;
}

export interface ReferenceSpec {
  sourceKind: "url" | "html" | "image";
  title: string;
  description: string;
  sections: ReferenceSection[];
  navigation: { style: string; items: string[] };
  contentSignals: { headings: string[]; links: number; images: number; words: number };
  visualStyle: {
    tone: string;
    density: string;
    contrast: string;
    palette: string[];
    aspectRatio?: string;
    bands?: string[];
  };
  interactionPatterns: string[];
  limitations: string[];
}

const clean = (value: string, max = 180) => value.replace(/\s+/g, " ").trim().slice(0, max);

export function analyzeHtml(source: string, fileName: string): ReferenceSpec {
  const doc = new DOMParser().parseFromString(source, "text/html");
  const title = clean(doc.title || fileName, 80);
  const description = clean(doc.querySelector('meta[name="description"]')?.getAttribute("content") || "", 220);
  const headings = Array.from(doc.querySelectorAll("h1,h2,h3"))
    .map((node) => clean(node.textContent || "", 60)).filter(Boolean).slice(0, 12);
  const navItems = Array.from(doc.querySelectorAll("nav a, header a"))
    .map((node) => clean(node.textContent || "", 32)).filter(Boolean).slice(0, 8);
  const sections = Array.from(doc.querySelectorAll("main > section, body > section, article > section, section"))
    .slice(0, 8).map((node, index) => {
      const heading = clean(node.querySelector("h1,h2,h3")?.textContent || "", 50);
      const classes = clean(typeof node.className === "string" ? node.className : "", 80);
      const hasGrid = /grid|card|gallery|portfolio|tiles/i.test(`${classes} ${node.innerHTML.slice(0, 800)}`);
      const hasImage = Boolean(node.querySelector("img, picture, video"));
      return {
        name: heading || `区块 ${index + 1}`,
        purpose: heading ? `围绕“${heading}”组织内容` : hasImage ? "图文内容展示" : "信息与行动内容",
        layout: hasGrid ? "网格或卡片" : hasImage ? "图文混排" : index === 0 ? "首屏主张" : "纵向内容区",
        evidence: [classes && `class=${classes}`, hasImage && "包含媒体", hasGrid && "检测到网格/卡片线索"].filter(Boolean).join("；"),
      };
    });
  const bodyText = clean(doc.body?.textContent || "", 1200);
  const wordCount = bodyText ? bodyText.split(/\s+/).length : 0;
  const styles = Array.from(doc.querySelectorAll("style"))
    .map((node) => node.textContent || "").join(" ");
  const palette = Array.from(`${styles} ${doc.body?.getAttribute("style") || ""}`.matchAll(/#[0-9a-f]{3,8}\b/gi))
    .map((match) => match[0].toUpperCase()).filter((color, index, list) => list.indexOf(color) === index).slice(0, 8);
  const classes = `${doc.body?.className || ""} ${Array.from(doc.querySelectorAll("body *"), (node) => typeof node.className === "string" ? node.className : "").join(" ")}`;
  const interactionPatterns = [
    /hover|悬停/i.test(source) && "悬停反馈",
    /sticky|fixed|position:\s*sticky/i.test(`${source} ${styles}`) && "固定或粘性导航",
    /accordion|details|折叠/i.test(source) && "折叠内容",
    /modal|dialog|弹窗/i.test(source) && "弹窗交互",
  ].filter(Boolean) as string[];
  return {
    sourceKind: "html", title, description,
    sections: sections.length ? sections : [{ name: "页面内容", purpose: "未识别到显式 section，需由 AI 根据 DOM 内容推断", layout: "纵向内容区", evidence: "未发现标准 section 标签" }],
    navigation: { style: navItems.length ? "顶部或页头链接导航" : "未识别到显式导航", items: navItems },
    contentSignals: { headings, links: doc.querySelectorAll("a").length, images: doc.querySelectorAll("img,picture,video").length, words: wordCount },
    visualStyle: {
      tone: /dark|黑|night/i.test(classes) ? "深色、沉浸" : /minimal|clean|极简/i.test(classes) ? "克制、极简" : "由页面内容和样式综合判断",
      density: wordCount > 700 || sections.length > 6 ? "信息密度较高" : "低到中等密度",
      contrast: /dark|black|#0[0-9a-f]{2,5}/i.test(`${classes} ${styles}`) ? "高对比" : "中等对比",
      palette,
    },
    interactionPatterns,
    limitations: ["HTML 分析基于 DOM、class 和内联样式；无法还原未加载的脚本运行结果"],
  };
}

export function analyzeImage(source: string): Promise<ReferenceSpec> {
  return new Promise((resolve, reject) => {
    const image = new Image();
    image.onload = () => {
      const width = image.naturalWidth; const height = image.naturalHeight;
      const canvas = document.createElement("canvas"); canvas.width = 96; canvas.height = 96;
      const context = canvas.getContext("2d", { willReadFrequently: true });
      if (!context) return reject(new Error("无法分析截图"));
      context.drawImage(image, 0, 0, 96, 96);
      const pixels = context.getImageData(0, 0, 96, 96).data;
      const buckets = new Map<string, number>(); let luminance = 0; let edges = 0;
      for (let y = 0; y < 96; y += 1) for (let x = 0; x < 96; x += 1) {
        const offset = (y * 96 + x) * 4; const r = pixels[offset] ?? 0; const g = pixels[offset + 1] ?? 0; const b = pixels[offset + 2] ?? 0;
        const color = `#${[r, g, b].map((v) => Math.round(v / 32) * 32).map((v) => Math.min(255, v).toString(16).padStart(2, "0")).join("")}`;
        buckets.set(color, (buckets.get(color) || 0) + 1); luminance += (r * 299 + g * 587 + b * 114) / 1000;
        if (x > 0) { const prev = (y * 96 + x - 1) * 4; edges += Math.abs(r - (pixels[prev] ?? 0)) + Math.abs(g - (pixels[prev + 1] ?? 0)) + Math.abs(b - (pixels[prev + 2] ?? 0)); }
      }
      const palette = [...buckets.entries()].sort((a, b) => b[1] - a[1]).slice(0, 6).map(([color]) => color.toUpperCase());
      const average = luminance / (96 * 96); const density = edges / (96 * 95);
      const bands = Array.from({ length: 8 }, (_, index) => {
        let variance = 0; const yStart = index * 12;
        for (let y = yStart; y < yStart + 12; y += 1) for (let x = 1; x < 96; x += 1) {
          const at = (y * 96 + x) * 4; const before = (y * 96 + x - 1) * 4;
          variance += Math.abs((pixels[at] ?? 0) - (pixels[before] ?? 0)) + Math.abs((pixels[at + 1] ?? 0) - (pixels[before + 1] ?? 0));
        }
        return variance;
      });
      const sectionCount = Math.max(2, bands.filter((value, index) => index === 0 || value > bands[index - 1]! * 1.35).length);
      resolve({ sourceKind: "image", title: "上传的网页截图", description: `截图 ${width}×${height}，已提取颜色、明暗、边缘密度和横向视觉分区`, sections: Array.from({ length: Math.min(6, sectionCount) }, (_, index) => ({ name: index === 0 ? "首屏视觉区" : `视觉区块 ${index + 1}`, purpose: index === 0 ? "建立首屏主张和视觉焦点" : "承载一组内容或行动", layout: density > 85 ? "高信息密度混排" : "留白分区", evidence: "由截图横向色彩与边缘变化推断" })), navigation: { style: "需结合截图顶部区域判断", items: [] }, contentSignals: { headings: [], links: 0, images: 0, words: 0 }, visualStyle: { tone: average < 105 ? "深色、沉浸" : average > 205 ? "明亮、轻盈" : "中性、平衡", density: density > 95 ? "高密度" : "低到中等密度", contrast: average < 120 || average > 190 ? "高对比" : "中等对比", palette, aspectRatio: `${width}:${height}`, bands: bands.map((value) => String(Math.round(value / 100))) }, interactionPatterns: [], limitations: ["截图分析可识别视觉结构和色彩线索，无法可靠读取图片中的文字与真实交互逻辑"] });
    };
    image.onerror = () => reject(new Error("截图格式无法识别")); image.src = source;
  });
}

export function referencePrompt(reference: { id: string; name: string; kind: string; spec?: ReferenceSpec; summary: string }): string {
  if (!reference.spec) return `${reference.name}（${reference.kind}）：${reference.summary}`;
  return `${reference.name}（${reference.kind}）的结构化参考分析：${JSON.stringify(reference.spec)}。只借鉴结构规律、视觉节奏和交互模式，不复制品牌、文字、图片或代码。`;
}
