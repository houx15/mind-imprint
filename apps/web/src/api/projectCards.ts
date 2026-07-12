import type { Anchor, TraceEvent } from "@mind-imprint/contracts";
import { API_BASE, apiFetch } from "./client";
import { parseSSE } from "./sse";
import { mapStudioFrame, type StudioTurnEvent } from "./studioTurn";

export async function activateProjectCard(projectId: string, cid: string): Promise<void> {
  await apiFetch<void>(`/api/v1/projects/${projectId}/cards/${cid}/activate`, { method: "POST" });
}

export async function skipProjectCard(projectId: string, cid: string, input: { event_trace: TraceEvent[] }): Promise<void> {
  await apiFetch<void>(`/api/v1/projects/${projectId}/cards/${cid}/skip`, {
    method: "POST",
    body: JSON.stringify({ event_trace: input.event_trace }),
  });
}

export async function* submitProjectCard(
  projectId: string,
  cid: string,
  input: { field_values: Record<string, unknown>; event_trace: TraceEvent[]; anchors: Anchor[] },
): AsyncGenerator<StudioTurnEvent> {
  const res = await fetch(`${API_BASE}/api/v1/projects/${projectId}/cards/${cid}/submit`, {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json", Accept: "text/event-stream" },
    body: JSON.stringify({ field_values: input.field_values, event_trace: input.event_trace, anchors: input.anchors }),
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
