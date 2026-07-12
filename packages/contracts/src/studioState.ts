import { z } from "zod";

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
});
export type StudioProjection = z.infer<typeof StudioProjection>;
