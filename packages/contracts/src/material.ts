import { z } from "zod";

export const MaterialKind = z.enum(["article", "draft"]);
export const MaterialSource = z.enum(["fetched", "pasted"]);

export const MaterialBlock = z.object({
  id: z.string(),
  text: z.string(),
});

export const Material = z.object({
  id: z.string(),
  task_id: z.string(),
  kind: MaterialKind,
  source: MaterialSource,
  title: z.string(),
  source_url: z.string().nullable(),
  blocks: z.array(MaterialBlock),
  scratch: z.string(),
  created_at: z.string(),
});

export type MaterialKind = z.infer<typeof MaterialKind>;
export type MaterialSource = z.infer<typeof MaterialSource>;
export type MaterialBlock = z.infer<typeof MaterialBlock>;
export type Material = z.infer<typeof Material>;
