import { z } from "zod";
import dualAxisJson from "./dualaxis.json";

export const SoloLevel = z.enum(["L1", "L2", "L3", "L4", "NA"]);
export type SoloLevel = z.infer<typeof SoloLevel>;

// A scored level excludes N/A (N/A = insufficient evidence, rendered neutrally, not on the bar).
export type ScoredLevel = Exclude<SoloLevel, "NA">;
export const SOLO_LABELS: Record<ScoredLevel, string> = { L1: "萌芽", L2: "发展中", L3: "熟练", L4: "卓越" };

export interface DualAxisDimension {
  id: string; axis: "depth" | "autonomy" | "cross"; name: string;
  anchors?: Record<string, string>; observationGuide?: string; guide?: string;
}
export interface DualAxisModel {
  id: string; name: string; axiom: string;
  axes: Record<string, { name: string; scoring: string; max: number }>;
  dimensions: DualAxisDimension[];
  promptTiers: { tier: string; label: string }[];
  soloLevels: { level: string; name: string }[];
}

const DualAxisModelSchema = z.object({
  id: z.string(), name: z.string(), axiom: z.string(),
  axes: z.record(z.object({ name: z.string(), scoring: z.string(), max: z.number() })),
  dimensions: z.array(z.object({
    id: z.string(), axis: z.enum(["depth", "autonomy", "cross"]), name: z.string(),
    anchors: z.record(z.string()).optional(),
    observationGuide: z.string().optional(), guide: z.string().optional(),
  })),
  promptTiers: z.array(z.object({ tier: z.string(), label: z.string() })),
  soloLevels: z.array(z.object({ level: z.string(), name: z.string() })),
});

export const DUALAXIS_MODEL: DualAxisModel = DualAxisModelSchema.parse(dualAxisJson) as DualAxisModel;

// assertModelComplete: every depth dim has all four 0–3 anchors; autonomy has a
// guide; cross has a guide.
export function assertModelComplete(m: DualAxisModel): void {
  for (const d of m.dimensions) {
    if (d.axis === "depth") {
      for (const k of ["0", "1", "2", "3"]) {
        if (!d.anchors?.[k]?.trim()) throw new Error(`dualaxis dim ${d.id} missing anchor ${k}`);
      }
    }
    if (d.axis === "autonomy" && !d.observationGuide?.trim()) throw new Error(`dualaxis dim ${d.id} missing observationGuide`);
    if (d.axis === "cross" && !d.guide?.trim()) throw new Error(`dualaxis dim ${d.id} missing guide`);
  }
}
assertModelComplete(DUALAXIS_MODEL);
