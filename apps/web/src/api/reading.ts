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

// summonCard is the lens-library summon path: the student BROWSES the
// reading deck and picks a card herself, rather than waiting for the
// read-together router to propose one (readTurn above). Same SSE
// generator shape as readTurn — a `card` frame on success, an
// `intervention` frame (no card) on rejection/failure, always ending in
// `done`.
export async function* summonCard(
  projectId: string,
  materialId: string,
  cardId: string,
): AsyncGenerator<StudioTurnEvent> {
  const res = await fetch(`${API_BASE}/api/v1/projects/${projectId}/materials/${materialId}/summon-card`, {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json", Accept: "text/event-stream" },
    body: JSON.stringify({ card_id: cardId }),
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

// getOpenCard — the deadlock-prevention fix: the reading room asks, on
// mount, whether a card is already open (proposed/active) for this material
// — a leftover from before the room reloaded, otherwise invisible behind the
// project-wide one-active mutex — so it can resume/show it instead of
// leaving the student stuck with no card and no way to summon a new one.
// null when there's no open card (server's card_instance_id: "" sentinel).
export async function getOpenCard(
  projectId: string,
  materialId: string,
): Promise<{ cardInstanceId: string; cardId: string; status: "proposed" | "active"; anchors: Anchor[] } | null> {
  const raw = await apiFetch<{
    card_instance_id: string;
    card_id: string;
    status: string;
    anchors: Anchor[];
  }>(`/api/v1/projects/${projectId}/materials/${materialId}/open-card`);
  if (!raw.card_instance_id) return null;
  return {
    cardInstanceId: raw.card_instance_id,
    cardId: raw.card_id,
    status: raw.status as "proposed" | "active",
    anchors: raw.anchors ?? [],
  };
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
