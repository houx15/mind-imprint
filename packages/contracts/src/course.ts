import { z } from "zod";

// The 7 course categories — a closed, no-emoji vocabulary stored as
// {slug, label} pairs so a stored slug is decoupled from its display label
// (renaming a label never migrates data). The generator may SUGGEST a slug but
// must never mint an 8th; the API rejects any slug outside this set.
export const COURSE_CATEGORIES = [
  { slug: "stance-value",     label: "立场与价值" },
  { slug: "source-check",     label: "信源核查" },
  { slug: "media-literacy",   label: "媒介与信息素养" },
  { slug: "self-knowledge",   label: "自我认知" },
  { slug: "data-literacy",    label: "数据素养" },
  { slug: "research-process", label: "研究流程" },
  { slug: "argument-writing", label: "论证写作" },
] as const;

export const CourseCategory = z.enum([
  "stance-value", "source-check", "media-literacy",
  "self-knowledge", "data-literacy", "research-process", "argument-writing",
]);
export type CourseCategory = z.infer<typeof CourseCategory>;

// 学科对标 — curriculum alignment, three tracks (IB / other international /
// domestic). Each an independent list of short labels.
export const CourseAlignment = z.object({
  ib: z.array(z.string()).default([]),
  otherIntl: z.array(z.string()).default([]),
  domestic: z.array(z.string()).default([]),
});
export type CourseAlignment = z.infer<typeof CourseAlignment>;

// The schema-driven course introduction (rendered deterministically on the
// detail page; filled by the generator). Mirrors docs/2026-08-19-courses.md's
// per-course structure: 导语 / 学生做什么 / 带走什么 / 学科对标 / 关键词.
export const CourseIntroduction = z.object({
  hook: z.string().default(""),
  whatYouDo: z.string().default(""),
  takeaways: z.array(z.string()).default([]),
  alignment: CourseAlignment.default({ ib: [], otherIntl: [], domestic: [] }),
  keywords: z.array(z.string()).default([]),
});
export type CourseIntroduction = z.infer<typeof CourseIntroduction>;

export const CourseAssetType = z.enum(["image", "link", "text"]);
export const CourseAsset = z.object({
  id: z.string(),
  type: CourseAssetType,
  title: z.string().default(""),
  src: z.string().default(""),
  ossKey: z.string().optional().default(""),
  note: z.string().optional().default(""),
}).passthrough();

// Named CourseInteractionType (not InteractionType) to avoid colliding with
// cardSpec.ts's InteractionType, which index.ts re-exports via `export *`.
export const CourseInteractionType = z.enum(["single_choice", "multiple_choice", "ordering"]);
export const InteractionOption = z.object({ id: z.string(), text: z.string() });
export const Interaction: z.ZodType<any> = z.lazy(() => z.object({
  id: z.string(),
  type: CourseInteractionType,
  prompt: z.string(),
  options: z.array(InteractionOption).default([]),
  correct_answer: z.array(z.string()).default([]),
  explanation: z.string().default(""),
  remediation_questions: z.array(Interaction).default([]),
}).passthrough());

export const StructureItem = z.object({ label: z.string(), text: z.string() });
export const RenderSegment = z.object({
  kind: z.enum(["teaching", "structure"]),
  flow_block_id: z.string().default(""),
  text: z.string().default(""),
  asset_ids: z.array(z.string()).default([]),
  items: z.array(StructureItem).default([]),
}).passthrough();

export const RenderStepContent = z.object({
  title: z.string().default(""),
  subtitle: z.string().default(""),
  segments: z.array(RenderSegment).default([]),
  interactions: z.array(Interaction).default([]),
  board: z.array(z.unknown()).default([]),
}).passthrough();

export const RenderCacheStep = z.object({ stepId: z.string(), content: RenderStepContent }).passthrough();
export const RenderCache = z.object({
  version: z.string().default(""),
  courseId: z.string(),
  courseTitle: z.string().default(""),
  steps: z.array(RenderCacheStep),
}).passthrough();

// Structure: the player only reads id/title/steps[].{id,title,materials} and
// asset_library for asset resolution; everything else is authoring metadata.
export const CourseStructureStep = z.object({
  id: z.string(),
  title: z.string().default(""),
  materials: z.array(CourseAsset).default([]),
}).passthrough();
export const CourseStructure = z.object({
  id: z.string(),
  title: z.string(),
  course_goal: z.string().default(""),
  teaching_thread: z.string().default(""),
  steps: z.array(CourseStructureStep),
  asset_library: z.array(CourseAsset).default([]),
}).passthrough();

export const CourseSummary = z.object({
  slug: z.string(),
  branch: z.string(),
  title: z.string(),
  blurb: z.string(),
  time_label: z.string(),
  card_ids: z.array(z.string()),
  step_count: z.number().int(),
  // Short-lived signed URL for the course's catalog cover image (resolved
  // server-side via resolveCoverURL); absent/"" when the course has no "img:"
  // cover or OSS is off — mirrors cardCatalog.ts's own coverUrl field.
  coverUrl: z.string().optional().default(""),
  category: CourseCategory.nullable().default(null),
  introduction: CourseIntroduction.nullable().default(null),
  featuredRank: z.number().int().nullable().default(null),
});

export const CoursePlayerPayload = z.object({
  slug: z.string(),
  title: z.string(),
  branch: z.string(),
  cardIds: z.array(z.string()),
  structure: CourseStructure,
  renderCache: RenderCache,
  audioKeys: z.record(z.string()).optional().default({}),
});

export const CourseProgress = z.object({
  course_slug: z.string(),
  current_ordinal: z.number().int(),
  completed_ordinals: z.array(z.number().int()),
  started_at: z.string().nullable(),
  completed_at: z.string().nullable(),
  updated_at: z.string(),
});

// CourseReport is the finished-course summary (GET /courses/{slug}/report):
// simple stats + the tool cards the course teaches. Populated for BOTH the
// legacy course model and the CourseDefinition-2.0 runtime (the server computes
// the 2.0 stats from the course_session + definition; cards from course.card_ids).
export const CourseReport = z.object({
  title: z.string(),
  goal: z.string(),
  teaching_thread: z.string(),
  completedStepTitles: z.array(z.string()),
  cardIds: z.array(z.string()),
  secondsSpent: z.number().int(),
  quiz: z.object({ total: z.number().int(), correct: z.number().int() }),
});

export type CourseAsset = z.infer<typeof CourseAsset>;
export type Interaction = z.infer<typeof Interaction>;
export type RenderSegment = z.infer<typeof RenderSegment>;
export type RenderStepContent = z.infer<typeof RenderStepContent>;
export type RenderCache = z.infer<typeof RenderCache>;
export type CourseStructure = z.infer<typeof CourseStructure>;
export type CourseSummary = z.infer<typeof CourseSummary>;
export type CoursePlayerPayload = z.infer<typeof CoursePlayerPayload>;
export type CourseProgress = z.infer<typeof CourseProgress>;
export type CourseReport = z.infer<typeof CourseReport>;
