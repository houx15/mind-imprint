import { z } from "zod";

// DualAxisReport: the canonical assessment object over the wire. Mirrors
// apps/api/internal/studio.ReportDTO byte-for-byte (camelCase) and, minus
// generatedAt, apps/api/internal/agent.Report (the persisted shape).
//
// RL-5: the two axes never compose into a total score. Depth carries no
// subtotal; the only percentage anywhere is officialProjection.readiness.score
// (project surface only, and its note says so).

const DepthDim = z.object({
  code: z.string(),
  name: z.string(),
  level: z.enum(["L1", "L2", "L3", "L4", "NA"]),
  levelRange: z.string().optional(),
  evidence: z.string(),
  promptEvidence: z.string(),
}).strict();

const AutonomySignal = z.object({
  code: z.string(),
  name: z.string(),
  level: z.number().int().min(0).max(5),
  opportunity: z.enum(["given_taken", "given_not_taken", "not_supplied"]),
  evidence: z.string(),
  promptEvidence: z.string(),
}).strict();

const LensStat = z.object({
  label: z.string(),
  value: z.string(),
}).strict();

const Lens = z.object({
  code: z.string(),
  name: z.string(),
  level: z.number().int().min(0).max(5),
  evidence: z.string(),
}).strict();

const PromptLens = z.object({
  stats: z.array(LensStat),
  lenses: z.array(Lens),
  note: z.string(),
}).strict();

const InteractionRow = z.object({
  round: z.number().int(),
  student: z.string(),
  aiSummary: z.string(),
  signal: z.string(),
}).strict();

const NextStep = z.object({
  title: z.string(),
  task: z.string(),
}).strict();

const Guidance = z.object({
  nextSteps: z.array(NextStep),
}).strict();

// Official-standard projection — project surface only.
const OfficialComponent = z.object({
  name: z.string(),
  judgement: z.string(),
  reason: z.string(),
}).strict();

const OfficialAlignment = z.object({
  item: z.string(),
  standard: z.string(),
  performance: z.string(),
  impact: z.string(),
}).strict();

const OfficialStandardRef = z.object({
  id: z.string(),
  name: z.string(),
}).strict();

const OfficialReadiness = z.object({
  score: z.number().int().min(0).max(100),
  note: z.string(),
}).strict();

const OfficialProjection = z.object({
  standard: OfficialStandardRef,
  components: z.array(OfficialComponent),
  alignment: z.array(OfficialAlignment),
  readiness: OfficialReadiness,
}).strict();

// Work & process — project surface only, defer evidence-map (Spec D).
const WorkSample = z.object({
  title: z.string(),
  text: z.string(),
}).strict();

const ProcessMaterial = z.object({
  name: z.string(),
  status: z.string(),
  diagnosis: z.string(),
}).strict();

const WorkAndProcess = z.object({
  workSamples: z.array(WorkSample),
  processMaterials: z.array(ProcessMaterial),
}).strict();

export const DualAxisReport = z.object({
  depthAxis: z.array(DepthDim),
  autonomyAxis: z.array(AutonomySignal),
  promptLens: PromptLens,
  interactionEvidence: z.array(InteractionRow),
  narrative: z.string(),
  guidance: Guidance,
  axiom: z.string(),
  generatedAt: z.string(),
  officialProjection: OfficialProjection.optional(),
  workAndProcess: WorkAndProcess.optional(),
}).strict();
export type DualAxisReport = z.infer<typeof DualAxisReport>;
