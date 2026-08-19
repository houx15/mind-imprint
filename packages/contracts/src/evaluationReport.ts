import { z } from "zod";

export const Ref = z.object({
  id: z.string(),
  label: z.string().optional(),
  ts: z.string().optional(),
}).strict();
export type Ref = z.infer<typeof Ref>;

// Option-A rich evidence item: quote is lifted verbatim from a chat_message;
// observation/boundary are short model prose. id = message/event id (G5).
export const EvidenceItem = z.object({
  id: z.string(),
  ts: z.string().optional(),
  stage: z.string().optional(),
  quote: z.string(),
  observation: z.string(),
  boundary: z.string().optional(),
}).strict();
export type EvidenceItem = z.infer<typeof EvidenceItem>;

const Milestones = z.object({
  started: z.string().nullable(),
  frameworkFinished: z.string().nullable(),
  proposalFinished: z.string().nullable(),
  writingFinished: z.string().nullable(),
  projectFinished: z.string().nullable(),
}).strict();

const Counters = z.object({
  aiTurns: z.number().int().nonnegative(),
  materialsRead: z.number().int().nonnegative(),
  wordsWritten: z.number().int().nonnegative(),
  aiCommentCount: z.number().int().nonnegative(),
  editCount: z.number().int().nonnegative(),
}).strict();

const Basics = z.object({
  title: z.string(),
  type: z.string(),
  startDate: z.string(),
  endDate: z.string().nullable(),
  milestones: Milestones,
  counters: Counters,
}).strict();

const RecommendedCourse = z.object({ courseId: z.string(), reason: z.string() }).strict();

const Abstract = z.object({
  overview: z.string(),
  materialSentence: z.string(),
  writingSentence: z.string(),
  aiSentence: z.string(),
  suggestionParagraph: z.string(),
  // Both list fields tolerate a null/absent value (coerced to []) so one empty
  // optional field never makes EvaluationReport.parse throw and blanks the whole
  // report — a report with no recommended courses is still a valid report. The
  // Go generator also emits [] now (reportgen.go), so this is belt-and-braces.
  suggestionSentences: z.array(z.string()).nullish().transform((v) => v ?? []),
  recommendedCourses: z.array(RecommendedCourse).nullish().transform((v) => v ?? []),
}).strict();

export const EventKind = z.enum(["chat", "reading", "graph", "writing", "review", "milestone"]);
export const EventEntry = z.object({
  ts: z.string(),
  kind: EventKind,
  summary: z.string(),
  aiTurns: z.number().int().nonnegative(),
  ref: Ref.optional(),
}).strict();
export type EventEntry = z.infer<typeof EventEntry>;

// Option-A: material gains finalStatus + cannotSupport.
export const MaterialEntry = z.object({
  materialId: z.string(),
  addedAt: z.string(),
  source: z.string(),
  url: z.string().nullable(),
  usedIn: Ref.nullable(),
  finalStatus: z.string(),
  comment: z.string(),
  cannotSupport: z.string(),
}).strict();
export type MaterialEntry = z.infer<typeof MaterialEntry>;

export const DepthDimResult = z.object({
  id: z.enum(["D1", "D2", "D3", "D4", "D5", "D6"]),
  level: z.number().int().min(1).max(4),
  summary: z.string(),
  evidence: z.array(EvidenceItem),
  suggestion: z.string(),
}).strict();
export type DepthDimResult = z.infer<typeof DepthDimResult>;

export const AutonomyDimResult = z.object({
  id: z.enum(["A1", "A2", "A3", "A4", "A5", "A6"]),
  band: z.number().int().min(0).max(5),
  summary: z.string(),
  evidence: z.array(EvidenceItem),
  suggestion: z.string(),
}).strict();
export type AutonomyDimResult = z.infer<typeof AutonomyDimResult>;

export const PromptItem = z.object({
  stage: z.string(),
  quote: z.string(),
  ref: Ref,
  observation: z.string(),
  relatedDomains: z.array(z.string()),
  attention: z.boolean(),
}).strict();
export type PromptItem = z.infer<typeof PromptItem>;

export const PromptLens = z.object({
  summary: z.string(),
  prompts: z.array(PromptItem),
}).strict();
export type PromptLens = z.infer<typeof PromptLens>;

export const ToolUsageEntry = z.object({
  toolId: z.string(),
  name: z.string(),
  stage: z.string(),
  purpose: z.string(),
  summary: z.string(),
}).strict();
export type ToolUsageEntry = z.infer<typeof ToolUsageEntry>;

export const RiskType = z.enum([
  "ai-ghostwrite", "missing-source", "argument-logic", "data-scope", "rabbit-hole-offtopic",
]);
export const RiskEntry = z.object({
  type: RiskType,
  behaviour: z.string(),
  ref: Ref.optional(),
  suggestion: z.string(),
}).strict();
export type RiskEntry = z.infer<typeof RiskEntry>;

export const EvaluationReport = z.object({
  version: z.literal(1),
  reportId: z.string(),
  projectId: z.string(),
  student: z.object({ id: z.string(), name: z.string() }).strict(),
  basics: Basics,
  abstract: Abstract,
  events: z.array(EventEntry),
  materials: z.array(MaterialEntry),
  depth: z.array(DepthDimResult),
  autonomy: z.array(AutonomyDimResult),
  promptLens: PromptLens,
  toolUsage: z.array(ToolUsageEntry),
  risks: z.array(RiskEntry),
  generatedAt: z.string(),
}).strict();
export type EvaluationReport = z.infer<typeof EvaluationReport>;
