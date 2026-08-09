import { z } from "zod";

// questionCard.ts — slice 3a · the 提问卡 (question card) adaptive sub-agent
// (all-statuses.md §2). Opening the card runs a multi-turn conversation that
// helps the student turn a vague/empty 目标 into a focused, personal research
// question, then fills the objective. This is the FIRST sub-agent card renderer.

// QuestionCardTurnReply — one turn of the sub-agent. `suggestedObjective` is the
// student's articulated research question echoed back for confirmation once
// `done` (never an AI invention — 铁律①); null mid-conversation.
export const QuestionCardTurnReply = z.object({
  narrate: z.string(),
  suggestedObjective: z.string().nullable(),
  done: z.boolean(),
});
export type QuestionCardTurnReply = z.infer<typeof QuestionCardTurnReply>;
