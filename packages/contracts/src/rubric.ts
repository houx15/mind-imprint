import { z } from "zod";
import ctRubricJson from "./ct-rubric.json";

export const SoloLevel = z.enum(["L1", "L2", "L3", "L4", "NA"]);
export type SoloLevel = z.infer<typeof SoloLevel>;

// A scored level excludes N/A (N/A = insufficient evidence, rendered neutrally, not on the bar).
export type ScoredLevel = Exclude<SoloLevel, "NA">;
export const SOLO_LABELS: Record<ScoredLevel, string> = { L1: "萌芽", L2: "发展中", L3: "熟练", L4: "卓越" };

export interface RubricDimension {
  id: string; name: string; framework: string;
  anchors: { L1: string; L2: string; L3: string; L4: string };
}

// Rubric container: wraps a set of dimensions with metadata and validation.
export interface Rubric {
  id: string;
  name: string;
  dimensions: RubricDimension[];
}

// Zod mirror of the Rubric/RubricDimension interfaces above, used only to
// validate the canonical JSON at import time (packages/contracts/src/ct-rubric.json
// is the single source of truth; this schema does not replace the interfaces).
const RubricDimensionSchema = z.object({
  id: z.string(),
  name: z.string(),
  framework: z.string(),
  anchors: z.object({
    L1: z.string(),
    L2: z.string(),
    L3: z.string(),
    L4: z.string(),
  }),
});

const RubricSchema = z.object({
  id: z.string(),
  name: z.string(),
  dimensions: z.array(RubricDimensionSchema),
});

// CT_RUBRIC: the Critical Thinking rubric used by the platform, parsed from
// the canonical JSON (packages/contracts/src/ct-rubric.json). Mirrored into
// apps/api/internal/rubric via `make sync-rubric` for the Go engine.
// OPCVL (HS-D1…D12) is a separate rubric, deferred to its module (assessment §10 Q3).
export const CT_RUBRIC: Rubric = RubricSchema.parse(ctRubricJson);
export const FULL_RUBRIC: RubricDimension[] = CT_RUBRIC.dimensions;

// assertRubricComplete: validates that a rubric has all required anchors (no blanks).
export function assertRubricComplete(r: Rubric): void {
  for (const d of r.dimensions) {
    for (const lvl of ["L1", "L2", "L3", "L4"] as const) {
      if (!d.anchors[lvl] || d.anchors[lvl].trim() === "") {
        throw new Error(`rubric ${r.id} dim ${d.id} missing ${lvl} anchor`);
      }
    }
  }
}

assertRubricComplete(CT_RUBRIC);
