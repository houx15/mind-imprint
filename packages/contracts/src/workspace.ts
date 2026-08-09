import { z } from "zod";
import { Proposal } from "./proposal";

// A project's lifecycle status, derived server-side: forming (kickoff not yet
// shaped) → working (proposal/plan under way) → evaluating (finish clicked,
// the process assessment is generating) → done (assessment ready).
export const ProjectStatus = z.enum(["forming", "working", "evaluating", "done"]);
export type ProjectStatus = z.infer<typeof ProjectStatus>;

// The GET /projects/{id} projection — lean by design; each room fetches its own
// data. Carries only what the shell needs: identity + status + the four dims.
export const WorkspaceProjection = z.object({
  id: z.string(),
  title: z.string(),
  qualification: z.string(),
  status: ProjectStatus,
  proposal: Proposal,
  // RFC3339 project creation time — anchors the plan timeline to calendar dates
  // (#14). Optional so older mocks without it still parse; client falls back to now.
  createdAt: z.string().optional(),
  // #20 · the ESSAY's 完成写作 milestone: true once the final paper is locked
  // read-only and the 回顾 room is unlocked. Drives ReviewBlock's view-only gate.
  // Optional (defaults false) so older mocks/responses still parse.
  writingFinished: z.boolean().optional().default(false),
  // Phase B · per-document 完成写作 state. WritingBlock reads the entry for its
  // ACTIVE doc to lock that document, so the proposal and essay lock
  // independently. Optional (defaults both-false) so older mocks still parse.
  writingFinish: z
    .object({ proposal: z.boolean(), essay: z.boolean() })
    .optional()
    .default({ proposal: false, essay: false }),
});
export type WorkspaceProjection = z.infer<typeof WorkspaceProjection>;
