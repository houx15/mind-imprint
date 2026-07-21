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
export type GraphNodeUnit = z.infer<typeof GraphNodeUnit>;
export const GraphEdgeUnit = z.object({
  id: z.string().min(1),
  from: z.string().min(1),
  to: z.string().min(1),
  type: z.string().min(1),
});
export type GraphEdgeUnit = z.infer<typeof GraphEdgeUnit>;
export const GraphState = z.object({
  nodes: z.array(GraphNodeUnit),
  edges: z.array(GraphEdgeUnit),
});
export type GraphState = z.infer<typeof GraphState>;

// compare (C1): two materials side by side with paired annotations. `right` is
// nullable by design — the empty right pane is not a loading state, it is the
// assignment SIFT exists to resolve. Every pair is student-authored: the
// relation and the note are her judgment and have no other honest producer.
export const CompareRelation = z.enum(["corroborates", "contradicts", "qualifies"]);
export type CompareRelation = z.infer<typeof CompareRelation>;

export const ComparePair = z.object({
  id: z.string().min(1),
  l_span: z.string().min(1),
  r_span: z.string().min(1),
  note: z.string(),
  relation: CompareRelation,
  author: z.literal("student"),
});
export type ComparePair = z.infer<typeof ComparePair>;

export const CompareState = z.object({
  left: AnnotateState,
  right: AnnotateState.nullable(),
  pairs: z.array(ComparePair),
});
export type CompareState = z.infer<typeof CompareState>;

// sort (C1): statements dropped into a fixed vocabulary of buckets, each with
// the student's own test/reason. Persists 1:1 into Anchor[]:
// quote = text, dimension = bucket, answer = reason, author = student.
export const SortItem = z.object({
  id: z.string().min(1),
  text: z.string(),
  bucket: z.string(),
  reason: z.string(),
  author: Author,
});
export type SortItem = z.infer<typeof SortItem>;
export const SortState = z.object({ items: z.array(SortItem) });
export type SortState = z.infer<typeof SortState>;

// scale (C1): items placed on an ORDERED axis of named stops. Same Anchor
// mapping as sort (dimension = stop); `rewrite` is a separate student-written
// field carried on its own anchor (dimension "rewrite") so it can ride the
// existing field_written_by completion predicate.
export const ScaleItem = z.object({
  id: z.string().min(1),
  text: z.string(),
  stop: z.string(),
  reason: z.string(),
  author: Author,
});
export type ScaleItem = z.infer<typeof ScaleItem>;
export const ScaleState = z.object({ items: z.array(ScaleItem), rewrite: z.string() });
export type ScaleState = z.infer<typeof ScaleState>;

// matrix (C1): student-authored rows x fixed columns. `id` is a client-only
// React key — the row's PERSISTED identity is `label` (Anchor.quote), which is
// what completion groups on and what the perspectives graph effect names the
// node. cells maps column id -> the student's cell text (Anchor.dimension ->
// Anchor.answer).
export const MatrixRow = z.object({
  id: z.string().min(1),
  label: z.string(),
  cells: z.record(z.string()),
  author: Author,
});
export type MatrixRow = z.infer<typeof MatrixRow>;
export const MatrixState = z.object({ rows: z.array(MatrixRow) });
export type MatrixState = z.infer<typeof MatrixState>;
