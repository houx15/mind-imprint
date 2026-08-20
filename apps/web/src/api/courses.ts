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
export async function saveCourseProgress(
  slug: string,
  input: { current_ordinal: number; completed_ordinal?: number; active_seconds_delta?: number },
): Promise<CourseProgress> {
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

// restartCourse wipes the student's progress for one course (both the 2.0
// runtime session and the legacy progress row) so it starts over from the
// beginning. Idempotent server-side; course events are kept (铁律④).
export async function restartCourse(slug: string): Promise<void> {
  await apiFetch<{ ok: true }>(`/api/v1/courses/${slug}/restart`, { method: "POST" });
}

/** One touched course in the student's learning history. status is the raw
 * runtime session status (created/opening/in-progress/closing/completed) or
 * 'completed'/'in-progress' for a legacy course. completedCount is the number of
 * completed steps for BOTH storages — a runtime course counts its completed
 * slices from the session (clamped to step_count when finished), a legacy course
 * counts its completed ordinals. */
export interface CourseHistoryItem {
  slug: string;
  status: string;
  completedCount: number;
  updatedAt: string;
}

// getCourseHistory lists the courses the student has engaged with, newest
// activity first. Title/cover are enriched client-side from listCourses().
export async function getCourseHistory(): Promise<CourseHistoryItem[]> {
  const r = await apiFetch<{ items: CourseHistoryItem[] }>(`/api/v1/courses/history`);
  return r.items;
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
