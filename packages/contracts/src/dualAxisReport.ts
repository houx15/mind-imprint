import { z } from "zod";
import { SoloLevel } from "./rubric";

export const PromptTier = z.enum(["P0", "P1", "P2", "P3"]);
export type PromptTier = z.infer<typeof PromptTier>;

const DepthDim = z.object({
  code: z.string(),
  name: z.string(),
  score: z.number().int().min(0).max(3),
  evidence: z.string(),
  promptEvidence: z.string(),
}).strict();

const AutonomyAxis = z.object({
  code: z.string(),
  name: z.string(),
  observation: z.string(),
  anchoredSignals: z.array(z.string()),
  promptedSignals: z.array(z.string()),
  adversaryInvites: z.number().int().min(0),
  promptEvidence: z.string(),
}).strict(); // .strict() forbids a score field — the axiom, enforced by the type

const CrossAxis = z.object({
  code: z.string(),
  name: z.string(),
  depthLevel: SoloLevel,
  initiative: z.string(),
  prose: z.string(),
  promptEvidence: z.string(),
}).strict();

const SoloRow = z.object({
  round: z.number().int(),
  excerpt: z.string(),
  level: SoloLevel,
  rationale: z.string(),
  initiative: z.string(),
}).strict();

const PromptSample = z.object({
  round: z.number().int(),
  quote: z.string(),
  annotation: z.string(),
}).strict();

const PromptLens = z.object({
  directiveRounds: z.number().int().min(0),
  totalRounds: z.number().int().min(0),
  boundarySettings: z.number().int().min(0),
  adversaryInvites: z.number().int().min(0),
  questions: z.array(z.object({ title: z.string(), body: z.string() }).strict()),
  bestPrompt: PromptSample,
  takeaway: PromptSample,
  perRound: z.array(z.object({ round: z.number().int(), tier: PromptTier, label: z.string() }).strict()),
}).strict();

const TimelineRow = z.object({
  round: z.number().int(),
  task: z.string(),
  prompt: z.string(),
  pTag: z.string(),
  dimTags: z.array(z.string()),
}).strict();

// DualAxisReport: the whole growth report over the wire. Mirrors
// apps/api/internal/studio.ReportDTO byte-for-byte (camelCase). The ONLY number
// is depthAxis.subtotal (within-axis); autonomy/cross carry no score (RL-5 + axiom).
export const DualAxisReport = z.object({
  depthAxis: z.object({
    dims: z.array(DepthDim),
    subtotal: z.number().int().min(0).max(12),
  }).strict(),
  autonomyAxis: AutonomyAxis,
  crossAxis: CrossAxis,
  solo: z.array(SoloRow),
  promptLens: PromptLens,
  timeline: z.array(TimelineRow),
  keyEvidence: z.array(z.object({ label: z.string(), quote: z.string() }).strict()),
  guidance: z.object({
    anchored: z.string(),
    prompted: z.string(),
    risk: z.string(),
    nextSteps: z.array(z.object({ title: z.string(), body: z.string() }).strict()),
  }).strict(),
  narrative: z.string(),
  axiom: z.string(),
  generatedAt: z.string(),
}).strict();
export type DualAxisReport = z.infer<typeof DualAxisReport>;
