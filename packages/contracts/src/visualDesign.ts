import { z } from "zod";
import { Author } from "./interactionPrimitive";

/**
 * 视觉 PBL 设计协议 v1。
 *
 * 三个文档共享 VisualNode/VisualLayout/VisualStyle/VisualInteraction，保证
 * 参考资料解析、AI 设计输出和画布局部修改使用同一种语言。坐标一律相对
 * page.viewport 的左上角，以 CSS px 表示；响应式适配由 breakpoint 覆盖。
 */

export const VISUAL_PBL_SCHEMA_VERSION = "1.0" as const;

export const StableVisualId = z
  .string()
  .min(1)
  .max(160)
  .regex(/^[A-Za-z0-9][A-Za-z0-9._:-]*$/, "id must be stable and contain only A-Z, a-z, 0-9, dot, underscore, colon or hyphen");

const finite = z.number().finite();
const nonNegative = finite.min(0);
const positive = finite.positive();
const cssColor = z.string().min(1).max(80).regex(
  /^(#[0-9a-f]{3,8}|rgba?\([^;{}]+\)|hsla?\([^;{}]+\)|transparent|currentColor)$/i,
  "color must be a hex, rgb(a), hsl(a), transparent or currentColor value",
);
const safeCssText = (max: number) => z.string().min(1).max(max).refine((v) => !/[;{}]/.test(v), "CSS text must not contain declarations");

export const VisualBoundsV1 = z.object({
  x: nonNegative,
  y: nonNegative,
  width: positive,
  height: positive,
  rotation: finite.min(-360).max(360).optional(),
  zIndex: z.number().int().min(-1000).max(1000).optional(),
}).strict();

export const VisualEdgeInsetsV1 = z.object({
  top: nonNegative,
  right: nonNegative,
  bottom: nonNegative,
  left: nonNegative,
}).strict();

export const VisualLayoutV1 = z.object({
  mode: z.enum(["free", "flow", "flex", "grid"]),
  direction: z.enum(["row", "column"]).optional(),
  columns: z.number().int().min(1).max(24).optional(),
  rows: z.number().int().min(1).max(100).optional(),
  gap: nonNegative.optional(),
  rowGap: nonNegative.optional(),
  columnGap: nonNegative.optional(),
  justify: z.enum(["start", "center", "end", "space-between", "space-around", "space-evenly"]).optional(),
  align: z.enum(["start", "center", "end", "stretch", "baseline"]).optional(),
  wrap: z.boolean().optional(),
  padding: VisualEdgeInsetsV1.optional(),
  margin: VisualEdgeInsetsV1.optional(),
}).strict().superRefine((layout, ctx) => {
  if (layout.mode === "grid" && layout.columns === undefined) {
    ctx.addIssue({ code: z.ZodIssueCode.custom, path: ["columns"], message: "grid layout requires columns" });
  }
  if (layout.mode === "flex" && layout.direction === undefined) {
    ctx.addIssue({ code: z.ZodIssueCode.custom, path: ["direction"], message: "flex layout requires direction" });
  }
});

export const VisualTypographyV1 = z.object({
  fontFamily: safeCssText(160),
  fontSize: positive.max(512),
  fontWeight: z.number().int().min(100).max(1000),
  lineHeight: positive.max(10),
  letterSpacing: finite.min(-20).max(100).optional(),
  textAlign: z.enum(["left", "center", "right", "justify"]).optional(),
  textTransform: z.enum(["none", "uppercase", "lowercase", "capitalize"]).optional(),
}).strict();

export const VisualBorderV1 = z.object({
  width: nonNegative.max(64),
  style: z.enum(["none", "solid", "dashed", "dotted"]),
  color: cssColor,
}).strict();

export const VisualShadowV1 = z.object({
  x: finite,
  y: finite,
  blur: nonNegative,
  spread: finite,
  color: cssColor,
}).strict();

export const VisualStyleV1 = z.object({
  background: cssColor.optional(),
  color: cssColor.optional(),
  opacity: finite.min(0).max(1).optional(),
  border: VisualBorderV1.optional(),
  borderRadius: nonNegative.max(1000).optional(),
  shadow: VisualShadowV1.optional(),
  typography: VisualTypographyV1.optional(),
  objectFit: z.enum(["fill", "contain", "cover", "none", "scale-down"]).optional(),
  overflow: z.enum(["visible", "hidden", "auto", "scroll"]).optional(),
}).strict();

export const VisualContentV1 = z.object({
  text: z.string().max(20_000).optional(),
  assetRef: z.string().min(1).max(1000).optional(),
  alt: z.string().max(500).optional(),
  placeholder: z.string().max(500).optional(),
  data: z.record(z.unknown()).optional(),
}).strict();

export const VisualNodeTypeV1 = z.enum([
  "page", "section", "group", "nav", "header", "footer", "grid", "card",
  "text", "image", "button", "link", "input", "shape", "divider", "custom",
]);

export const VisualNodeV1 = z.object({
  id: StableVisualId,
  type: VisualNodeTypeV1,
  name: z.string().min(1).max(160),
  parentId: StableVisualId.nullable(),
  order: z.number().int().min(0),
  bounds: VisualBoundsV1,
  layout: VisualLayoutV1.optional(),
  style: VisualStyleV1,
  content: VisualContentV1.optional(),
  interactionIds: z.array(StableVisualId).default([]),
  author: Author,
  locked: z.boolean().optional(),
  confidence: finite.min(0).max(1).optional(),
}).strict();

export const VisualInteractionV1 = z.object({
  id: StableVisualId,
  sourceNodeId: StableVisualId,
  trigger: z.enum(["click", "hover", "focus", "submit", "scroll", "load", "drag", "drop"]),
  action: z.enum(["navigate", "open-url", "scroll-to", "toggle", "show", "hide", "open-modal", "submit", "custom"]),
  targetNodeId: StableVisualId.optional(),
  href: z.string().url().max(2048).optional(),
  description: z.string().min(1).max(500),
  payload: z.record(z.unknown()).optional(),
  confidence: finite.min(0).max(1).optional(),
}).strict().superRefine((interaction, ctx) => {
  if ((interaction.action === "navigate" || interaction.action === "open-url") && !interaction.href) {
    ctx.addIssue({ code: z.ZodIssueCode.custom, path: ["href"], message: `${interaction.action} requires href` });
  }
  if (["scroll-to", "toggle", "show", "hide", "open-modal"].includes(interaction.action) && !interaction.targetNodeId) {
    ctx.addIssue({ code: z.ZodIssueCode.custom, path: ["targetNodeId"], message: `${interaction.action} requires targetNodeId` });
  }
});

export const VisualViewportV1 = z.object({
  width: positive.max(20_000),
  height: positive.max(100_000),
  device: z.enum(["desktop", "tablet", "mobile", "slide", "spread", "custom"]),
}).strict();

export const VisualPageV1 = z.object({
  id: StableVisualId,
  name: z.string().min(1).max(160),
  viewport: VisualViewportV1,
  background: cssColor.optional(),
  nodes: z.array(VisualNodeV1).min(1).max(5000),
}).strict();

export const VisualPaletteV1 = z.object({
  background: z.array(cssColor).max(20),
  surface: z.array(cssColor).max(20),
  text: z.array(cssColor).max(20),
  accent: z.array(cssColor).max(20),
}).strict();

export const VisualThemeV1 = z.object({
  palette: VisualPaletteV1,
  headingFont: safeCssText(160),
  bodyFont: safeCssText(160),
  baseFontSize: positive.max(128),
  spacingScale: z.array(nonNegative).min(1).max(20),
  defaultRadius: nonNegative.max(1000),
}).strict();

function validateVisualGraph(
  pages: z.infer<typeof VisualPageV1>[],
  interactions: z.infer<typeof VisualInteractionV1>[],
  ctx: z.RefinementCtx,
) {
  const pageIds = new Set<string>();
  const nodes = new Map<string, { pageId: string; parentId: string | null; interactionIds: string[] }>();
  pages.forEach((page, pageIndex) => {
    if (pageIds.has(page.id)) ctx.addIssue({ code: z.ZodIssueCode.custom, path: ["pages", pageIndex, "id"], message: "duplicate page id" });
    pageIds.add(page.id);
    page.nodes.forEach((node, nodeIndex) => {
      if (nodes.has(node.id)) ctx.addIssue({ code: z.ZodIssueCode.custom, path: ["pages", pageIndex, "nodes", nodeIndex, "id"], message: "duplicate node id" });
      nodes.set(node.id, { pageId: page.id, parentId: node.parentId, interactionIds: node.interactionIds });
      if (node.bounds.x + node.bounds.width > page.viewport.width + 0.01 || node.bounds.y + node.bounds.height > page.viewport.height + 0.01) {
        ctx.addIssue({ code: z.ZodIssueCode.custom, path: ["pages", pageIndex, "nodes", nodeIndex, "bounds"], message: "node bounds exceed its viewport" });
      }
    });
  });
  nodes.forEach((node, id) => {
    if (node.parentId) {
      const parent = nodes.get(node.parentId);
      if (!parent || parent.pageId !== node.pageId || node.parentId === id) {
        ctx.addIssue({ code: z.ZodIssueCode.custom, message: `node ${id} has an invalid parentId` });
      }
    }
    const visited = new Set<string>([id]);
    let cursor = node.parentId;
    while (cursor) {
      if (visited.has(cursor)) {
        ctx.addIssue({ code: z.ZodIssueCode.custom, message: `node ${id} belongs to a parent cycle` });
        break;
      }
      visited.add(cursor);
      cursor = nodes.get(cursor)?.parentId ?? null;
    }
  });
  const interactionIds = new Set<string>();
  interactions.forEach((interaction, index) => {
    if (interactionIds.has(interaction.id)) ctx.addIssue({ code: z.ZodIssueCode.custom, path: ["interactions", index, "id"], message: "duplicate interaction id" });
    interactionIds.add(interaction.id);
    if (!nodes.has(interaction.sourceNodeId)) ctx.addIssue({ code: z.ZodIssueCode.custom, path: ["interactions", index, "sourceNodeId"], message: "interaction source node does not exist" });
    if (interaction.targetNodeId && !nodes.has(interaction.targetNodeId)) ctx.addIssue({ code: z.ZodIssueCode.custom, path: ["interactions", index, "targetNodeId"], message: "interaction target node does not exist" });
  });
  nodes.forEach((node, id) => node.interactionIds.forEach((interactionId) => {
    if (!interactionIds.has(interactionId)) ctx.addIssue({ code: z.ZodIssueCode.custom, message: `node ${id} references missing interaction ${interactionId}` });
  }));
}

const ReferenceSourceBaseV1 = z.object({
  id: StableVisualId,
  name: z.string().min(1).max(300),
  sha256: z.string().regex(/^[a-f0-9]{64}$/i).optional(),
}).strict();

export const ReferenceSourceV1 = z.discriminatedUnion("type", [
  ReferenceSourceBaseV1.extend({ type: z.literal("web-url"), url: z.string().url().max(2048) }).strict(),
  ReferenceSourceBaseV1.extend({
    type: z.literal("html"),
    assetRef: z.string().min(1).max(1000).optional(),
    inlineHtml: z.string().min(1).max(2_000_000).optional(),
    baseUrl: z.string().url().max(2048).optional(),
  }).strict(),
  ReferenceSourceBaseV1.extend({
    type: z.literal("webpage-screenshot"),
    assetRef: z.string().min(1).max(1000),
    mimeType: z.enum(["image/png", "image/jpeg", "image/webp"]),
  }).strict(),
  ReferenceSourceBaseV1.extend({
    type: z.literal("whiteboard"),
    assetRef: z.string().min(1).max(1000).optional(),
    format: z.enum(["structured", "raster", "mixed"]),
  }).strict(),
  ReferenceSourceBaseV1.extend({
    type: z.literal("reference-image"),
    assetRef: z.string().min(1).max(1000),
    mimeType: z.enum(["image/png", "image/jpeg", "image/webp", "image/svg+xml"]),
  }).strict(),
]).superRefine((source, ctx) => {
  if (source.type === "html" && !source.assetRef && !source.inlineHtml) {
    ctx.addIssue({ code: z.ZodIssueCode.custom, path: ["assetRef"], message: "html source requires assetRef or inlineHtml" });
  }
});

export const VisualPblDesignInputV1 = z.object({
  schema: z.literal("visual-pbl-design-input"),
  version: z.literal(VISUAL_PBL_SCHEMA_VERSION),
  referenceId: StableVisualId,
  source: ReferenceSourceV1,
  status: z.enum(["parsed", "partially-parsed", "needs-review"]),
  pageType: z.string().min(1).max(160).optional(),
  summary: z.string().min(1).max(4000),
  palette: VisualPaletteV1,
  typography: z.object({
    families: z.array(safeCssText(160)).max(20),
    sizes: z.array(positive.max(512)).max(50),
    weights: z.array(z.number().int().min(100).max(1000)).max(20),
    hierarchy: z.array(z.string().min(1).max(300)).max(30),
  }).strict(),
  spacing: z.object({
    scale: z.array(nonNegative).max(30),
    commonMargins: z.array(VisualEdgeInsetsV1).max(20),
    commonGaps: z.array(nonNegative).max(30),
  }).strict(),
  pages: z.array(VisualPageV1).min(1).max(500),
  interactions: z.array(VisualInteractionV1).max(5000),
  reusablePatterns: z.array(z.string().min(1).max(500)).max(100),
  protectedContent: z.array(z.string().min(1).max(500)).max(100),
  limitations: z.array(z.string().min(1).max(500)).max(100),
  confidence: finite.min(0).max(1),
}).strict().superRefine((document, ctx) => validateVisualGraph(document.pages, document.interactions, ctx));

export const VisualReferenceInfluenceV1 = z.object({
  referenceId: StableVisualId,
  borrowedPatterns: z.array(z.string().min(1).max(500)).max(100),
  rejectedPatterns: z.array(z.string().min(1).max(500)).max(100),
  affectedNodeIds: z.array(StableVisualId).max(500),
}).strict();

export const VisualPblDesignOutputV1 = z.object({
  schema: z.literal("visual-pbl-design-output"),
  version: z.literal(VISUAL_PBL_SCHEMA_VERSION),
  designId: StableVisualId,
  revision: z.number().int().min(1),
  artifactType: z.enum(["webpage", "presentation", "picture-book", "poster", "visual-report"]),
  title: z.string().min(1).max(300),
  brief: z.object({
    topic: z.string().min(1).max(500),
    audience: z.array(z.string().min(1).max(200)).max(30),
    goal: z.string().min(1).max(1000),
    constraints: z.array(z.string().min(1).max(500)).max(100),
  }).strict(),
  rationale: z.string().min(1).max(5000),
  theme: VisualThemeV1,
  pages: z.array(VisualPageV1).min(1).max(500),
  interactions: z.array(VisualInteractionV1).max(5000),
  referenceInfluence: z.array(VisualReferenceInfluenceV1).max(100),
  assumptions: z.array(z.string().min(1).max(500)).max(100),
}).strict().superRefine((document, ctx) => {
  validateVisualGraph(document.pages, document.interactions, ctx);
  const nodeIds = new Set(document.pages.flatMap((page) => page.nodes.map((node) => node.id)));
  document.referenceInfluence.forEach((influence, index) => influence.affectedNodeIds.forEach((id) => {
    if (!nodeIds.has(id)) ctx.addIssue({ code: z.ZodIssueCode.custom, path: ["referenceInfluence", index, "affectedNodeIds"], message: `affected node ${id} does not exist` });
  }));
});

const VisualBoundsPatchV1 = z.object({
  x: nonNegative.optional(), y: nonNegative.optional(), width: positive.optional(), height: positive.optional(),
  rotation: finite.min(-360).max(360).optional(), zIndex: z.number().int().min(-1000).max(1000).optional(),
}).strict().refine((v) => Object.keys(v).length > 0, "bounds patch cannot be empty");

const VisualStylePatchV1 = VisualStyleV1.partial().strict().refine((v) => Object.keys(v).length > 0, "style patch cannot be empty");
const VisualThemePatchV1 = VisualThemeV1.partial().strict().refine((v) => Object.keys(v).length > 0, "theme patch cannot be empty");

const operationBase = { operationId: StableVisualId };
export const VisualPatchOperationV1 = z.discriminatedUnion("action", [
  z.object({ ...operationBase, action: z.literal("add-node"), pageId: StableVisualId, parentId: StableVisualId.nullable(), index: z.number().int().min(0), node: VisualNodeV1 }).strict(),
  z.object({ ...operationBase, action: z.literal("remove-node"), nodeId: StableVisualId, expectedParentId: StableVisualId.nullable().optional() }).strict(),
  z.object({ ...operationBase, action: z.literal("move-node"), nodeId: StableVisualId, targetPageId: StableVisualId, targetParentId: StableVisualId.nullable(), index: z.number().int().min(0), position: z.object({ x: nonNegative, y: nonNegative }).strict().optional() }).strict(),
  z.object({ ...operationBase, action: z.literal("resize-node"), nodeId: StableVisualId, bounds: VisualBoundsPatchV1 }).strict(),
  z.object({ ...operationBase, action: z.literal("reorder-children"), parentId: StableVisualId.nullable(), pageId: StableVisualId, childIds: z.array(StableVisualId).min(1) }).strict(),
  z.object({ ...operationBase, action: z.literal("replace-content"), nodeId: StableVisualId, content: VisualContentV1 }).strict(),
  z.object({ ...operationBase, action: z.literal("set-style"), nodeId: StableVisualId, style: VisualStylePatchV1 }).strict(),
  z.object({ ...operationBase, action: z.literal("set-layout"), nodeId: StableVisualId, layout: VisualLayoutV1 }).strict(),
  z.object({ ...operationBase, action: z.literal("upsert-interaction"), interaction: VisualInteractionV1 }).strict(),
  z.object({ ...operationBase, action: z.literal("remove-interaction"), interactionId: StableVisualId }).strict(),
  z.object({ ...operationBase, action: z.literal("set-theme"), theme: VisualThemePatchV1 }).strict(),
]);

export const VisualPblPatchV1 = z.object({
  schema: z.literal("visual-pbl-local-patch"),
  version: z.literal(VISUAL_PBL_SCHEMA_VERSION),
  patchId: StableVisualId,
  designId: StableVisualId,
  baseRevision: z.number().int().min(1),
  scope: z.object({
    type: z.enum(["document", "page", "section", "node"]),
    id: StableVisualId,
  }).strict(),
  instruction: z.string().min(1).max(4000),
  summary: z.string().min(1).max(2000),
  operations: z.array(VisualPatchOperationV1).min(1).max(200),
  referenceIds: z.array(StableVisualId).max(100),
  author: Author,
}).strict().superRefine((patch, ctx) => {
  const seen = new Set<string>();
  patch.operations.forEach((operation, index) => {
    if (seen.has(operation.operationId)) ctx.addIssue({ code: z.ZodIssueCode.custom, path: ["operations", index, "operationId"], message: "duplicate operation id" });
    seen.add(operation.operationId);
  });
});

// English aliases keep API/model prompts concise while Chinese names remain the product-facing contract names.
export const ReferenceSpecV1 = VisualPblDesignInputV1;
export const VisualDesignSpecV1 = VisualPblDesignOutputV1;
export const PatchSpecV1 = VisualPblPatchV1;

export type StableVisualId = z.infer<typeof StableVisualId>;
export type VisualBoundsV1 = z.infer<typeof VisualBoundsV1>;
export type VisualLayoutV1 = z.infer<typeof VisualLayoutV1>;
export type VisualStyleV1 = z.infer<typeof VisualStyleV1>;
export type VisualNodeV1 = z.infer<typeof VisualNodeV1>;
export type VisualInteractionV1 = z.infer<typeof VisualInteractionV1>;
export type ReferenceSpecV1 = z.infer<typeof ReferenceSpecV1>;
export type VisualDesignSpecV1 = z.infer<typeof VisualDesignSpecV1>;
export type PatchSpecV1 = z.infer<typeof PatchSpecV1>;
