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
});
export type WorkspaceProjection = z.infer<typeof WorkspaceProjection>;
