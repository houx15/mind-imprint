import { z } from "zod";
import { FieldPrimitive } from "./primitives";

export const Step = z.object({
  key: z.string().min(1),
  title: z.string().min(1),
  disclose: z.enum(["always", "on_demand"]),
  methodology_note: z.string(),
  fields: z.array(FieldPrimitive).min(1),
});

export const CardSpec = z.object({
  id: z.string().min(1),
  category: z.string().min(1),
  name: z.string().min(1),
  purpose: z.string(),
  trigger_condition: z.string(),
  steps: z.array(Step).min(1),
  rubric_tags: z.array(z.string()),
});

export type Step = z.infer<typeof Step>;
export type CardSpec = z.infer<typeof CardSpec>;
