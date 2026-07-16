import { z } from "zod";
import { SoloLevel } from "./rubric";

// DimensionScore: one CT dimension's growth-report row — level + the
// behavioral evidence backing it (RL-5: diagnostic per-dimension, never a
// grade/rank/total). Mirrors apps/api/internal/studio.AssessmentDimensionDTO
// (camelCase over the wire) byte-for-byte.
export const DimensionScore = z.object({
  code: z.string(),
  name: z.string(),
  level: SoloLevel,
  evidence: z.string(),
});
export type DimensionScore = z.infer<typeof DimensionScore>;

// Assessment: the growth report's own wire surface — NOT part of
// StudioProjection (the assessor is isolated from the coach loop, Slice 10
// spec). Mirrors apps/api/internal/studio.AssessmentDTO.
export const Assessment = z.object({
  dimensions: z.array(DimensionScore),
  narrative: z.string(),
  generatedAt: z.string(),
});
export type Assessment = z.infer<typeof Assessment>;
