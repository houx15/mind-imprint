import { z } from "zod";

export const SkillKind = z.enum(["project", "course"]);
export type SkillKind = z.infer<typeof SkillKind>;

// MachineItem is one machine-checkable gate item (mirrors Go's skills.MachineItem).
// gate.machine accepts either this object form (what the authored skill JSON
// actually uses) or a bare string, for backward compatibility with callers
// that pass a machine kind alone.
export const MachineItem = z.object({
  kind: z.string(),
  type: z.string().optional(),
  n: z.number().int().optional(),
});
export type MachineItem = z.infer<typeof MachineItem>;

export const Gate = z.object({
  machine: z.array(z.union([z.string(), MachineItem])).default([]),
  student_written: z.array(z.string()).default([]),
  human: z.array(z.string()).default([]),
});
export type Gate = z.infer<typeof Gate>;

export const FloorItem = z.object({
  kind: z.enum(["steps_viewed", "card_dispositioned", "student_turns_at_least"]),
  steps: z.array(z.number().int()).optional(),
  card_id: z.string().optional(),
  n: z.number().int().optional(),
});
export type FloorItem = z.infer<typeof FloorItem>;

export const PhasePage = z.object({
  title: z.string(),
  subtitle: z.string().default(""),
  body: z.array(z.string()).default([]),
});
export type PhasePage = z.infer<typeof PhasePage>;

export const AnchorMaterial = z.object({ title: z.string(), text: z.string() });
export type AnchorMaterial = z.infer<typeof AnchorMaterial>;

export const Contract = z.object({
  requires: z.array(z.string()).default([]),
  produces: z.array(z.string()).default([]),
  view: z.string().optional(),
  repertoire: z.array(z.string()).default([]),
  gate: Gate,
  // Course-only fields (Slice 12). All optional — writing-project.json sets
  // none of them and must keep parsing unchanged.
  goal: z.string().optional(),
  steps: z.array(z.number().int()).optional(),
  page: PhasePage.optional(),
  cards: z.array(z.string()).optional(),
  anchor_material: AnchorMaterial.optional(),
  ask_chips: z.array(z.string()).optional(),
  floor: z.array(FloorItem).optional(),
  soft_condition: z.string().optional(),
});
export type Contract = z.infer<typeof Contract>;

export const Skill = z.object({
  id: z.string().min(1),
  kind: SkillKind,
  contracts: z.record(Contract),
  intake: z.record(z.unknown()).optional(),
  vocabulary: z.string().optional(),
  cards: z.array(z.string()).default([]),
  course_id: z.string().optional(),
});
export type Skill = z.infer<typeof Skill>;
