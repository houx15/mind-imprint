import type { CourseSession } from "@mind-imprint/contracts";
import { API_BASE, apiFetch } from "./client";
import { parseSSE } from "./sse";

export type CourseTurnEvent =
  | { type: "reply"; body: string }
  | { type: "card"; cardInstanceId: string; cardId: string; materialId: string }
  | { type: "phase"; to: string }
  | { type: "done" }
  | { type: "error"; code: string; message: string };

export function startCourseSession(courseId: string): Promise<CourseSession> {
  return apiFetch<CourseSession>(`/api/v1/courses/${courseId}/session`, { method: "POST" });
}
export function getCourseSession(courseId: string): Promise<CourseSession> {
  return apiFetch<CourseSession>(`/api/v1/courses/${courseId}/session`);
}
export function submitCourseCard(courseId: string, cardInstanceId: string, payload: { field_values: unknown; event_trace: unknown; anchors: unknown }): Promise<void> {
  return apiFetch<void>(`/api/v1/courses/${courseId}/session/cards/${cardInstanceId}/submit`, { method: "POST", body: JSON.stringify(payload) });
}
export function skipCourseCard(courseId: string, cardInstanceId: string): Promise<void> {
  return apiFetch<void>(`/api/v1/courses/${courseId}/session/cards/${cardInstanceId}/skip`, { method: "POST" });
}

// runCourseTurn is the shared body of courseAsk/courseAdvance — both drive an
// SSE stream carrying the same frame vocabulary as Chat's turn, plus the new
// `phase` frame (Slice 12). Mirrors chatTurn.
async function* runCourseTurn(url: string, body?: unknown): AsyncGenerator<CourseTurnEvent> {
  const res = await fetch(url, {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json", Accept: "text/event-stream" },
    body: body !== undefined ? JSON.stringify(body) : undefined,
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
      case "phase": yield { type: "phase", to: data.to }; break;
      case "done": yield { type: "done" }; break;
      case "error": yield { type: "error", code: data.error?.code ?? "internal_error", message: data.error?.message ?? "" }; break;
    }
  }
}

export function courseAsk(courseId: string, userInput: string): AsyncGenerator<CourseTurnEvent> {
  return runCourseTurn(`${API_BASE}/api/v1/courses/${courseId}/session/ask`, { user_input: userInput });
}

// courseAdvance decodes NO body at all — the phase floor's inputs are all
// server-side (DEC-12.2): a floor the client can assert is not a floor.
export function courseAdvance(courseId: string): AsyncGenerator<CourseTurnEvent> {
  return runCourseTurn(`${API_BASE}/api/v1/courses/${courseId}/session/advance`);
}
