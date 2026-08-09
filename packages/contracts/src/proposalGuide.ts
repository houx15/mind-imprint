import { z } from "zod";

// proposalGuide.ts — slice 3a · the proposal-writing guide-step track wire
// shapes. The track is a dimension of the proposal document (free/guided); in
// guided mode the server derives a dynamic step list (all-statuses.md §4): the
// research plan is a "define 2–4 sub-questions" step followed by ONE card per
// sub-question. `GET /projects/{id}/proposal-track` returns a ProposalGuideStep.

// SubQuestion — one of the 2–4 sub-questions the student decomposes the key
// research question into. `id` is minted server-side; `text` is the student's own.
export const SubQuestion = z.object({ id: z.string(), text: z.string() });
export type SubQuestion = z.infer<typeof SubQuestion>;

// GuideCard — the AI-generated (fast model) scaffold for one step: a guiding
// question applied to the current prompt + an English example (for a DIFFERENT
// prompt, never the answer — 铁律①) + an optional pointer into the framework.
export const GuideCard = z.object({
  prompt: z.string(),
  example: z.string(),
  refHint: z.string().optional(),
});
export type GuideCard = z.infer<typeof GuideCard>;

// StepKind — fixed golden-standard part | the sub-question define step | a
// per-sub-question card.
export const StepKind = z.enum(["fixed", "subq-define", "subq"]);
export type StepKind = z.infer<typeof StepKind>;

// ProposalGuideStep — the whole track state + the current step. `card` is null
// until guided AND started (and is generated lazily for the current step).
// StepRef — one entry in the ordered step list (slice 4b: lets the guided
// surface assemble the per-part text back into the document in order).
export const StepRef = z.object({ key: z.string(), title: z.string(), kind: StepKind });
export type StepRef = z.infer<typeof StepRef>;

export const ProposalGuideStep = z.object({
  key: z.string(),
  title: z.string(),
  kind: StepKind,
  index: z.number().int(),
  total: z.number().int(),
  mode: z.enum(["", "free", "guided"]),
  started: z.boolean(),
  subQuestions: z.array(SubQuestion),
  card: GuideCard.nullable(),
  // The full ordered step list (optional/defaulted for back-compat with fixtures).
  steps: z.array(StepRef).default([]),
});
export type ProposalGuideStep = z.infer<typeof ProposalGuideStep>;
