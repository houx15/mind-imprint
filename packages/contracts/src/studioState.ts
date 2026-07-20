import { z } from "zod";
import { Anchor } from "./anchor";

// Slice 5b wire DTO: the server-side StudioState projection (read path). Lean by
// design — the rich center-pane view types (material/structure/writing/review)
// stay in the frontend until their slice (6–9) makes them live.

export const StationCode = z.enum(["S0", "S1", "S2", "S3", "S4", "S5", "S6"]);
export type StationCode = z.infer<typeof StationCode>;

export const StationView = z.enum(["结构", "素材", "写作", "评估", "onboarding"]);
export type StationView = z.infer<typeof StationView>;

export const StationState = z.enum(["done", "current", "locked"]);
export type StationState = z.infer<typeof StationState>;

export const Station = z.object({
  code: StationCode,
  name: z.string(),
  view: StationView,
  state: StationState,
  gate: z.object({ total: z.number().int(), passed: z.number().int() }).optional(),
  backflow: z.boolean().optional(),
});
export type Station = z.infer<typeof Station>;

// tag = CT criterion (e.g. "D5"); anchor = focus label (e.g. "论证图 · 治理决心主张").
export const CoachMessage = z.discriminatedUnion("kind", [
  z.object({ kind: z.literal("student"), body: z.string() }),
  z.object({ kind: z.literal("ai"), body: z.string(), tag: z.string().optional(), anchor: z.string().optional() }),
  z.object({ kind: z.literal("flag"), label: z.string(), body: z.string() }),
]);
export type CoachMessage = z.infer<typeof CoachMessage>;

export const EquipCard = z.object({
  id: z.string(),
  name: z.string(),
  spont: z.enum(["自发", "提示后"]),
  meth: z.string(),
  // The material this card_instance evaluates (its card_instance--evaluates-->
  // material graph edge target) — carried so the client never has to guess
  // which material an equipment-bar card is about (whole-branch review
  // finding [5]). Empty string when no such edge exists.
  materialId: z.string(),
});
export type EquipCard = z.infer<typeof EquipCard>;

export const RubricRow = z.object({
  official: z.string(),
  plain: z.string(),
  weak: z.boolean(),
});
export type RubricRow = z.infer<typeof RubricRow>;

export const OnboardingFx = z.object({
  restatePrompt: z.string(),
  rubricRows: z.array(RubricRow),
  planSteps: z.array(z.string()),
  assignmentText: z.string(),
  studentRestate: z.string(),
  studentWeakPicks: z.array(z.number().int()),
});
export type OnboardingFx = z.infer<typeof OnboardingFx>;

export const MaterialBlock = z.object({ id: z.string(), text: z.string() });
export type MaterialBlock = z.infer<typeof MaterialBlock>;

// One source in the 素材 dossier. Every field has a real producer (spec §3):
// locked/role come from the CRAAP mint (evaluated-as edge → evidence node's
// source_quality.risk_note), tier/takeaway from the student's source-log entry,
// anchors from the persisted card_instances.anchors targeting this material.
// There is deliberately NO verdict field — a 可信/存疑 judgment has no honest
// producer and would have to be fabricated.
export const MaterialSource = z.object({
  id: z.string(),
  title: z.string(),
  sourceUrl: z.string(),
  kind: z.string(),
  origin: z.string(),
  blocks: z.array(MaterialBlock),
  locked: z.boolean(),
  role: z.string(),
  tier: z.string(),
  takeaway: z.string(),
  anchors: z.array(Anchor),
  // Accumulated reading time (seconds) from the source-log entry's own
  // time_spent_s column — spec §5's ledger row binds "停留 Nm" alongside the
  // takeaway/tier. A freshly ingested source (never opened) is 0.
  timeSpentS: z.number(),
  // Mirrors source_log_entry.lateral_read (Slice 6c): true only once a
  // cross_check mint has flipped it on THIS source (the one checked, not the
  // lateral source used to check it). A fact about what happened — still no
  // credibility verdict here or anywhere else.
  lateralRead: z.boolean(),
  // True when this material is itself the lateral source some OTHER
  // cross_check cited (a "cites" edge from a cross_check node to this
  // material) — server-derived from the same graph fact
  // SurfaceCardCandidates' treadmill guard uses to keep a lateral instrument
  // from ever getting its own SIFT proposal. The client must gate the
  // 需横向阅读 chip on this instead of recomputing graph reachability itself
  // (whole-branch review finding [4]) — otherwise the chip can promise a
  // workflow the summon rule will never actually offer.
  isLateralInstrument: z.boolean(),
  // True when a SIFT card_instance targeting this material was explicitly
  // skipped. The server's summon rule (agent/classifier.go's siftSurfaced)
  // treats a skip the same as any in-flight/terminal status and never
  // re-proposes SIFT on this material again — so the 需横向阅读 chip must
  // stop showing once skipped instead of promising a workflow the coach will
  // in fact never offer again (FIX 3, whole-branch review). The skip itself
  // is still permanently recorded in the process tree; this field only keeps
  // the LIVE chip from mis-describing the state as "currently being
  // verified" once nothing is.
  siftSkipped: z.boolean(),
  // Derived from the cross_check node reached by this material's own
  // "cross-checked-by" edge (Slice 6c's SIFT mint): her own chosen relation
  // (印证/反驳/限定) and her own revised-judgment sentence, given back exactly
  // as she wrote them — never an AI verdict. "" when no cross_check exists
  // for this material yet — required (not optional), same as every other
  // field on this DTO: a missing producer must never hide behind an
  // optional key (mirrors riskNote()'s derive-never-decorate pattern).
  lateralRelation: z.string(),
  lateralJudgment: z.string(),
});
export type MaterialSource = z.infer<typeof MaterialSource>;

// ActiveCard is the one project-wide card_instance that is "proposed" or
// "active" (agent.SurfaceCardCandidates' own invariant guarantees at most
// one at a time) — projected so a page reload can rehydrate the open card
// instead of losing it while the row stays open server-side (which, after
// FIX-D's suppression, would otherwise block every future card from ever
// surfacing again). Carries exactly what conversation.ts's "card" SSE event
// carries, minus the CardSpec itself — the client already resolves that from
// CARD_REGISTRY by cardId.
export const ActiveCard = z.object({
  cardInstanceId: z.string(),
  cardId: z.string(),
  status: z.enum(["proposed", "active"]),
  anchors: z.array(Anchor),
  materialId: z.string(),
});
export type ActiveCard = z.infer<typeof ActiveCard>;

// StructureCard is one role in the S4 argument (论证构建), projected once the
// Toulmin card is locked. status/preview are derived from the minted graph
// nodes server-side (studio.projectStructure) — never invented. The array is
// empty until the argument exists, so the 结构 pane keeps its placeholder.
export const StructureCard = z.object({
  id: z.string(),
  role: z.string(),
  status: z.enum(["done", "empty"]),
  preview: z.string(),
});
export type StructureCard = z.infer<typeof StructureCard>;

export const WritingBudget = z.object({
  state: z.enum(["in", "over", "under"]),
  delta: z.number().int(),
});
export type WritingBudget = z.infer<typeof WritingBudget>;

export const WritingSnapshot = z.object({
  id: z.string(),
  seq: z.number().int(),
  committedAt: z.string(),
  wordCount: z.number().int(),
  inBand: z.boolean(),
  budget: WritingBudget,
});
export type WritingSnapshot = z.infer<typeof WritingSnapshot>;

export const WritingReviewItem = z.object({
  interventionId: z.string(),
  criterion: z.string(),
  band: z.string(),
  evidence: z.string(),
  missing: z.string(),
  fix: z.string(),
  voice: z.enum(["board", "sceptic", "layperson", "executioner"]),
  disposition: z
    .object({ action: z.enum(["accept", "reject", "rewrite"]), reason: z.string() })
    .nullable(),
});
export type WritingReviewItem = z.infer<typeof WritingReviewItem>;

export const WritingProjection = z.object({
  buffer: z.string(),
  latestSnapshot: WritingSnapshot.nullable(),
  wordBudget: z.object({ min: z.number().int(), max: z.number().int() }),
  citationsMatched: z.boolean(),
  review: z.object({ items: z.array(WritingReviewItem) }),
});
export type WritingProjection = z.infer<typeof WritingProjection>;

export const Gauge = z.object({
  code: z.string(),
  name: z.string(),
  lit: z.number().int(),
  total: z.number().int(),
  note: z.string(),
  level: z.enum(["full", "partial", "empty"]),
});
export type Gauge = z.infer<typeof Gauge>;

export const StudioProjection = z.object({
  project: z.object({ title: z.string(), qualLabel: z.string() }),
  stations: z.array(Station),
  activeStation: StationCode,
  coach: z.object({
    anchor: z.string(),
    messages: z.array(CoachMessage),
    equipment: z.array(EquipCard),
  }),
  onboarding: OnboardingFx,
  materials: z.array(MaterialSource),
  activeCard: ActiveCard.nullable(),
  structure: z.array(StructureCard),
  writing: WritingProjection,
  readiness: z.array(Gauge),
  finished: z.boolean(),
  canFinish: z.boolean(),
});
export type StudioProjection = z.infer<typeof StudioProjection>;

export const CreateProjectBody = z.object({
  title: z.string().optional(),
  prompt: z.string().min(1),
});
export type CreateProjectBody = z.infer<typeof CreateProjectBody>;

export const CreateProjectResult = z.object({ id: z.string() });
export type CreateProjectResult = z.infer<typeof CreateProjectResult>;

export const OnboardingSubmitBody = z.object({
  restate: z.string(),
  weakPicks: z.array(z.number().int()),
});
export type OnboardingSubmitBody = z.infer<typeof OnboardingSubmitBody>;
