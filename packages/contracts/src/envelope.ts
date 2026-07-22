import { z } from "zod";
import { Anchor } from "./anchor";

// N3c Task 6: span_located/span_not_found are additive — existing kinds are
// unchanged in shape. This union is enforced a SECOND time in Go's
// traceKinds set (apps/api/internal/api/cards.go); the two must move
// together or every L2/L3 submit 400s at the widened kind.
export const TraceEvent = z.discriminatedUnion("kind", [
  z.object({ kind: z.literal("field_change"), path: z.string(), at: z.string() }),
  z.object({ kind: z.literal("step_expand"), step_key: z.string(), at: z.string() }),
  z.object({ kind: z.literal("note_open"), step_key: z.string(), at: z.string() }),
  z.object({ kind: z.literal("skip"), at: z.string() }),
  z.object({ kind: z.literal("submit"), at: z.string() }),
  z.object({ kind: z.literal("span_located"), dimension: z.string(), block_id: z.string(), at: z.string() }),
  z.object({ kind: z.literal("span_not_found"), dimension: z.string(), at: z.string() }),
]);

export const CardStatus = z.enum(["proposed", "active", "completed", "skipped"]);

export const CardInstance = z.object({
  id: z.string(),
  card_id: z.string(),
  task_id: z.string(),
  parent_node_id: z.string().nullable(),
  status: CardStatus,
  field_values: z.record(z.unknown()),
  event_trace: z.array(TraceEvent),
  anchors: z.array(Anchor).default([]),
  rubric_tags: z.array(z.string()),
  created_at: z.string(),
  completed_at: z.string().nullable(),
});

export type TraceEvent = z.infer<typeof TraceEvent>;
export type CardStatus = z.infer<typeof CardStatus>;
export type CardInstance = z.infer<typeof CardInstance>;
