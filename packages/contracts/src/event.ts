import { z } from "zod";

export const Surface = z.enum(["studio", "course", "chat"]);
export type Surface = z.infer<typeof Surface>;

// Event types registry for the event stream
export const EVENT_TYPES = [
  "prompt_sent",
  "card_clicked",
  "gate_attempt",
  "suggestion_disposition",
  "verbalization_submitted",
  "source_opened",
  "citation_added",
  "version_saved",
  "rescue_triggered",
  "stance_change_logged",
  "chat_message",
] as const;

// Discriminated union of all event variants
export const StudioEvent = z.discriminatedUnion("type", [
  z.object({
    type: z.literal("prompt_sent"),
    surface: Surface,
    prompt_id: z.string().optional(),
  }),
  z.object({
    type: z.literal("card_clicked"),
    surface: Surface,
    card_id: z.string(),
    unprompted: z.boolean(),
  }),
  z.object({
    type: z.literal("gate_attempt"),
    surface: Surface,
    gate_id: z.string(),
    passed: z.boolean(),
  }),
  z.object({
    type: z.literal("suggestion_disposition"),
    surface: Surface,
    action: z.enum(["accept", "reject", "rewrite"]),
    reason: z.string(),
  }),
  z.object({
    type: z.literal("verbalization_submitted"),
    surface: Surface,
    text: z.string(),
    source: z.string().optional(),
  }),
  z.object({
    type: z.literal("source_opened"),
    surface: Surface,
    url: z.string(),
    time_spent_s: z.number(),
    tier: z.string().optional(),
    lateral_read: z.boolean().optional(),
  }),
  z.object({
    type: z.literal("citation_added"),
    surface: Surface,
    source_id: z.string(),
    quote: z.string().optional(),
  }),
  z.object({
    type: z.literal("version_saved"),
    surface: Surface,
    artifact_id: z.string(),
    version_num: z.number().int(),
  }),
  z.object({
    type: z.literal("rescue_triggered"),
    surface: Surface,
    reason: z.string(),
  }),
  z.object({
    type: z.literal("stance_change_logged"),
    surface: Surface,
    topic: z.string(),
    old_stance: z.string(),
    new_stance: z.string(),
  }),
  z.object({
    type: z.literal("chat_message"),
    surface: Surface,
    message_id: z.string(),
    role: z.enum(["user", "assistant"]),
  }),
]);
export type StudioEvent = z.infer<typeof StudioEvent>;
