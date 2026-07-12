import type { Anchor } from "@mind-imprint/contracts";
import { API_BASE } from "./client";
import { apiFetch } from "./client";
import { parseSSE } from "./sse";

export type StudioTurnEvent =
  | { type: "intervention"; interventionId: string; body: string; anchor: string; criterion: string; level: string }
  | { type: "gate"; contract: string; status: string; passed: number; total: number; missing: string[] }
  | { type: "card"; cardInstanceId: string; cardId: string; nudgeText: string; anchors: Anchor[] }
  | { type: "done" }
  | { type: "error"; code: string; message: string };

export async function* studioTurn(projectId: string, userInput: string): AsyncGenerator<StudioTurnEvent> {
  const res = await fetch(`${API_BASE}/api/v1/projects/${projectId}/turn`, {
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
  for await (const frame of parseSSE(res.body)) {
    let data: any;
    try { data = frame.data ? JSON.parse(frame.data) : {}; } catch { continue; }
    switch (frame.event) {
      case "intervention": yield { type: "intervention", interventionId: data.intervention_id, body: data.body, anchor: data.anchor, criterion: data.criterion, level: data.level }; break;
      case "gate": yield { type: "gate", contract: data.contract, status: data.status, passed: data.passed, total: data.total, missing: data.missing ?? [] }; break;
      case "card": yield { type: "card", cardInstanceId: data.card_instance_id, cardId: data.card_id, nudgeText: data.nudge_text, anchors: data.anchors ?? [] }; break;
      case "done": yield { type: "done" }; break;
      case "error": yield { type: "error", code: data.error?.code ?? "internal_error", message: data.error?.message ?? "" }; break;
    }
  }
}

export async function postDisposition(projectId: string, interventionId: string, action: "accept" | "rewrite" | "reject", reason: string): Promise<void> {
  await apiFetch<void>(`/api/v1/projects/${projectId}/interventions/${interventionId}/disposition`, {
    method: "POST",
    body: JSON.stringify({ action, reason }),
  });
}
