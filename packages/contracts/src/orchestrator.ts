import { z } from "zod";
import { CardTurnRef } from "./cardReflect";

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

export const OpenTool = z.enum(["chat", "forming", "plan", "reading", "writing", "reflection"]);
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
  started: z.boolean(),
  // slice 3a · a lean read of the proposal guide-step track (the full track,
  // incl. sub-questions + cached guides, is served by GET /proposal-track).
  // nullish so old rows / the essay path (no proposal track) parse cleanly.
  proposalTrack: z
    .object({
      mode: z.enum(["", "free", "guided"]),
      stepIndex: z.number().int(),
      started: z.boolean(),
    })
    .nullish(),
  // slice 3a (Finding 2) · the student waived the 反例 prompt before plan-gen.
  counterpointsWaived: z.boolean().optional(),
  // slice 4a · the essay's stage (research → statement → submission). nullish
  // until 完成提案 enters the essay.
  essayTrack: z.object({ stage: z.enum(["research", "statement", "submission"]) }).nullish(),
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
  z.object({ name: z.literal("generate_plan"), args: z.object({}) }),
  z.object({ name: z.literal("propose_question"), args: z.object({ text: z.string() }) }),
]);
export type OrchestratorTool = z.infer<typeof OrchestratorTool>;

// A note the student confirms before it lands (铁律①). Producer: propose_note.
export const NoteProposal = z.object({ section: ProposalSection, value: z.string() });
export type NoteProposal = z.infer<typeof NoteProposal>;

// A candidate inquiry question the student confirms before it lands. Producer: propose_question.
export const QuestionProposal = z.object({ text: z.string() });
export type QuestionProposal = z.infer<typeof QuestionProposal>;

// The card chip surfaced this turn. Producer: summon_card. Mirrors the existing
// coachProposalDTO (cardId/reason/nudgeText).
export const CardProposalWire = z.object({
  cardId: z.string(),
  reason: z.string(),
  nudgeText: z.string(),
});
export type CardProposalWire = z.infer<typeof CardProposalWire>;

// The turn response the frontend applies.
// NextStep is the one-tap advance the deterministic flow router offers when a
// status milestone is reached (铁律②: 打开由学生确认). toStatus is a FlowStatus
// code; surface is the OpenTool that opens on advance.
export const NextStep = z.object({
  label: z.string(),
  toStatus: z.string(),
  surface: z.string(),
});
export type NextStep = z.infer<typeof NextStep>;

// ReviewVerdict — a reasoning-model reviewer's read at a gate point (slice 2:
// framework readiness). Strong-advisory: `ready` reflects "solid enough to
// proceed", never blocks; `suggestions` are concrete, pointed at the weakest
// spots. Attached to OrchestratorReply when a gate reviewer ran this turn.
export const ReviewVerdict = z.object({
  ready: z.boolean(),
  why: z.string(),
  suggestions: z.array(z.string()),
});
export type ReviewVerdict = z.infer<typeof ReviewVerdict>;

export const OrchestratorReply = z.object({
  narrate: z.string(),
  directive: StudioState,
  note: NoteProposal.nullable(),
  card: CardProposalWire.nullable(),
  question: QuestionProposal.nullable(),
  reviewRequested: z.boolean(),
  planGenerated: z.boolean(),
  compacted: z.boolean(),
  nextStep: NextStep.nullable().optional(),
  reviewVerdict: ReviewVerdict.nullable().optional(),
});
export type OrchestratorReply = z.infer<typeof OrchestratorReply>;

// A single coach-thread message as returned by GET /coach/history. Mirrors the
// web-local CoachHistoryMsg previously declared ad hoc in workspace.ts; card
// reuses CardTurnRef (cardReflect.ts) rather than duplicating its shape.
export const CoachHistoryMsg = z.object({
  role: z.enum(["student", "ai"]),
  text: z.string(),
  card: CardTurnRef.nullish(),
});
export type CoachHistoryMsg = z.infer<typeof CoachHistoryMsg>;

// A paginated page of coach history. recap/nextCursor are explicitly nullable
// (not optional) — the server always includes both keys.
export const CoachHistoryPage = z.object({
  messages: z.array(CoachHistoryMsg),
  hasMore: z.boolean(),
  recap: z.string().nullable(),
  nextCursor: z.string().nullable(),
});
export type CoachHistoryPage = z.infer<typeof CoachHistoryPage>;
