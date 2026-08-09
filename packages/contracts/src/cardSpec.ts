import { z } from "zod";
import { FieldPrimitive } from "./primitives";
import { PrimitiveKind } from "./interactionPrimitive";

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
  methodology: Methodology,
  fields: z.array(FieldPrimitive).min(1),
  // sentence_frames — additive, optional (#6). Fill-in-the-blank 句式 scaffolds
  // (TOEFL-style) a student can adapt into her own sentence, shown above the
  // field. Skeletons only — never finished sentences about her own thesis
  // (铁律①: 印记 never writes the deliverable). Present on sentence-building
  // writing cards; absent everywhere else.
  sentence_frames: z.array(z.string().min(1)).optional(),
});

export const Priority = z.enum(["P0", "P1", "P2"]);
export const DisclosureTier = z.enum(["tier-0", "tier-1", "tier-2"]);
export const InteractionType = z.enum([
  "步骤引导卡", "选择追问卡", "分类标注卡", "量表光谱卡",
  "角色模拟卡", "画布导图卡", "回放验证卡", "报告生成卡",
]);
export const BodyStatus = z.enum(["full", "stub"]);

// reading_lens — additive, optional. Present ONLY on the reading-room
// disciplinary lenses (category 学科透镜): the fields the pick-one-sentence
// read-together mechanic needs but the writing tool cards never carried.
// A lens answers "从什么角度看"; its method_ids point at the writing tool
// cards that answer "具体做什么" (the methods layer, surfaced later). Absent
// on every tool card — the summon/router prompts fall back to purpose/trigger
// when it is nil.
export const ReadingLens = z.object({
  family: z.enum(["reasoning", "institutions", "context"]),
  task_prompt: z.string().min(1), // "选一句…" — what the student should pick
  selection_hint: z.string().min(1), // "留意…" — how to spot such a sentence
  example_focus: z.string().min(1), // what the AI's one example should highlight
  method_ids: z.array(z.string()).optional(), // tool cards that operationalize this lens
});

export const CardSpec = z.object({
  id: z.string().min(1),
  category: z.string().min(1),
  name: z.string().min(1),
  purpose: z.string(),
  trigger_condition: z.string(),
  steps: z.array(Step).min(1),
  rubric_tags: z.array(z.string()),
  mode: z.enum(["annotation", "form"]).default("form"),
  // gallery metadata (工具卡图鉴) — additive, optional.
  // asset_id links the card to its cover-art set (T-number, e.g. "T01"); absent
  // when no design exists for the card (see docs/2026-08-01-card-asset-coverage.md).
  // example is a short worked example shown in the gallery detail; when absent the
  // web falls back to the first step's methodology.example, then methodology.how.
  asset_id: z.string().optional(),
  example: z.string().optional(),
  // routing metadata (library frontmatter) — additive, optional
  name_en: z.string().optional(),
  priority: Priority.optional(),
  disclosure_tier: DisclosureTier.optional(),
  age_band: z.array(z.string()).optional(),
  trigger_keywords: z.array(z.string()).optional(),
  // deprecated: interaction_type (retire when cards are ported in Slice 3)
  interaction_type: InteractionType.optional(),
  // interaction: first-class card interaction type (re-catalog 2026-08-09),
  // replacing the deprecated interaction_type. Absent → treated as "form".
  //   form         — schema-driven form (the default; every card today)
  //   sub-agent    — opens a guided modal sub-conversation (e.g. question-card)
  //   function     — a pre-built producer, no student form (e.g. learning-report)
  interaction: z.enum(["form", "sub-agent", "function"]).optional(),
  // placement: which studio surface offers this card (re-catalog 2026-08-09).
  // Documentation + gallery grouping; the runtime binding lists live in Go
  // (StatusRegistry decks / ReadingDeckIDs / ReadingToolkitIDs / CrossCuttingCardIDs)
  // and a test cross-checks that these tags agree with those lists.
  placement: z.enum(["status", "reading", "reading-toolkit", "cross-cutting"]).optional(),
  rubric_dims: z.array(z.string()).optional(),
  related: z.array(z.string()).optional(),
  body_status: BodyStatus.optional(),
  // C2: card format evolution — primitive binding + subject/stage/completion metadata
  primitive: PrimitiveKind.optional(),
  subject: z.array(z.string()).optional(),
  stage: z.array(z.string()).optional(),
  target_type: z.string().optional(),
  params: z.record(z.unknown()).optional(),
  completion: z.array(z.record(z.unknown())).optional(),
  graph_effects: z.array(z.record(z.unknown())).optional(),
  observe: z.array(z.object({ when: z.string(), move: z.record(z.unknown()) })).optional(),
  consolidation: z.string().optional(),
  intrusiveness_cap: z.enum(["I0", "I1", "I2", "I3", "I4"]).optional(),
  reading_lens: ReadingLens.optional(),
});

export type Methodology = z.infer<typeof Methodology>;
export type Step = z.infer<typeof Step>;
export type Priority = z.infer<typeof Priority>;
export type DisclosureTier = z.infer<typeof DisclosureTier>;
export type InteractionType = z.infer<typeof InteractionType>;
export type BodyStatus = z.infer<typeof BodyStatus>;
export type ReadingLens = z.infer<typeof ReadingLens>;
export type CardSpec = z.infer<typeof CardSpec>;
