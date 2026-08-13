import { QuestionCardTurnReply } from "@mind-imprint/contracts";
import { apiFetch } from "./client";

// questionCard.ts — slice 3a · the 提问卡 adaptive sub-agent client. The turn
// loop is stateless server-side: the modal holds the conversation and posts the
// whole history each turn. Commit fills proposal.objective.

export type QuestionCardMsg = { role: "student" | "ai"; text: string };

// The saved in-progress conversation (§2): reopening the modal continues where
// the student left off instead of restarting. Empty messages ⇒ a fresh card.
export type QuestionCardState = { messages: QuestionCardMsg[]; done: boolean; objective: string };

export async function getQuestionCardState(projectId: string): Promise<QuestionCardState> {
  const raw = await apiFetch<Partial<QuestionCardState>>(`/api/v1/projects/${projectId}/cards/question-card`, {
    method: "GET",
  });
  return {
    messages: Array.isArray(raw?.messages) ? (raw!.messages as QuestionCardMsg[]) : [],
    done: raw?.done === true,
    objective: typeof raw?.objective === "string" ? raw.objective : "",
  };
}

export async function questionCardTurn(projectId: string, messages: QuestionCardMsg[]): Promise<QuestionCardTurnReply> {
  const raw = await apiFetch<unknown>(`/api/v1/projects/${projectId}/cards/question-card/turn`, {
    method: "POST",
    body: JSON.stringify({ messages }),
  });
  return QuestionCardTurnReply.parse(raw);
}

export async function commitQuestionCard(
  projectId: string,
  objective: string,
  messages: QuestionCardMsg[] = [],
): Promise<{ objective: string }> {
  // The whole modal conversation goes with the commit — the design treats the
  // chat history as the 提问卡's detailed content, kept in the process tree.
  return apiFetch<{ objective: string }>(`/api/v1/projects/${projectId}/cards/question-card/commit`, {
    method: "POST",
    body: JSON.stringify({ objective, messages }),
  });
}
