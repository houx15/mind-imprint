import type { CourseSession } from "@mind-imprint/course-contract";
import { apiFetch } from "./client";

// courseDefinition.ts — client for the Course Runtime 2.0 surfaces (Slice 8):
// the stored CourseDefinition document and the per-(user,course) CourseSession.
// Envelopes mirror the Go handlers exactly:
//   GET  /courses/{slug}/definition → { definition }         (course_definition.go)
//   POST /courses/{slug}/session    → { session }  get-or-create (course_session.go)
//   PUT  /courses/{slug}/session    → { ok: true }  snapshot-save (course_session.go)
// A course without a 2.0 definition 404s on both the definition and the POST
// (the frontend then routes that course to the legacy player).

/**
 * getCourseDefinition returns the raw stored CourseDefinition 2.0 document
 * (`{ schemaVersion: "2.0", course }`). Throws ApiError(status 404) for a legacy
 * course with no 2.0 definition — the caller branches on that to pick the player.
 * The document is validated deeply by the runtime (validateCourseDefinition), so
 * the shape stays `unknown` here.
 */
export async function getCourseDefinition(slug: string): Promise<unknown> {
  const r = await apiFetch<{ definition: unknown }>(`/api/v1/courses/${slug}/definition`);
  return r.definition;
}

/**
 * createCourseSession get-or-creates the authed student's CourseSession for one
 * course and returns the server's authoritative session (a fresh "created"
 * session, or the existing one on resume). The server mints identity/authorship
 * from the authed user — the client never dictates them.
 */
export async function createCourseSession(slug: string): Promise<CourseSession> {
  const r = await apiFetch<{ session: CourseSession }>(`/api/v1/courses/${slug}/session`, { method: "POST" });
  return r.session;
}

/**
 * saveCourseSession snapshot-writes the whole CourseSession blob (the runtime
 * holds authoritative state client-side; a full snapshot is sufficient). Owner-
 * scoped on the server by the authed user.
 */
export async function saveCourseSession(slug: string, session: CourseSession): Promise<void> {
  await apiFetch<{ ok: true }>(`/api/v1/courses/${slug}/session`, {
    method: "PUT",
    body: JSON.stringify({ session }),
  });
}
