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

// teaching — additive, optional. The student-facing "how to use this card"
// content shown in the 图鉴 / 课程 detail modal's 介绍 tab. Distinct from the
// per-step `methodology` (which the card RUNTIME uses to coach a live task):
// `teaching` is written to TEACH the method itself, plainly, before a student
// ever runs the card. When present it REPLACES the old why/how/when dump in the
// modal; when absent the modal falls back to steps[0].methodology.
// Every field but `tagline` is optional — an author includes only what fits the
// card (a 口诀 only where one naturally exists, a flow only for staged methods).
export const TeachingStep = z.object({
  // Short imperative title, optionally prefixed with a step marker (①/②/…).
  title: z.string().min(1),
  // One or two plain sentences: what to actually do in this step.
  detail: z.string().min(1),
});
export const TeachingMnemonic = z.object({
  // The memorable line or acronym itself, e.g. "CRAAP" or a 口诀.
  phrase: z.string().min(1),
  // Short unpack of the phrase (what each letter/word stands for). Optional.
  gloss: z.string().optional(),
});
export const Teaching = z.object({
  // One plain sentence: what this card is / does for the student. The modal's
  // opening line, replacing the dense internal `purpose`. Required.
  tagline: z.string().min(1),
  // Optional 口诀 / acronym callout.
  mnemonic: TeachingMnemonic.optional(),
  // Optional lightweight "diagram": an ordered list of short phrases rendered as
  // arrow-connected chips (e.g. ["框定来源","五维体检","汇总结论"]).
  flow: z.array(z.string().min(1)).optional(),
  // The how-to, as ordered scaffolded steps. The core teaching content.
  steps: z.array(TeachingStep).optional(),
  // The single most common mistake / 易错点, shown as a warning callout. Optional.
  watchOut: z.string().optional(),
  // One concrete worked mini-example. Optional (may duplicate top-level example).
  example: z.string().optional(),
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
  teaching: Teaching.optional(),
});

export type TeachingStep = z.infer<typeof TeachingStep>;
export type TeachingMnemonic = z.infer<typeof TeachingMnemonic>;
export type Teaching = z.infer<typeof Teaching>;
export type Methodology = z.infer<typeof Methodology>;
export type Step = z.infer<typeof Step>;
export type Priority = z.infer<typeof Priority>;
export type DisclosureTier = z.infer<typeof DisclosureTier>;
export type InteractionType = z.infer<typeof InteractionType>;
export type BodyStatus = z.infer<typeof BodyStatus>;
export type ReadingLens = z.infer<typeof ReadingLens>;
export type CardSpec = z.infer<typeof CardSpec>;
