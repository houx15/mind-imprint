import { DualAxisReport } from "@mind-imprint/contracts";
import { apiFetch } from "./client";

// A2: the chat thread's report. Student-opt-in — generateChatAssessment is only
// ever called from an explicit click, never on load (铁律 2).
export async function getChatAssessment(threadId: string): Promise<DualAxisReport | null> {
  const raw = await apiFetch<unknown>(`/api/v1/chat/threads/${threadId}/assessment`);
  if (raw == null) return null;
  return DualAxisReport.parse(raw);
}

export async function generateChatAssessment(threadId: string): Promise<DualAxisReport> {
  const raw = await apiFetch<unknown>(`/api/v1/chat/threads/${threadId}/assessment`, { method: "POST" });
  return DualAxisReport.parse(raw);
}
