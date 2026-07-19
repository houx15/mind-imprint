import { z } from "zod";

// AbilityModel: the student-level 能力素养 model — a cross-session merge of the
// DualAxis reports. Mirrors apps/api/internal/ability.Model (camelCase). RL-5: the
// three blocks never combine into a total; a depth level of -1 = 证据不足 (fewer than
// 2 contributing sessions); 智识自主/元认知 carry no level.
export const AbilityDepth = z.object({
  code: z.string(),
  name: z.string(),
  level: z.number().int(),        // -1 = insufficient, else 0..3
  levelLabel: z.string(),
  evidenceCount: z.number().int().min(0),
});

export const AbilityModel = z.object({
  totalSessions: z.number().int().min(0),
  depth: z.array(AbilityDepth),
  autonomy: z.object({
    sessions: z.number().int().min(0),
    boundarySettings: z.number().int().min(0),
    adversaryInvites: z.number().int().min(0),
    anchoredSignals: z.number().int().min(0),
    promptedSignals: z.number().int().min(0),
  }),
  metacognition: z.object({
    highestSolo: z.string(),
    distribution: z.record(z.number().int()),
    spontaneous: z.number().int().min(0),
    prompted: z.number().int().min(0),
  }),
});
export type AbilityModel = z.infer<typeof AbilityModel>;
