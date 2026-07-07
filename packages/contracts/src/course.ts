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

export type CourseStepKind = z.infer<typeof CourseStepKind>;
export type CourseAsset = z.infer<typeof CourseAsset>;
export type CourseStep = z.infer<typeof CourseStep>;
export type CourseSummary = z.infer<typeof CourseSummary>;
export type Course = z.infer<typeof Course>;
export type CourseProgress = z.infer<typeof CourseProgress>;
