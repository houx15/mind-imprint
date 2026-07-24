import { z } from "zod";
import dualAxisJson from "./dualaxis.json";

// Depth (D1–D6): SOLO-style anchors L1–L4. D6 (reflection) additionally
// carries reflectionRule — no self-written reflection ⇒ NA, never a low score.
const DepthDimSchema = z.object({
  id: z.string(),
  name: z.string(),
  means: z.string(),
  anchors: z.object({ L1: z.string(), L2: z.string(), L3: z.string(), L4: z.string() }).strict(),
  reflectionRule: z.string().optional(),
}).strict();
export type DepthDim = z.infer<typeof DepthDimSchema>;

// Autonomy (A1–A6): behavior-count band, not a quality score. `event`
// describes what counts as one countable event for this signal.
const AutonomySignalSchema = z.object({
  id: z.string(),
  name: z.string(),
  means: z.string(),
  event: z.string(),
}).strict();
export type AutonomySignal = z.infer<typeof AutonomySignalSchema>;

// Prompt lens: reads AI-interaction traces only, never a third scoring axis.
const LensSchema = z.object({
  id: z.string(),
  name: z.string(),
  guide: z.string(),
}).strict();
export type Lens = z.infer<typeof LensSchema>;

// Official-standard projection (e.g. AP Research) — project-surface-only,
// training-conversion component note that it never composes with D/A.
const OfficialComponentSpecSchema = z.object({
  name: z.string(),
  scale: z.string(),
  "口径": z.string(),
}).strict();
export type OfficialComponentSpec = z.infer<typeof OfficialComponentSpecSchema>;

const OfficialStandardSchema = z.object({
  id: z.string(),
  name: z.string(),
  components: z.array(OfficialComponentSpecSchema),
  alignmentItems: z.array(z.string()),
}).strict();
export type OfficialStandard = z.infer<typeof OfficialStandardSchema>;

const DualAxisModelSchema = z.object({
  id: z.string(),
  name: z.string(),
  axiom: z.string(),
  depth: z.array(DepthDimSchema),
  autonomy: z.array(AutonomySignalSchema),
  autonomyBand: z.string(),
  opportunityRule: z.string(),
  lenses: z.array(LensSchema),
  lensNote: z.string(),
  standards: z.array(OfficialStandardSchema),
}).strict();
export type DualAxis = z.infer<typeof DualAxisModelSchema>;

export const DUALAXIS_MODEL: DualAxis = DualAxisModelSchema.parse(dualAxisJson);

export function Model(): DualAxis {
  return DUALAXIS_MODEL;
}
export function DepthDims(): DepthDim[] {
  return DUALAXIS_MODEL.depth;
}
export function AutonomySignals(): AutonomySignal[] {
  return DUALAXIS_MODEL.autonomy;
}
export function Lenses(): Lens[] {
  return DUALAXIS_MODEL.lenses;
}
export function AutonomyBand(): string {
  return DUALAXIS_MODEL.autonomyBand;
}
export function Standard(id: string): OfficialStandard | undefined {
  return DUALAXIS_MODEL.standards.find((s) => s.id === id);
}

// assertModelComplete: every depth dim has all four L1–L4 anchors non-empty;
// every autonomy signal has a non-empty event; every lens has a non-empty guide.
export function assertModelComplete(m: DualAxis): void {
  for (const d of m.depth) {
    for (const k of ["L1", "L2", "L3", "L4"] as const) {
      if (!d.anchors[k]?.trim()) throw new Error(`dualaxis depth ${d.id} missing anchor ${k}`);
    }
  }
  for (const a of m.autonomy) {
    if (!a.event?.trim()) throw new Error(`dualaxis autonomy ${a.id} missing event`);
  }
  for (const l of m.lenses) {
    if (!l.guide?.trim()) throw new Error(`dualaxis lens ${l.id} missing guide`);
  }
}
assertModelComplete(DUALAXIS_MODEL);
