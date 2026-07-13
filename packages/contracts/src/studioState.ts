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
});
export type MaterialSource = z.infer<typeof MaterialSource>;

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
});
export type StudioProjection = z.infer<typeof StudioProjection>;
