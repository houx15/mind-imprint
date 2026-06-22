import { z } from "zod";
import { FieldPrimitive } from "./primitives";

export const Methodology = z.object({
  why: z.string().min(1),
  how: z.string().min(1),
  when: z.string().min(1),
  example: z.string().optional(),
});

export const Step = z.object({
  key: z.string().min(1),
  title: z.string().min(1),
  disclose: z.enum(["always", "on_demand"]),
  methodology_note: z.string(),
  methodology: Methodology.optional(),
  fields: z.array(FieldPrimitive).min(1),
});

export const Priority = z.enum(["P0", "P1", "P2"]);
export const DisclosureTier = z.enum(["tier-0", "tier-1", "tier-2"]);
export const InteractionType = z.enum([
  "步骤引导卡", "选择追问卡", "分类标注卡", "量表光谱卡",
  "角色模拟卡", "画布导图卡", "回放验证卡", "报告生成卡",
]);
export const BodyStatus = z.enum(["full", "stub"]);

export const CardSpec = z.object({
  id: z.string().min(1),
  category: z.string().min(1),
  name: z.string().min(1),
  purpose: z.string(),
  trigger_condition: z.string(),
  steps: z.array(Step).min(1),
  rubric_tags: z.array(z.string()),
  // routing metadata (library frontmatter) — additive, optional
  name_en: z.string().optional(),
  priority: Priority.optional(),
  disclosure_tier: DisclosureTier.optional(),
  age_band: z.array(z.string()).optional(),
  trigger_keywords: z.array(z.string()).optional(),
  interaction_type: InteractionType.optional(),
  rubric_dims: z.array(z.string()).optional(),
  related: z.array(z.string()).optional(),
  body_status: BodyStatus.optional(),
});

export type Methodology = z.infer<typeof Methodology>;
export type Step = z.infer<typeof Step>;
export type Priority = z.infer<typeof Priority>;
export type DisclosureTier = z.infer<typeof DisclosureTier>;
export type InteractionType = z.infer<typeof InteractionType>;
export type BodyStatus = z.infer<typeof BodyStatus>;
export type CardSpec = z.infer<typeof CardSpec>;
