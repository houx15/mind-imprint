import { z } from "zod";

// Closed set of agent verbs (platform primitives for output)
export const Verb = z.enum([
  "surface_card",
  "post_intervention",
  "check_gate",
  "plan",
  "replan",
  "advance",
  "route",
  "invite_commit",
  "reply",
  "propose",
  "order_review",
]);
export type Verb = z.infer<typeof Verb>;

// Anchors can reference heavy nodes or light graph nodes
export const OutputAnchor = z.object({
  kind: z.enum(["graph_node", "material", "draft_snapshot", "card_instance", "artifact", "gate_item"]),
  id: z.string(),
  span: z.record(z.unknown()).optional(),
});
export type OutputAnchor = z.infer<typeof OutputAnchor>;

// Agent output is a discriminated union across the six output types
export const AgentOutput = z.discriminatedUnion("type", [
  z.object({
    type: z.literal("question"),
    anchor: OutputAnchor,
    criterion: z.string(),
    body: z.string(),
  }),
  z.object({
    type: z.literal("diagnostic"),
    anchor: OutputAnchor,
    criterion: z.string(),
    body: z.string(),
  }),
  z.object({
    type: z.literal("reference"),
    anchor: OutputAnchor,
    quote: z.string(),
    provenance: z.string().min(1),
  }),
  z.object({
    type: z.literal("proposal"),
    anchor: OutputAnchor,
    criterion: z.string(),
    body: z.string(),
  }),
  z.object({
    type: z.literal("plan"),
    route: z.array(z.string()),
  }),
  z.object({
    type: z.literal("reply"),
    body: z.string().min(1),
  }),
]);
export type AgentOutput = z.infer<typeof AgentOutput>;
