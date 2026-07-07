import type { Anchor } from "@mind-imprint/contracts";
import { API_BASE } from "./client";
import { parseSSE } from "./sse";

export type TurnEvent =
  | { type: "text"; delta: string }
  | { type: "card"; cardInstanceId: string; cardId: string; nudgeText: string; anchors: Anchor[] }
  | { type: "done"; messageId: string }
  | { type: "error"; code: string; message: string };

export async function* runTurn(taskId: string, userInput?: string, source?: "voice"): AsyncGenerator<TurnEvent> {
  const res = await fetch(`${API_BASE}/api/v1/tasks/${taskId}/turn`, {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json", Accept: "text/event-stream" },
    body: JSON.stringify({ user_input: userInput ?? "", source: source ?? "" }),
  });

  // Gate failures (403/404/…) come back as a JSON error envelope, not SSE.
  if (!res.ok || !res.body) {
    let code = "internal_error";
    let message = `HTTP ${res.status}`;
    try { const b = await res.json(); if (b?.error) { code = b.error.code ?? code; message = b.error.message ?? message; } } catch { /* */ }
    yield { type: "error", code, message };
    return;
  }

  for await (const frame of parseSSE(res.body)) {
    let d: Record<string, unknown>;
    try { d = JSON.parse(frame.data); } catch { continue; }
    if (frame.event === "text") yield { type: "text", delta: String(d.delta ?? "") };
    else if (frame.event === "card") yield { type: "card", cardInstanceId: String(d.card_instance_id), cardId: String(d.card_id), nudgeText: String(d.nudge_text ?? ""), anchors: Array.isArray(d.anchors) ? (d.anchors as Anchor[]) : [] };
    else if (frame.event === "done") yield { type: "done", messageId: String(d.message_id ?? "") };
    else if (frame.event === "error") {
      const err = (d.error ?? {}) as { code?: string; message?: string };
      yield { type: "error", code: err.code ?? "internal_error", message: err.message ?? "出错了" };
    }
  }
}
