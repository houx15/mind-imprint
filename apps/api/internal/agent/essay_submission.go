package agent

// essay_submission.go — slice 4c · the essay SUBMISSION stage's guide-step track
// (§6 stage-3 · submission complete). After the statement stage built the argument
// body, the student assembles and finishes the whole paper:
//   引言 (intro) → 结论 (conclusion) → 成文 (compose) → 润色 (polish loop) → 完成.
// The steps are FIXED (no per-claim fan-out — the claims are done). Compose +
// polish are driven by the writing surface (assemble the buffer + the whole-draft
// review loop); intro/conclusion are GuidedWritingCards. Keys are namespaced
// `sub:*` so they never collide with the statement step keys in StepGuides.

// DeriveSubmissionSteps builds the submission-stage step list.
func DeriveSubmissionSteps() []Step {
	return []Step{
		{Key: "sub:intro", Title: "引言", Kind: KindFixed},
		{Key: "sub:conclusion", Title: "结论", Kind: KindFixed},
		{Key: "sub:compose", Title: "成文", Kind: KindFixed},
		{Key: "sub:polish", Title: "润色定稿", Kind: KindFixed},
	}
}
