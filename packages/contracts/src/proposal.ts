import { z } from "zod";

// The four kick-off dimensions of a project (replaces the old onboarding/
// framing/perspectives kickoff). Server-projected onto WorkspaceProjection;
// zero-value is {"","","",""} when the student has not started forming yet.
export const Proposal = z.object({
  objective: z.string(),
  reason: z.string(),
  activities: z.string(),
  resources: z.string(),
});
export type Proposal = z.infer<typeof Proposal>;
