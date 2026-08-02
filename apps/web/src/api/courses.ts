import type { CourseSummary, CoursePlayerPayload, CourseProgress, CourseReport } from "@mind-imprint/contracts";
import { API_BASE, apiFetch } from "./client";
import { parseSSE } from "./sse";

export type CourseAskEvent =
  | { type: "reply"; body: string }
  | { type: "error"; code?: string; message: string };

export async function listCourses(): Promise<CourseSummary[]> {
  const r = await apiFetch<{ courses: CourseSummary[] }>("/api/v1/courses");
  return r.courses;
}
export async function getCourse(slug: string): Promise<CoursePlayerPayload> {
  const r = await apiFetch<{ course: CoursePlayerPayload }>(`/api/v1/courses/${slug}`);
  return r.course;
}
export async function getCourseProgress(slug: string): Promise<CourseProgress> {
  const r = await apiFetch<{ progress: CourseProgress }>(`/api/v1/courses/${slug}/progress`);
  return r.progress;
}
export async function saveCourseProgress(slug: string, input: { current_ordinal: number }): Promise<CourseProgress> {
  const r = await apiFetch<{ progress: CourseProgress }>(`/api/v1/courses/${slug}/progress`, { method: "PUT", body: JSON.stringify(input) });
  return r.progress;
}
export async function answerCourseQuiz(slug: string, body: { stepId: string; interactionId: string; selected: string[]; correct: boolean }): Promise<void> {
  await apiFetch<void>(`/api/v1/courses/${slug}/quiz-answer`, { method: "POST", body: JSON.stringify(body) });
}
export async function getCourseReport(slug: string): Promise<CourseReport> {
  const r = await apiFetch<{ report: CourseReport }>(`/api/v1/courses/${slug}/report`);
  return r.report;
}

// courseAsk mirrors chatTurn (see chat.ts): raw fetch with an SSE Accept
// header, then translate the wire frames (`text`/`error`/`done`) into the
// client's public shape. Wire `text` frames carry `data.delta`; we
// accumulate them into a running `reply` string and yield the accumulated
// body each time, exactly like chatTurn does.
export async function* courseAsk(slug: string, input: string, ordinal: number): AsyncGenerator<CourseAskEvent> {
  const res = await fetch(`${API_BASE}/api/v1/courses/${slug}/ask`, {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json", Accept: "text/event-stream" },
    body: JSON.stringify({ input, ordinal }),
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
      case "done": break;
      case "error": yield { type: "error", code: data.error?.code ?? "internal_error", message: data.error?.message ?? "" }; break;
    }
  }
}
