import type { CardInstance, TraceEvent } from "@mind-imprint/contracts";
import { apiFetch } from "./client";

export async function activateCard(taskId: string, cardId: string): Promise<CardInstance> {
  const r = await apiFetch<{ card: CardInstance }>(`/api/v1/tasks/${taskId}/cards/${cardId}`, {
    method: "PATCH", body: JSON.stringify({ status: "active" }),
  });
  return r.card;
}

export async function submitCard(taskId: string, cardId: string, env: CardInstance): Promise<CardInstance> {
  const r = await apiFetch<{ card: CardInstance }>(`/api/v1/tasks/${taskId}/cards/${cardId}`, {
    method: "PUT",
    body: JSON.stringify({ status: "completed", field_values: env.field_values, event_trace: env.event_trace, anchors: env.anchors }),
  });
  return r.card;
}

export async function skipCard(taskId: string, cardId: string, eventTrace: TraceEvent[]): Promise<CardInstance> {
  const r = await apiFetch<{ card: CardInstance }>(`/api/v1/tasks/${taskId}/cards/${cardId}/skip`, {
    method: "POST", body: JSON.stringify({ event_trace: eventTrace }),
  });
  return r.card;
}
