import type {
  DesignDirection,
  PageBlock,
  PageComponent,
  PageDocument,
  WireframeDocument,
  WireframeKind,
  WireframeNode,
} from "./types";

const COORDINATE_MAX = 1000;

export interface MindPageElement {
  id: string;
  type: WireframeKind;
  parent: string;
  x: number;
  y: number;
  width: number;
  height: number;
  content: string;
  points?: Array<{ x: number; y: number }>;
}

export interface MindPageDslDocument {
  language: "MINDPAGE";
  version: 1;
  viewport: "desktop" | "phone";
  elements: MindPageElement[];
}

const normalize = (value: number, total: number) => Math.max(0, Math.min(COORDINATE_MAX, Math.round(value / total * COORDINATE_MAX)));

function contains(section: WireframeNode, node: WireframeNode) {
  const centerX = node.x + node.width / 2;
  const centerY = node.y + node.height / 2;
  return centerX >= section.x && centerX <= section.x + section.width && centerY >= section.y && centerY <= section.y + section.height;
}

export function wireframeToMindPage(wireframe: WireframeDocument): MindPageDslDocument {
  const sections = wireframe.nodes.filter((node) => node.kind === "section");
  return {
    language: "MINDPAGE",
    version: 1,
    viewport: wireframe.viewport,
    elements: wireframe.nodes.map((node) => {
      const parent = node.kind === "section"
        ? "page"
        : sections
          .filter((section) => contains(section, node))
          .sort((a, b) => a.width * a.height - b.width * b.height)[0]?.id ?? "page";
      return {
        id: node.id,
        type: node.kind,
        parent,
        x: normalize(node.x, wireframe.width),
        y: normalize(node.y, wireframe.height),
        width: Math.max(1, normalize(node.width, wireframe.width)),
        height: Math.max(1, normalize(node.height, wireframe.height)),
        content: node.text ?? "",
        ...(node.points ? {
          points: node.points.map((point) => ({ x: normalize(point.x, wireframe.width), y: normalize(point.y, wireframe.height) })),
        } : {}),
      };
    }),
  };
}

export function serializeMindPage(wireframe: WireframeDocument): string {
  const document = wireframeToMindPage(wireframe);
  const lines = [
    `MINDPAGE/${document.version}`,
    `PAGE viewport=${document.viewport} coordinates=${COORDINATE_MAX}x${COORDINATE_MAX}`,
  ];
  for (const element of document.elements) {
    const points = element.points?.length
      ? ` points=${JSON.stringify(element.points.map((point) => `${point.x},${point.y}`).join(";"))}`
      : "";
    lines.push(
      `NODE id=${JSON.stringify(element.id)} type=${element.type} parent=${JSON.stringify(element.parent)} x=${element.x} y=${element.y} w=${element.width} h=${element.height} content=${JSON.stringify(element.content)}${points}`,
    );
  }
  return lines.join("\n");
}

function parseFields(line: string): Record<string, string> {
  const result: Record<string, string> = {};
  for (const match of line.matchAll(/(\w+)=("(?:\\.|[^"\\])*"|[^\s]+)/g)) {
    const key = match[1];
    const raw = match[2];
    if (!key || raw === undefined) continue;
    result[key] = raw.startsWith('"') ? JSON.parse(raw) as string : raw;
  }
  return result;
}

export function parseMindPage(source: string): MindPageDslDocument {
  const lines = source.split(/\r?\n/).map((line) => line.trim()).filter(Boolean);
  if (lines[0] !== "MINDPAGE/1") throw new Error("不支持的 MindPage DSL 版本");
  const page = parseFields(lines[1] ?? "");
  if (page.coordinates !== `${COORDINATE_MAX}x${COORDINATE_MAX}` || !page.viewport || !["desktop", "phone"].includes(page.viewport)) {
    throw new Error("MindPage DSL 缺少有效的 PAGE 声明");
  }
  const elements = lines.slice(2).map((line) => {
    if (!line.startsWith("NODE ")) throw new Error(`无法识别的 MindPage DSL 语句：${line}`);
    const field = parseFields(line);
    if (!field.id || !field.type || !field.parent) throw new Error("MindPage DSL 节点缺少 id、type 或 parent");
    const type = field.type as WireframeKind;
    if (!["section", "text", "image", "button", "divider", "pen"].includes(type)) throw new Error(`不支持的节点类型：${type}`);
    const number = (name: string) => {
      const value = Number(field[name]);
      if (!Number.isFinite(value)) throw new Error(`节点 ${field.id} 的 ${name} 不是有效坐标`);
      return Math.max(0, Math.min(COORDINATE_MAX, value));
    };
    return {
      id: field.id,
      type,
      parent: field.parent,
      x: number("x"),
      y: number("y"),
      width: Math.max(1, number("w")),
      height: Math.max(1, number("h")),
      content: field.content ?? "",
      ...(field.points ? {
        points: field.points.split(";").filter(Boolean).map((point) => {
          const [rawX, rawY] = point.split(",");
          const x = Number(rawX);
          const y = Number(rawY);
          if (!Number.isFinite(x) || !Number.isFinite(y)) throw new Error(`节点 ${field.id} 的 points 不是有效坐标`);
          return { x, y };
        }),
      } : {}),
    } satisfies MindPageElement;
  });
  return { language: "MINDPAGE", version: 1, viewport: page.viewport as "desktop" | "phone", elements };
}

function componentFromElement(element: MindPageElement, firstText: boolean, parentWidth: number, direction: DesignDirection): PageComponent {
  const generic = !element.content || ["标题或正文", "图片", "按钮", "分隔线"].includes(element.content);
  const type: PageComponent["type"] = element.type === "text"
    ? firstText || /标题|姓名|名称|主张|headline/i.test(element.content) ? "heading" : "text"
    : element.type === "divider" ? "color"
    : element.type as PageComponent["type"];
  const text = type === "heading" && generic
    ? direction.headline
    : type === "text" && generic
      ? direction.description
      : type === "button" && generic
        ? direction.cta
        : type === "image" || type === "color"
          ? ""
          : element.content;
  const span = Math.max(2, Math.min(12, Math.round(element.width / Math.max(1, parentWidth) * 12)));
  const height = Math.max(type === "color" ? 80 : 48, Math.min(560, Math.round(element.height / COORDINATE_MAX * 720)));
  return {
    id: element.id,
    type,
    text,
    span,
    height,
    ...(type === "color" ? { color: "#cbd5e1" } : {}),
  };
}

export function pageFromMindPage(source: string, direction: DesignDirection, fallback: PageDocument): PageDocument {
  const document = parseMindPage(source);
  const sections = document.elements.filter((element) => element.type === "section").sort((a, b) => a.y - b.y || a.x - b.x);
  const visible = document.elements.filter((element) => !["section", "pen"].includes(element.type));
  if (sections.length === 0 && visible.length === 0) return fallback;

  const blockFrom = (id: string, name: string, width: number, children: MindPageElement[]): PageBlock => {
    const ordered = [...children].sort((a, b) => Math.abs(a.y - b.y) < 35 ? a.x - b.x : a.y - b.y);
    let hasText = false;
    return {
      id,
      name,
      items: ordered.map((element) => {
        const firstText = element.type === "text" && !hasText;
        if (element.type === "text") hasText = true;
        return componentFromElement(element, firstText, width, direction);
      }),
    };
  };

  const blocks = sections.map((section, index) => {
    const children = visible.filter((element) => element.parent === section.id);
    const name = section.content || `页面区块 ${index + 1}`;
    if (children.length) return blockFrom(section.id, name, section.width, children);
    return {
      id: section.id,
      name,
      items: [{ id: `${section.id}.heading`, type: "heading", text: name, span: 12 }],
    } satisfies PageBlock;
  });
  const orphans = visible.filter((element) => element.parent === "page");
  if (orphans.length) blocks.unshift(blockFrom("wire.page-content", "页面内容", COORDINATE_MAX, orphans));

  return {
    ...fallback,
    title: direction.name,
    navItems: sections.length ? sections.map((section) => section.content).filter(Boolean).slice(0, 5) : fallback.navItems,
    blocks,
  };
}
