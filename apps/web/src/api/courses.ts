import type { Course, CourseSummary, CourseProgress, RenderedStep } from "@mind-imprint/contracts";
import { apiFetch } from "./client";

export async function listCourses(): Promise<CourseSummary[]> {
  const r = await apiFetch<{ courses: CourseSummary[] }>("/api/v1/courses");
  return r.courses;
}
export async function getCourse(id: string): Promise<Course> {
  const r = await apiFetch<{ course: Course }>(`/api/v1/courses/${id}`);
  return r.course;
}
export async function getCourseProgress(id: string): Promise<CourseProgress> {
  const r = await apiFetch<{ progress: CourseProgress }>(`/api/v1/courses/${id}/progress`);
  return r.progress;
}
// completed_ordinals is intentionally ABSENT from this write-side type: the
// server (course.go's putCourseProgress) only ever accepts current_ordinal —
// completed_ordinals is the steps_viewed floor's input and is written solely
// by the render handler (course_render.go) when a step is actually opened.
// It still appears on the READ side (CourseProgress, from getCourseProgress)
// for resume + the progress-bar stepper.
export async function saveCourseProgress(id: string, input: { current_ordinal: number }): Promise<CourseProgress> {
  const r = await apiFetch<{ progress: CourseProgress }>(`/api/v1/courses/${id}/progress`, { method: "PUT", body: JSON.stringify(input) });
  return r.progress;
}
export async function renderCourseStep(courseId: string, ordinal: number): Promise<RenderedStep> {
  const r = await apiFetch<{ rendered: RenderedStep }>(`/api/v1/courses/${courseId}/steps/${ordinal}/render`, { method: "POST" });
  return r.rendered;
}
