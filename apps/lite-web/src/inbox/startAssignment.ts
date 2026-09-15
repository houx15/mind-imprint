// inbox/startAssignment.ts — where 开始作业 (POST .../start) actually lands
// her. The endpoint's response (see api/assignments.ts's StartAssignmentResult,
// mirroring writeStartResponse in apps/api/internal/api/lite_student_assignments.go)
// names the room by kind; this turns that into the route she is pushed to.

import { liteRoutePath } from "../routing";

export interface StartedAssignment {
  kind: string;
  atomId: string;
  projectId: string | null;
}

/**
 * A project's room is keyed by `projectId`, which is the same value as
 * `atomId` (`pbl_project`'s primary key IS its atom id) — but the response
 * always sends `projectId` explicitly for a project kind, so this reads that
 * field rather than re-deriving it, and only falls back to `atomId` if the
 * server ever sent a project with no id (defensive, not expected on the wire).
 */
export function roomPathForStart(r: StartedAssignment): string {
  if (r.kind === "project") {
    return liteRoutePath({ tab: "projects", projectId: r.projectId ?? r.atomId });
  }
  if (r.kind === "writing") {
    return liteRoutePath({ tab: "writings", writingId: r.atomId });
  }
  return liteRoutePath({ tab: "readings", readingId: r.atomId });
}
