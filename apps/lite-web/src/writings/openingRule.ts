// writings/openingRule.ts — whether 印记's opening line should be requested.
//
// POST /writings/{id}/opening makes a model call whenever the transcript has
// no line from 印记. A finished writing never needs one: once she presses 修改
// the room reopens on a piece that already has a draft and at least one
// submitted version, and an opening line there would cost a model call and
// restart the conversation as if the writing were new.

import { isWritingFinished, type Writing } from "../api/writings";

export function coachOpeningNeeded(
  writing: Pick<Writing, "setupAt" | "status" | "finishedAt">,
  messages: readonly { role: string }[],
): boolean {
  return writing.setupAt !== null && !isWritingFinished(writing) && !messages.some((m) => m.role === "ai");
}
