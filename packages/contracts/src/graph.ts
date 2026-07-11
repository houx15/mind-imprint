import { z } from "zod";
import { Author } from "./interactionPrimitive";

// Light argument-graph node types. Heavy participants (material, draft_snapshot,
// card_instance) keep their own tables and are referenced by NodeRefKind.
export const GraphNodeType = z.enum(["claim", "evidence", "plan", "gate_state", "note"]);
export type GraphNodeType = z.infer<typeof GraphNodeType>;

export const GraphNode = z.object({
  id: z.string(),
  project_id: z.string(),
  type: GraphNodeType,
  body: z.record(z.unknown()),
  author: Author,
  span_ref: z.record(z.unknown()).nullable().optional(),
  created_at: z.string(),
});
export type GraphNode = z.infer<typeof GraphNode>;

// Polymorphic edge endpoints: an edge may link a light graph_node to a heavy node.
export const NodeRefKind = z.enum(["graph_node", "material", "draft_snapshot", "card_instance"]);
export type NodeRefKind = z.infer<typeof NodeRefKind>;

export const GraphEdge = z.object({
  id: z.string(),
  project_id: z.string(),
  type: z.string().min(1),
  from_kind: NodeRefKind,
  from_id: z.string(),
  to_kind: NodeRefKind,
  to_id: z.string(),
  created_at: z.string(),
});
export type GraphEdge = z.infer<typeof GraphEdge>;
