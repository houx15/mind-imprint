import { z } from "zod";

// The four REQUIRED kick-off dimensions of a project plus the optional 5th
// section (counterpoints/反例·张力). Server-projected onto WorkspaceProjection;
// zero-value is all-"" when the student has not started forming yet. Only the
// four required dims gate plan generation; counterpoints never does.
export const Proposal = z.object({
  objective: z.string(),
  reason: z.string(),
  activities: z.string(),
  resources: z.string(),
  counterpoints: z.string().default(""),
});
export type Proposal = z.infer<typeof Proposal>;
