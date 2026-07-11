import { z } from "zod";

// author is tracked on every mutable unit — enforcement + assessment depend on it.
export const Author = z.enum(["student", "ai", "imported"]);
export type Author = z.infer<typeof Author>;

// The finite hand-built primitive library (agent-spec C1). annotate + graph are
// defined now (built first in Slice 1); the rest are added when their slice needs them.
export const PRIMITIVE_KINDS = ["annotate", "graph", "sort", "matrix", "scale", "compare"] as const;
export const PrimitiveKind = z.enum(PRIMITIVE_KINDS);
export type PrimitiveKind = z.infer<typeof PrimitiveKind>;

export const AnnotateSpan = z.object({
  id: z.string().min(1),
  // span anchors either to a character range or a block id in the material
  range: z.object({ start: z.number().int(), end: z.number().int() }).optional(),
  block_ref: z.string().optional(),
  tag: z.string().min(1),
  note: z.string(),
  author: Author,
});
export const AnnotateState = z.object({
  material_id: z.string().min(1),
  spans: z.array(AnnotateSpan),
});
export type AnnotateState = z.infer<typeof AnnotateState>;

export const GraphNodeUnit = z.object({
  id: z.string().min(1),
  type: z.string().min(1),
  text: z.string(),
  author: Author,
});
export const GraphEdgeUnit = z.object({
  id: z.string().min(1),
  from: z.string().min(1),
  to: z.string().min(1),
  type: z.string().min(1),
});
export const GraphState = z.object({
  nodes: z.array(GraphNodeUnit),
  edges: z.array(GraphEdgeUnit),
});
export type GraphState = z.infer<typeof GraphState>;
