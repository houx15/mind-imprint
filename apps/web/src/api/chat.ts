import type { ChatThread, ChatMessage } from "@mind-imprint/contracts";
import { API_BASE, apiFetch } from "./client";
import { parseSSE } from "./sse";

export type ChatTurnEvent =
  | { type: "reply"; body: string }
  | { type: "card"; cardInstanceId: string; cardId: string; materialId: string }
  | { type: "done" }
  | { type: "error"; code: string; message: string };

export function listThreads(): Promise<ChatThread[]> {
  return apiFetch<ChatThread[]>("/api/v1/chat/threads");
}
export function createThread(title = ""): Promise<ChatThread> {
  return apiFetch<ChatThread>("/api/v1/chat/threads", { method: "POST", body: JSON.stringify({ title }) });
}
export function getMessages(threadId: string): Promise<ChatMessage[]> {
  return apiFetch<ChatMessage[]>(`/api/v1/chat/threads/${threadId}/messages`);
}
export function submitChatCard(threadId: string, cardInstanceId: string, payload: { field_values: unknown; event_trace: unknown; anchors: unknown }): Promise<void> {
  return apiFetch<void>(`/api/v1/chat/threads/${threadId}/cards/${cardInstanceId}/submit`, { method: "POST", body: JSON.stringify(payload) });
}
export function skipChatCard(threadId: string, cardInstanceId: string): Promise<void> {
  return apiFetch<void>(`/api/v1/chat/threads/${threadId}/cards/${cardInstanceId}/skip`, { method: "POST" });
}

export async function* chatTurn(threadId: string, userInput: string): AsyncGenerator<ChatTurnEvent> {
  const res = await fetch(`${API_BASE}/api/v1/chat/threads/${threadId}/turn`, {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json", Accept: "text/event-stream" },
    body: JSON.stringify({ user_input: userInput }),
  });
  if (!res.ok || !res.body) {
    let code = "internal_error", message = `HTTP ${res.status}`;
    try { const b = await res.json(); if (b?.error) { code = b.error.code ?? code; message = b.error.message ?? message; } } catch { /* non-JSON */ }
    yield { type: "error", code, message };
    return;
  }
  let reply = "";
  for await (const frame of parseSSE(res.body)) {
    let data: any;
    try { data = frame.data ? JSON.parse(frame.data) : {}; } catch { continue; }
    switch (frame.event) {
      case "text": reply += data.delta ?? ""; yield { type: "reply", body: reply }; break;
      case "card": yield { type: "card", cardInstanceId: data.card_instance_id, cardId: data.card_id, materialId: data.material_id ?? "" }; break;
      case "done": yield { type: "done" }; break;
      case "error": yield { type: "error", code: data.error?.code ?? "internal_error", message: data.error?.message ?? "" }; break;
    }
  }
}
