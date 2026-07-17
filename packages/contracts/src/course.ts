import { z } from "zod";

export const CourseStepKind = z.enum(["teaching", "challenge"]);
export const CourseAssetKind = z.enum(["image", "text", "link"]);

export const CourseAsset = z.object({
  id: z.string(),
  kind: CourseAssetKind,
  title: z.string(),
  value: z.string(),
});

export const CourseStep = z.object({
  id: z.string(),
  course_id: z.string(),
  ordinal: z.number().int(),
  kind: CourseStepKind,
  purpose: z.string(),
  assets: z.array(CourseAsset),
  challenge_type: z.string().nullable(),
  authored_content: z.unknown(),
});

export const CourseSummary = z.object({
  id: z.string(),
  branch: z.string(),
  title: z.string(),
  blurb: z.string(),
  tasks_count: z.number().int(),
  tools_count: z.number().int(),
  time_label: z.string(),
  step_count: z.number().int(),
});

export const Course = CourseSummary.extend({ steps: z.array(CourseStep) });

export const CourseProgress = z.object({
  course_id: z.string(),
  current_ordinal: z.number().int(),
  completed_ordinals: z.array(z.number().int()),
  updated_at: z.string(),
});

export const RenderedStep = z.object({
  ordinal: z.number().int(),
  kind: CourseStepKind,
  template: z.enum(["teaching", "challenge"]),
  content: z.unknown(),
  source: z.enum(["generated", "authored"]),
});

export type CourseStepKind = z.infer<typeof CourseStepKind>;
export type CourseAsset = z.infer<typeof CourseAsset>;
export type CourseStep = z.infer<typeof CourseStep>;
export type CourseSummary = z.infer<typeof CourseSummary>;
export type Course = z.infer<typeof Course>;
export type CourseProgress = z.infer<typeof CourseProgress>;
export type RenderedStep = z.infer<typeof RenderedStep>;

// Slice 12 (Course policy): the session runtime layer above the page-position
// layer above. camelCase, matching the Go DTOs' JSON tags (a fresh runtime
// surface, following Chat's convention rather than this file's older
// snake_case one).
export const CourseMessage = z.object({
  id: z.string(),
  phase: z.string(),
  role: z.enum(["student", "assistant"]),
  content: z.string(),
  createdAt: z.string(),
});

// CourseCardOffer is one card offer the session has not yet dispositioned
// (status proposed or active) — carried on session load so a reload can
// restore an offer that would otherwise live only in React state (Slice 12
// whole-branch Critical-2: without this, a reload during `guided` erased the
// offer and the card_dispositioned floor could never be met again).
export const CourseCardOffer = z.object({
  cardInstanceId: z.string(),
  cardId: z.string(),
  materialId: z.string(),
});

export const CourseSession = z.object({
  id: z.string(),
  courseId: z.string(),
  phase: z.string(),
  phaseTitle: z.string(),
  status: z.enum(["active", "finished"]),
  messages: z.array(CourseMessage),
  openCards: z.array(CourseCardOffer),
});

export type CourseMessage = z.infer<typeof CourseMessage>;
export type CourseCardOffer = z.infer<typeof CourseCardOffer>;
export type CourseSession = z.infer<typeof CourseSession>;
