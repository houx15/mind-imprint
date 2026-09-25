import type {
  AnnotationScope,
  DesignDirection,
  PageDocument,
  PatchProposal,
  Recommendation,
  SiteArchetype,
  SiteReference,
  WebBrief,
} from "./types";
import { attachTemplates, TEMPLATE_GUIDANCE } from "./templateLibrary";
import { referencePrompt } from "./referenceAnalysis";
import type { VisionReferenceAnalysis } from "./referenceSpec";
import {
  PatchSpecV1,
  VisualDesignSpecV1,
  type PatchSpecV1 as PatchSpec,
  type ReferenceSpecV1 as ReferenceSpec,
  type VisualDesignSpecV1 as VisualDesignSpec,
} from "@mind-imprint/contracts";

const API_BASE = (import.meta.env.VITE_WEB_DESIGN_API_BASE_URL || "http://127.0.0.1:8787").replace(/\/$/, "");
const COLOR_NAMES: Array<[RegExp, string]> = [
  [/深蓝|藏蓝/, "#1d4ed8"],
  [/浅蓝/, "#60a5fa"],
  [/蓝色|蓝/, "#2563eb"],
  [/深红/, "#991b1b"],
  [/红色|红/, "#dc2626"],
  [/绿色|绿/, "#16a34a"],
  [/黄色|黄/, "#d97706"],
  [/橙色|橙/, "#ea580c"],
  [/紫色|紫/, "#7c3aed"],
  [/粉色|粉/, "#db2777"],
  [/青色|青/, "#0891b2"],
  [/黑色|黑/, "#171717"],
  [/白色|白/, "#ffffff"],
  [/灰色|灰/, "#6b7280"],
];

interface RawRecommendation {
  brief: WebBrief;
  directions: Omit<DesignDirection, "layout">[];
  provider?: string;
  model?: string;
}

export interface VisualDesignIssue {
  path: (string | number)[];
  message: string;
  code: string;
}

export interface VisualDesignAttemptDiagnostic {
  attempt: number;
  rawOutput?: string;
  parseError?: string;
  qualityError?: string;
  schemaError?: string;
  callError?: string;
}

export interface VisualDesignDiagnostic {
  source: "gateway" | "frontend-schema";
  stage: string;
  raw?: unknown;
  canonicalized?: unknown;
  attempts: VisualDesignAttemptDiagnostic[];
  issues: VisualDesignIssue[];
}

export class VisualDesignValidationError extends Error {
  readonly diagnostic: VisualDesignDiagnostic;

  constructor(diagnostic: VisualDesignDiagnostic) {
    const first = diagnostic.issues[0];
    super(`AI 返回的设计规范不合法：${first?.path.join(".") || "root"} ${first?.message ?? "未知错误"}`);
    this.name = "VisualDesignValidationError";
    this.diagnostic = diagnostic;
  }
}

interface GatewayErrorPayload {
  code?: string;
  message?: string;
  details?: {
    stage?: string;
    attempts?: VisualDesignAttemptDiagnostic[];
  };
}

class GatewayResponseError extends Error {
  readonly payload: GatewayErrorPayload;

  constructor(status: number, payload: GatewayErrorPayload) {
    super(payload.message || `AI 请求失败（HTTP ${status}）`);
    this.name = "GatewayResponseError";
    this.payload = payload;
  }
}

async function json<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(`${API_BASE}${path}`, {
    ...init,
    headers: { "Content-Type": "application/json", ...init?.headers },
  });
  const body = (await response.json().catch(() => null)) as T | { error?: GatewayErrorPayload } | null;
  if (!response.ok) {
    const payload = body && typeof body === "object" && "error" in body ? body.error ?? {} : {};
    throw new GatewayResponseError(response.status, payload);
  }
  return body as T;
}

export async function checkGateway(): Promise<{ provider: string; model: string }> {
  return json<{ provider: string; model: string }>("/healthz");
}

export async function requestVisionAnalysis(name: string, mimeType: string, dataUrl: string): Promise<VisionReferenceAnalysis> {
  return json<VisionReferenceAnalysis>("/api/v1/web-design/analyze-reference-image", { method: "POST", body: JSON.stringify({ name, mimeType, dataUrl }) });
}

export async function requestVisualDesign(requirement: string, references: ReferenceSpec[]): Promise<VisualDesignSpec> {
  const compactReferences = references.map((reference) => {
    const source = reference.source;
    const compactSource = source.type === "html"
      ? { ...source, inlineHtml: source.inlineHtml ? `[HTML 已在本地解析，共 ${source.inlineHtml.length} 字符]` : undefined }
      : "assetRef" in source && typeof source.assetRef === "string" && source.assetRef.startsWith("data:")
        ? { ...source, assetRef: `[图片已在本地解析，原始 data URL ${source.assetRef.length} 字符]` }
        : source;
    return { ...reference, source: compactSource };
  });
  let raw: unknown;
  try {
    raw = await json<unknown>("/api/v1/web-design/generate-design", {
      method: "POST",
      body: JSON.stringify({ requirement, references: compactReferences }),
    });
  } catch (cause) {
    if (cause instanceof GatewayResponseError && cause.payload.details?.attempts?.length) {
      const attempts = cause.payload.details.attempts;
      const issues = attempts.flatMap((attempt) => ([
        attempt.callError ? { path: ["attempts", attempt.attempt], message: attempt.callError, code: "call_error" } : null,
        attempt.parseError ? { path: ["attempts", attempt.attempt, "rawOutput"], message: attempt.parseError, code: "parse_error" } : null,
        attempt.schemaError ? { path: ["attempts", attempt.attempt, "schema"], message: attempt.schemaError, code: "schema_error" } : null,
        attempt.qualityError ? { path: ["attempts", attempt.attempt, "quality"], message: attempt.qualityError, code: "quality_error" } : null,
      ].filter((issue): issue is VisualDesignIssue => Boolean(issue))));
      throw new VisualDesignValidationError({
        source: "gateway",
        stage: cause.payload.details.stage || "gateway",
        raw: attempts.at(-1)?.rawOutput ?? attempts[0]?.rawOutput,
        attempts,
        issues: issues.length ? issues : [{ path: ["gateway"], message: cause.message, code: cause.payload.code || "gateway_error" }],
      });
    }
    throw cause;
  }
  let parsed = VisualDesignSpecV1.safeParse(raw);
  const canonicalized = canonicalizeVisualDesign(raw);
  if (!parsed.success) parsed = VisualDesignSpecV1.safeParse(canonicalized);
  if (!parsed.success) {
    console.error("VisualDesignSpecV1 validation failed", JSON.stringify(parsed.error.issues));
    throw new VisualDesignValidationError({
      source: "frontend-schema",
      stage: "schema-validation",
      raw,
      canonicalized,
      attempts: [],
      issues: parsed.error.issues.map((issue) => ({ path: issue.path, message: issue.message, code: issue.code })),
    });
  }
  return parsed.data;
}

function canonicalizeVisualDesign(input: unknown): unknown {
  if (!input || typeof input !== "object") return input;
  const root = input as Record<string, any>;
  const pick = (source: Record<string, any> | undefined, keys: string[]) => Object.fromEntries(keys.filter((key) => source?.[key] !== undefined).map((key) => [key, source![key]]));
  const inset = (value: unknown) => {
    if (typeof value === "number") return { top: value, right: value, bottom: value, left: value };
    if (typeof value === "string") {
      const values = Array.from(value.matchAll(/-?\d+(?:\.\d+)?/g), (match) => Math.max(0, Number(match[0])));
      if (values.length === 1) return { top: values[0], right: values[0], bottom: values[0], left: values[0] };
      if (values.length === 2) return { top: values[0], right: values[1], bottom: values[0], left: values[1] };
      if (values.length === 4) return { top: values[0], right: values[1], bottom: values[2], left: values[3] };
    }
    return value;
  };
  const pages = Array.isArray(root.pages) ? root.pages.map((page: Record<string, any>) => ({
    ...pick(page, ["id", "name", "viewport", "background"]),
    nodes: Array.isArray(page.nodes) ? page.nodes.map((node: Record<string, any>, nodeIndex: number) => {
      const sourceStyle = node.style && typeof node.style === "object" ? node.style : {};
      const directTypography = pick(sourceStyle, ["fontFamily", "fontSize", "fontWeight", "lineHeight", "letterSpacing", "textAlign", "textTransform"]);
      const typography = sourceStyle.typography ?? (Object.keys(directTypography).length ? {
        fontFamily: directTypography.fontFamily ?? "system-ui", fontSize: directTypography.fontSize ?? 16,
        fontWeight: directTypography.fontWeight ?? 400, lineHeight: directTypography.lineHeight ?? 1.5,
        ...pick(directTypography, ["letterSpacing", "textAlign", "textTransform"]),
      } : undefined);
      const layoutSource = node.layout && typeof node.layout === "object" ? node.layout : {};
      const layout = Object.keys(layoutSource).length || sourceStyle.padding !== undefined ? {
        ...pick(layoutSource, ["direction", "columns", "rows", "gap", "rowGap", "columnGap", "justify", "align", "wrap"]),
        mode: layoutSource.mode ?? "free",
        ...(layoutSource.margin !== undefined ? { margin: inset(layoutSource.margin) } : {}),
        ...(layoutSource.padding !== undefined || sourceStyle.padding !== undefined ? { padding: inset(layoutSource.padding ?? sourceStyle.padding) } : {}),
      } : undefined;
      return {
        ...pick(node, ["id", "type", "name", "parentId", "order", "bounds", "content", "interactionIds", "author", "locked", "confidence"]),
        name: node.name ?? node.id ?? `节点 ${nodeIndex + 1}`,
        parentId: node.parentId ?? null,
        order: node.order ?? nodeIndex,
        interactionIds: Array.isArray(node.interactionIds) ? node.interactionIds : [],
        author: "ai",
        ...(layout ? { layout } : {}),
        style: { ...pick(sourceStyle, ["background", "color", "opacity", "border", "borderRadius", "shadow", "objectFit", "overflow"]), ...(typography ? { typography } : {}) },
      };
    }) : [],
  })) : root.pages;
  return {
    ...pick(root, ["schema", "version", "designId", "revision", "artifactType", "title", "brief", "rationale", "theme", "interactions", "referenceInfluence", "assumptions"]),
    pages,
  };
}

export async function requestVisualPatch(instruction: string, design: VisualDesignSpec, nodeId?: string): Promise<PatchSpec> {
  const raw = await json<unknown>("/api/v1/web-design/propose-design-patch", {
    method: "POST",
    body: JSON.stringify({
      instruction,
      scope: nodeId ? { type: "node", id: nodeId } : { type: "document", id: design.designId },
      design,
    }),
  });
  const parsed = PatchSpecV1.safeParse(raw);
  if (!parsed.success) {
    console.error("PatchSpecV1 validation failed", parsed.error.issues);
    throw new Error(`AI 返回的修改规范不合法：${parsed.error.issues[0]?.message ?? "未知错误"}`);
  }
  return parsed.data;
}

export async function requestRecommendation(requirement: string, wireframeSummary?: string, references: SiteReference[] = []): Promise<Recommendation> {
  const referenceSummary = references.length
    ? `\n\n以下是用户上传的参考网站/截图结构化分析。它们是本次生成的设计约束，不是背景资料。请在每个方向的 reason 中说明实际借鉴点，并让页面结构与其产生可观察差异：\n${references.map((item) => `- ${referencePrompt(item)}`).join("\n")}\n如果参考资料与用户需求冲突，以用户需求为主；如果没有可解析内容，明确说明限制。`
    : "";
  const requestText = (wireframeSummary
    ? `${requirement}\n\n以下是用户绘制的 MindPage DSL v1。PAGE 声明目标设备；NODE 的 parent 表示所属区块；x/y/w/h 是 0–1000 归一化坐标和尺寸；type 表示 section、text、image、button、divider 或 pen；content 是用户定义的内容或语义标签。请保留层级、相对顺序和主要比例，只对字体、颜色、留白、装饰和内容表达做美化，不要机械复述 DSL，也不要用预设结构覆盖它：\n${wireframeSummary}`
    : requirement) + referenceSummary + `\n\n页面模板选择要求：${TEMPLATE_GUIDANCE}`;
  const raw = await json<RawRecommendation>("/api/v1/web-design/recommend", {
    method: "POST",
    body: JSON.stringify({ requirement: requestText }),
  });
  if (!raw.brief || !Array.isArray(raw.directions) || raw.directions.length !== 3) {
    throw new Error("AI 返回的设计方向不完整");
  }
  const archetype = inferSiteArchetype(requirement, raw.brief);
  const templateEvidence = [requirement, ...references.map(referencePrompt)].join("\n");
  return {
    ...raw,
    archetype,
    sourceRequirement: requirement,
    directions: attachTemplates(raw.directions, templateEvidence, raw.brief, archetype),
  };
}

export function inferSiteArchetype(requirement: string, brief?: WebBrief): SiteArchetype {
  const source = `${requirement} ${brief?.kind ?? ""} ${brief?.topic ?? ""}`.toLowerCase();
  if (/(个人主页|个人网站|个人介绍|个人品牌|我的主页|作品集|简历|portfolio|about me|介绍自己|创作者主页|设计师.{0,6}主页|摄影师.{0,6}主页|主页.{0,8}作品)/i.test(source)) return "personal";
  if (/(社团|团队|组织|工作室|机构|协会|company|studio|team)/i.test(source)) return "organization";
  if (/(活动|展览|招募|发布会|比赛|市集|event|campaign)/i.test(source)) return "event";
  if (/(博客|文章|知识|教程|专栏|blog|documentation|wiki)/i.test(source)) return "knowledge";
  return "project";
}

export async function requestPatch(
  instruction: string,
  scope: AnnotationScope,
  page: PageDocument,
): Promise<PatchProposal> {
  const direct = adaptUnsupportedIntent(
    { summary: "", before: "", after: "", changes: [], provider: "interaction" },
    instruction,
    scope,
    page,
    inferRequestedColor(instruction),
  );
  if (direct.changes.length > 0) {
    if (!hasEffectiveChange(direct, page)) throw new Error("这项操作与页面当前状态相同");
    return direct;
  }
  if (scope.type === "page") {
    throw new Error("整页批注当前支持更换背景色，请指定颜色或 HEX 色值");
  }

  const proposal = await json<PatchProposal>("/api/v1/web-design/propose-patch", {
    method: "POST",
    body: JSON.stringify({ instruction, scope, page }),
  });
  if (!hasEffectiveChange(proposal, page)) {
    throw new Error("AI 已理解意见，但没有返回可执行的页面变化，请换一种说法");
  }
  return proposal;
}

function adaptUnsupportedIntent(
  proposal: PatchProposal,
  instruction: string,
  scope: AnnotationScope,
  page: PageDocument,
  requestedColor: string | null,
): PatchProposal {
  const scopedComponent = scope.type === "component"
    ? page.blocks.flatMap((block) => block.items).find((item) => item.id === scope.id)
    : undefined;
  const scopedBlock = scope.type === "block"
    ? page.blocks.find((block) => block.id === scope.id)
    : page.blocks.find((block) => block.items.some((item) => item.id === scope.id));

  if (/(新增|添加|插入)/.test(instruction) && scopedBlock) {
    const kind = inferComponentKind(instruction);
    if (kind) {
      const component = createRequestedComponent(kind, instruction, proposal.after);
      return {
        ...proposal,
        summary: `在${scopedBlock.name}新增${componentLabel(kind)}`,
        before: `${scopedBlock.items.length} 个组件`,
        after: `${scopedBlock.items.length + 1} 个组件`,
        changes: [{ componentId: component.id, action: "add_component", blockId: scopedBlock.id, component }],
      };
    }
  }

  if (scopedComponent && /(删除|移除|去掉)/.test(instruction)) {
    return {
      ...proposal,
      summary: `删除${scope.label}`,
      before: componentLabel(scopedComponent.type),
      after: "组件已移除，批注历史保留",
      changes: [{ componentId: scopedComponent.id, action: "delete_component" }],
    };
  }

  if (requestedColor && scope.type === "page") {
    return {
      ...proposal,
      summary: "更换整页背景",
      before: page.background ?? "当前主题纸张色",
      after: requestedColor,
      changes: [{ componentId: "site.page", action: "set_page_background", color: requestedColor }],
    };
  }

  if (requestedColor && scopedComponent) {
    return {
      ...proposal,
      summary: `将${scope.label}改为指定颜色`,
      before: scopedComponent.color ?? "当前主题默认色",
      after: requestedColor,
      changes: [{ componentId: scope.id, action: "set_color", color: requestedColor }],
    };
  }

  if (scopedComponent && scopedBlock && /(移动|移到|放到|上移|下移|最前|最后)/.test(instruction)) {
    const targetBlock = page.blocks.find((block) => instruction.includes(block.name)) ?? scopedBlock;
    const currentIndex = scopedBlock.items.findIndex((item) => item.id === scopedComponent.id);
    const index = /(最前|顶部|开头|上移)/.test(instruction)
      ? Math.max(0, /(上移)/.test(instruction) ? currentIndex - 1 : 0)
      : /(最后|底部|末尾|下移)/.test(instruction)
        ? Math.min(targetBlock.items.length, /(下移)/.test(instruction) ? currentIndex + 2 : targetBlock.items.length)
        : targetBlock.items.length;
    return {
      ...proposal,
      summary: `移动${scope.label}`,
      before: `${scopedBlock.name}第 ${currentIndex + 1} 位`,
      after: `${targetBlock.name}第 ${Math.min(index + 1, targetBlock.items.length + 1)} 位`,
      changes: [{ componentId: scopedComponent.id, action: "move_component", blockId: targetBlock.id, index }],
    };
  }

  const requestedSpan = scopedComponent ? inferRequestedSpan(instruction, scopedComponent.span) : null;
  if (scopedComponent && requestedSpan && requestedSpan !== scopedComponent.span) {
    return {
      ...proposal,
      summary: `调整${scope.label}宽度`,
      before: `宽度 ${scopedComponent.span}/12`,
      after: `宽度 ${requestedSpan}/12`,
      changes: [{ componentId: scopedComponent.id, action: "resize", span: requestedSpan }],
    };
  }

  const currentHeight = scopedComponent ? scopedComponent.height ?? defaultHeight(scopedComponent.type) : 0;
  const requestedHeight = scopedComponent ? inferRequestedHeight(instruction, currentHeight) : null;
  if (scopedComponent && requestedHeight && requestedHeight !== currentHeight) {
    return {
      ...proposal,
      summary: `调整${scope.label}高度`,
      before: `高度 ${currentHeight}px`,
      after: `高度 ${requestedHeight}px`,
      changes: [{ componentId: scopedComponent.id, action: "resize_height", height: requestedHeight }],
    };
  }

  return proposal;
}

function inferComponentKind(instruction: string): PageDocument["blocks"][number]["items"][number]["type"] | null {
  if (/(色块|颜色块|纯色块)/.test(instruction)) return "color";
  if (/(图片|图像|照片)/.test(instruction)) return "image";
  if (/(按钮|链接)/.test(instruction)) return "button";
  if (/(卡片|列表)/.test(instruction)) return "cards";
  if (/(标题|小标题)/.test(instruction)) return "heading";
  if (/(正文|文字|说明|段落)/.test(instruction)) return "text";
  return null;
}

function componentLabel(kind: PageDocument["blocks"][number]["items"][number]["type"]): string {
  return { heading: "标题", text: "文本框", button: "按钮", image: "图片", color: "纯色块", cards: "卡片" }[kind];
}

function createRequestedComponent(
  kind: PageDocument["blocks"][number]["items"][number]["type"],
  instruction: string,
  aiText: string,
): PageDocument["blocks"][number]["items"][number] {
  const quoted = instruction.match(/[“\"]([^”\"]+)[”\"]/)?.[1];
  const defaults = {
    heading: "新的章节标题",
    text: "在这里补充项目内容。",
    button: "了解更多",
    image: "",
    color: "",
    cards: ["要点一", "要点二", "要点三"],
  };
  const spans = { heading: 8, text: 8, button: 4, image: 6, color: 6, cards: 12 };
  const fallback = kind === "image" || kind === "color" || kind === "cards" ? defaults[kind] : quoted || aiText.slice(0, 80) || defaults[kind];
  return {
    id: `site.custom.${kind}.${crypto.randomUUID().slice(0, 8)}`,
    type: kind,
    text: fallback,
    span: spans[kind],
    ...(kind === "image" || kind === "color" ? { height: 180 } : {}),
    ...(kind === "color" ? { color: "#cbd5e1" } : {}),
  };
}

function inferRequestedSpan(instruction: string, current: number): number | null {
  if (/(高度|纵向|上下|高一点|矮一点)/.test(instruction)) return null;
  if (/(通栏|全宽|占满)/.test(instruction)) return 12;
  if (/(一半|半宽)/.test(instruction)) return 6;
  if (/三分之一/.test(instruction)) return 4;
  if (/四分之一/.test(instruction)) return 3;
  if (/(放大|变大|宽一点)/.test(instruction)) return Math.min(12, current + 2);
  if (/(缩小|变小|窄一点)/.test(instruction)) return Math.max(2, current - 2);
  return null;
}

function defaultHeight(kind: PageDocument["blocks"][number]["items"][number]["type"]): number {
  return kind === "image" || kind === "color" ? 180 : kind === "cards" ? 120 : 72;
}

function inferRequestedHeight(instruction: string, current: number): number | null {
  const pixels = instruction.match(/(?:高度|高)[^\d]{0,4}(\d{2,3})\s*(?:px|像素)?/i)?.[1];
  if (pixels) return Math.max(48, Math.min(640, Number(pixels)));
  if (/(高一点|增高|纵向放大|上下拉大)/.test(instruction)) return Math.min(640, current + 48);
  if (/(矮一点|降低高度|纵向缩小|上下缩小)/.test(instruction)) return Math.max(48, current - 48);
  return null;
}

function inferRequestedColor(instruction: string): string | null {
  if (!/(颜色|色彩|背景|改成|改为|换成)/.test(instruction)) return null;
  const hex = instruction.match(/#[0-9a-fA-F]{3,8}\b/)?.[0];
  if (hex) return hex;
  return COLOR_NAMES.find(([pattern]) => pattern.test(instruction))?.[1] ?? null;
}

function hasEffectiveChange(proposal: PatchProposal, page: PageDocument): boolean {
  const components = new Map(page.blocks.flatMap((block) => block.items).map((item) => [item.id, item]));
  return proposal.changes.some((change) => {
    if (change.action === "add_component") {
      return Boolean(change.blockId && change.component && page.blocks.some((block) => block.id === change.blockId));
    }
    if (change.action === "set_page_background") return Boolean(change.color) && page.background !== change.color;
    const component = components.get(change.componentId);
    if (!component) return false;
    if (change.action === "replace_text") return Boolean(change.text) && component.text !== change.text;
    if (change.action === "resize") return Boolean(change.span) && component.span !== change.span;
    if (change.action === "resize_height") return Boolean(change.height) && component.height !== change.height;
    if (change.action === "set_color") return Boolean(change.color) && component.color !== change.color;
    if (change.action === "delete_component") return true;
    if (change.action === "move_component") {
      const sourceBlock = page.blocks.find((block) => block.items.some((item) => item.id === change.componentId));
      const targetBlock = page.blocks.find((block) => block.id === change.blockId);
      if (!sourceBlock || !targetBlock || change.index === undefined) return false;
      return sourceBlock.id !== targetBlock.id || sourceBlock.items.findIndex((item) => item.id === change.componentId) !== change.index;
    }
    return false;
  });
}

export function gatewayAddress(): string {
  return API_BASE;
}
