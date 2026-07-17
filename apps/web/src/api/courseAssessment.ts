import { Assessment } from "@mind-imprint/contracts";
import { apiFetch } from "./client";

// A1: the course session's report. Scoped by course id — the server resolves
// the session from (caller, course_id), so no session id appears in the URL
// (IDOR closed by construction, mirroring every other course route).
export async function getCourseAssessment(courseId: string): Promise<Assessment | null> {
  const raw = await apiFetch<unknown>(`/api/v1/courses/${courseId}/session/assessment`);
  if (raw == null) return null;
  return Assessment.parse(raw);
}

export async function generateCourseAssessment(courseId: string): Promise<Assessment> {
  const raw = await apiFetch<unknown>(`/api/v1/courses/${courseId}/session/assessment`, { method: "POST" });
  return Assessment.parse(raw);
}
