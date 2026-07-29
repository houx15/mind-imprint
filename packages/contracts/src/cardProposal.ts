import { z } from "zod";

// S4 cross-phase card proposing: the always-reply coach may, at the 克制
// ladder's summon rung, attach an OPTIONAL proposal to its reply — an offer to
// open a student card. Opening is the student's tap (铁律: triggering is
// automatic, opening is confirmed). respond/hint rungs carry no proposal.
export const CardProposal = z.object({
  cardId: z.string().min(1),
  reason: z.string(),
  nudgeText: z.string(),
});
export type CardProposal = z.infer<typeof CardProposal>;

// The /coach JSON response: reply is always present; proposal is nullish
// (absent or null) on the respond/hint rungs.
export const CoachReply = z.object({
  reply: z.string(),
  proposal: CardProposal.nullish(),
});
export type CoachReply = z.infer<typeof CoachReply>;

// S4 compaction backstop's rolling per-project digest (folded older turns).
export const ConversationDigest = z.object({
  prose: z.string(),
  turnsFolded: z.number().int().nonnegative(),
});
export type ConversationDigest = z.infer<typeof ConversationDigest>;
