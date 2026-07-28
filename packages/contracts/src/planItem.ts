import { z } from "zod";

// One card on the plan board. tag routes the doorway to a room; column is its
// kanban lane; start/days drive the gantt bar; position orders it within a lane.
export const PlanTag = z.enum(["read", "write", "review"]);
export type PlanTag = z.infer<typeof PlanTag>;

export const PlanColumn = z.enum(["todo", "doing", "done"]);
export type PlanColumn = z.infer<typeof PlanColumn>;

export const PlanItem = z.object({
  id: z.string(),
  title: z.string(),
  tag: PlanTag,
  column: PlanColumn,
  stage: z.string(),
  refMaterialId: z.string().nullable(),
  start: z.number().int(),
  days: z.number().int(),
  position: z.number().int(),
});
export type PlanItem = z.infer<typeof PlanItem>;
