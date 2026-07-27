import type { Anchor } from "@mind-imprint/contracts";
import { SelectionEval } from "@mind-imprint/contracts";
import { API_BASE, apiFetch } from "./client";
import { parseSSE } from "./sse";
import { mapStudioFrame, type StudioTurnEvent } from "./studioTurn";

export async function* readTurn(
  projectId: string,
  materialId: string,
  body: { student_text: string; focused_spans: { block_id: string; quote: string }[] },
): AsyncGenerator<StudioTurnEvent> {
  const res = await fetch(`${API_BASE}/api/v1/projects/${projectId}/materials/${materialId}/read-turn`, {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json", Accept: "text/event-stream" },
    body: JSON.stringify(body),
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

export async function evaluateCardSelection(
  projectId: string,
  cid: string,
  body: { block_id: string; start: number; end: number; quote: string; dimension: string },
): Promise<SelectionEval> {
  const raw = await apiFetch<unknown>(`/api/v1/projects/${projectId}/cards/${cid}/evaluate`, {
    method: "POST",
    body: JSON.stringify(body),
  });
  return SelectionEval.parse(raw);
}
