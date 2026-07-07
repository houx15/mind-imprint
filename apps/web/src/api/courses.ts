import type { Course, CourseSummary, CourseProgress } from "@mind-imprint/contracts";
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
export async function saveCourseProgress(id: string, input: { current_ordinal: number; completed_ordinals: number[] }): Promise<CourseProgress> {
  const r = await apiFetch<{ progress: CourseProgress }>(`/api/v1/courses/${id}/progress`, { method: "PUT", body: JSON.stringify(input) });
  return r.progress;
}
