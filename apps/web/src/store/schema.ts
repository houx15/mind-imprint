import { z } from "zod";
import { Task, Message, CardInstance, Evaluation } from "@mind-imprint/contracts";

export const StoreState = z.object({
  version: z.literal(1),
  tasks: z.array(Task),
  messages: z.array(Message),
  cards: z.array(CardInstance),
  evaluations: z.array(Evaluation).default([]),
  lastSeenEvaluationAt: z.record(z.string()).default({}),
});

export type StoreState = z.infer<typeof StoreState>;

export const EMPTY_STATE: StoreState = { version: 1, tasks: [], messages: [], cards: [], evaluations: [], lastSeenEvaluationAt: {} };
