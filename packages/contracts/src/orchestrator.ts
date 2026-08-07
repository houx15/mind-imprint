import { z } from "zod";

export const StudioStage = z.enum([
  "topic_discussion",
  "proposal_forming",
  "plan_generation",
  "proposal_writing",
  "proposal_review",
  "body_writing",
  "retrospective",
]);
export type StudioStage = z.infer<typeof StudioStage>;

export const OpenTool = z.enum(["chat", "plan", "reading", "writing", "reflection"]);
export type OpenTool = z.infer<typeof OpenTool>;

export const WidthTier = z.enum(["chat", "half", "wide"]);
export type WidthTier = z.infer<typeof WidthTier>;

export const ProposalSection = z.enum(["objective", "reason", "activities", "resources", "counterpoints"]);
export type ProposalSection = z.infer<typeof ProposalSection>;

export const ReferenceRef = z.object({
  kind: z.enum(["material", "note", "annotation"]),
  id: z.string(),
  label: z.string(),
});
export type ReferenceRef = z.infer<typeof ReferenceRef>;

export const StudioState = z.object({
  stage: StudioStage,
  openTool: OpenTool,
  widthTier: WidthTier,
  reference: z.array(ReferenceRef),
  updatedAtTurn: z.number().int(),
});
export type StudioState = z.infer<typeof StudioState>;

// The orchestrator's tool vocabulary — mirrors summonCard.ts's {name,args} shape.
// This is what the MODEL emits (parsed server-side); the client never sends it.
export const OrchestratorTool = z.discriminatedUnion("name", [
  z.object({ name: z.literal("set_status"), args: z.object({ stage: StudioStage }) }),
  z.object({ name: z.literal("open_tool"), args: z.object({ tool: OpenTool, reason: z.string() }) }),
  z.object({ name: z.literal("curate_reference"), args: z.object({ items: z.array(ReferenceRef) }) }),
  z.object({ name: z.literal("propose_note"), args: z.object({ section: ProposalSection, value: z.string() }) }),
  z.object({ name: z.literal("summon_card"), args: z.object({ card_id: z.string(), reason: z.string(), nudge_text: z.string() }) }),
  z.object({ name: z.literal("request_review"), args: z.object({}) }),
]);
export type OrchestratorTool = z.infer<typeof OrchestratorTool>;

// A note the student confirms before it lands (铁律①). Producer: propose_note.
export const NoteProposal = z.object({ section: ProposalSection, value: z.string() });
export type NoteProposal = z.infer<typeof NoteProposal>;

// The card chip surfaced this turn. Producer: summon_card. Mirrors the existing
// coachProposalDTO (cardId/reason/nudgeText).
export const CardProposalWire = z.object({
  cardId: z.string(),
  reason: z.string(),
  nudgeText: z.string(),
});
export type CardProposalWire = z.infer<typeof CardProposalWire>;

// The turn response the frontend applies.
export const OrchestratorReply = z.object({
  narrate: z.string(),
  directive: StudioState,
  note: NoteProposal.nullable(),
  card: CardProposalWire.nullable(),
  reviewRequested: z.boolean(),
});
export type OrchestratorReply = z.infer<typeof OrchestratorReply>;
