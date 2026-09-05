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

// Which kinds of student a course is offered to (migration 0133). Today the
// dimension is the school's edition; the column is a text[] so a third value
// needs no migration, only an entry here and in course_audience.go.
//
// An EMPTY list means "no restriction" — every student sees the course. That
// direction is deliberate: an untagged course should stay visible, because a
// missed tag would otherwise remove it from every catalog with no UI saying so.
export const COURSE_AUDIENCES = [
  { slug: "lite", label: "轻量版" },
  { slug: "pro",  label: "完整版" },
] as const;

export const CourseAudience = z.enum(["lite", "pro"]);
export type CourseAudience = z.infer<typeof CourseAudience>;

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

/** One student's state on one catalog card — the ring and the pill, nothing more. */
export const CourseCatalogProgress = z.object({
  status: z.enum(["in-progress", "completed"]),
  completedSteps: z.number().int().nonnegative(),
  /** ISO-8601; the 最近学习 ordering reads this. */
  updatedAt: z.string(),
});

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
  // Which student audiences this course is offered to. [] = no restriction.
  // The catalog endpoint already filters by the caller's edition, so a student
  // never receives a row they are not allowed to open; this field is here for
  // the admin listing, where "who is this course for" has to be visible.
  audience: z.array(CourseAudience).default([]),
  // The AUTHED student's own state on this course, resolved server-side so the
  // catalog is one round trip (it used to fan out a /progress request per card).
  // null = never opened, which is what the card reads as 未开始 — a zero-valued
  // object could not say that. `status` is already normalized to the two values
  // the catalog renders, never the runtime session's five-state status.
  progress: CourseCatalogProgress.nullable().default(null),
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

// CourseAnswerReport is the per-attempt answer detail (GET
// /courses/{slug}/report/answers?attempt=<id>): the student's actual recorded
// answers + time, read back from the stored course_session blob + the course
// definition. Kept SEPARATE from CourseReport (lazy-loaded when the student
// opens the 小测/我的答案 drawer) so the main report fetch stays lean. 2.0
// courses only; a legacy course returns an empty `slices` array.
//
// One `CourseAnswerItem` per recorded interactive block:
//   - singleChoice / fillBlank → the real question (prompt) + the student's
//     answer (option label / typed text), graded correctness, attempts;
//   - interactiveHtml / video   → the completion evidence the frame reported
//     (a summarized value), correctness when the frame graded it.
// `correct` is null for ungraded blocks (survey / reflection / no grade). An
// unanswered assessment block is still listed with `answered:false` so a report
// shows what was left blank, not a silent gap.
export const CourseAnswerItem = z.object({
  blockId: z.string(),
  type: z.string(),
  prompt: z.string(),
  answered: z.boolean(),
  yourAnswer: z.string(),
  correct: z.boolean().nullable(),
  attempts: z.number().int(),
});

export const CourseAnswerSlice = z.object({
  sliceId: z.string(),
  title: z.string(),
  timeSpentSeconds: z.number().int(),
  items: z.array(CourseAnswerItem),
});

export const CourseAnswerReport = z.object({
  slices: z.array(CourseAnswerSlice),
});

export type CourseAsset = z.infer<typeof CourseAsset>;
export type Interaction = z.infer<typeof Interaction>;
export type RenderSegment = z.infer<typeof RenderSegment>;
export type RenderStepContent = z.infer<typeof RenderStepContent>;
export type RenderCache = z.infer<typeof RenderCache>;
export type CourseStructure = z.infer<typeof CourseStructure>;
export type CourseCatalogProgress = z.infer<typeof CourseCatalogProgress>;
export type CourseSummary = z.infer<typeof CourseSummary>;
export type CoursePlayerPayload = z.infer<typeof CoursePlayerPayload>;
export type CourseProgress = z.infer<typeof CourseProgress>;
export type CourseReport = z.infer<typeof CourseReport>;
export type CourseAnswerItem = z.infer<typeof CourseAnswerItem>;
export type CourseAnswerSlice = z.infer<typeof CourseAnswerSlice>;
export type CourseAnswerReport = z.infer<typeof CourseAnswerReport>;
