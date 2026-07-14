import type { Anchor } from "@mind-imprint/contracts";
import { API_BASE } from "./client";
import { apiFetch } from "./client";
import { parseSSE, type SSEFrame } from "./sse";

export type StudioTurnEvent =
  | { type: "intervention"; interventionId: string; body: string; anchor: string; criterion: string; level: string }
  | { type: "gate"; contract: string; status: string; passed: number; total: number; missing: string[] }
  // materialId is the material this card is ABOUT (the server's
  // card_instance--evaluates-->material edge target, whole-branch review
  // finding [5]) — carried so the client never has to guess it from anchor
  // contents or array position, which is exactly the coin flip a compare
  // card's two-material anchor set invites.
  | { type: "card"; cardInstanceId: string; cardId: string; nudgeText: string; anchors: Anchor[]; materialId: string }
  | { type: "done" }
  | { type: "error"; code: string; message: string };

/** Maps one SSE frame to a StudioTurnEvent (or null for unrecognized/unparsable frames).
 *  Shared by studioTurn and submitProjectCard so the intervention/gate/card/done/error
 *  switch lives in exactly one place. */
export function mapStudioFrame(frame: SSEFrame): StudioTurnEvent | null {
  let data: any;
  try { data = frame.data ? JSON.parse(frame.data) : {}; } catch { return null; }
  switch (frame.event) {
    case "intervention": return { type: "intervention", interventionId: data.intervention_id, body: data.body, anchor: data.anchor, criterion: data.criterion, level: data.level };
    case "gate": return { type: "gate", contract: data.contract, status: data.status, passed: data.passed, total: data.total, missing: data.missing ?? [] };
    case "card": return { type: "card", cardInstanceId: data.card_instance_id, cardId: data.card_id, nudgeText: data.nudge_text, anchors: data.anchors ?? [], materialId: data.material_id ?? "" };
    case "done": return { type: "done" };
    case "error": return { type: "error", code: data.error?.code ?? "internal_error", message: data.error?.message ?? "" };
    default: return null;
  }
}

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
    const event = mapStudioFrame(frame);
    if (event) yield event;
  }
}

export async function postDisposition(projectId: string, interventionId: string, action: "accept" | "rewrite" | "reject", reason: string): Promise<void> {
  await apiFetch<void>(`/api/v1/projects/${projectId}/interventions/${interventionId}/disposition`, {
    method: "POST",
    body: JSON.stringify({ action, reason }),
  });
}
