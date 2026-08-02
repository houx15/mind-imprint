import { z } from "zod";

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
});

export const CoursePlayerPayload = z.object({
  slug: z.string(),
  title: z.string(),
  branch: z.string(),
  cardIds: z.array(z.string()),
  structure: CourseStructure,
  renderCache: RenderCache,
});

export const CourseProgress = z.object({
  course_slug: z.string(),
  current_ordinal: z.number().int(),
  completed_ordinals: z.array(z.number().int()),
  started_at: z.string().nullable(),
  completed_at: z.string().nullable(),
  updated_at: z.string(),
});

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
