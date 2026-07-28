import { z } from "zod";
import { Proposal } from "./proposal";

// The GET /projects/{id} projection — lean by design; each room fetches its own
// data. Carries only what the shell needs: identity + the four proposal dims.
export const WorkspaceProjection = z.object({
  id: z.string(),
  title: z.string(),
  qualification: z.string(),
  proposal: Proposal,
});
export type WorkspaceProjection = z.infer<typeof WorkspaceProjection>;
