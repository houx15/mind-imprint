import { z } from "zod";

export const SkillKind = z.enum(["project", "course"]);
export type SkillKind = z.infer<typeof SkillKind>;

export const Gate = z.object({
  machine: z.array(z.string()).default([]),
  student_written: z.array(z.string()).default([]),
  human: z.array(z.string()).default([]),
});
export type Gate = z.infer<typeof Gate>;

export const Contract = z.object({
  requires: z.array(z.string()).default([]),
  produces: z.array(z.string()).default([]),
  view: z.string().optional(),
  repertoire: z.array(z.string()).default([]),
  gate: Gate,
});
export type Contract = z.infer<typeof Contract>;

export const Skill = z.object({
  id: z.string().min(1),
  kind: SkillKind,
  contracts: z.record(Contract),
  intake: z.record(z.unknown()).optional(),
  vocabulary: z.string().optional(),
  cards: z.array(z.string()).default([]),
});
export type Skill = z.infer<typeof Skill>;
