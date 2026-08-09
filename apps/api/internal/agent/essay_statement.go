package agent

// essay_statement.go — slice 4b · the essay statement stage's guide-step track.
// The steps are DERIVED from the sub-questions (like the proposal's research
// plan), but the shape is the paper's, not the proposal's (§6 stage-2):
//   outline → one claim per sub-question → 比较/综合 → 面对反方观点 → 结论 → 论证结构.
// 反方观点 is a DEDICATED step (user) so it isn't skipped. Everything here is
// pure; guide-card generation lives in proposal_guide.go (doc-aware).

// EssayStatementFixedTail are the fixed steps after the per-claim steps.
func essayStatementTail() []Step {
	return []Step{
		{Key: "synthesis", Title: "比较 / 综合", Kind: KindFixed},
		{Key: "challenges", Title: "面对反方观点", Kind: KindFixed},
		{Key: "conclusion", Title: "结论", Kind: KindFixed},
		{Key: "structure", Title: "论证结构", Kind: KindFixed},
	}
}

// DeriveStatementSteps builds the statement-stage step list: the outline step,
// one claim step per sub-question (Key "claim:<id>"), then the fixed tail.
func DeriveStatementSteps(subQuestions []SubQuestion) []Step {
	out := make([]Step, 0, 2+len(subQuestions)+4)
	out = append(out, Step{Key: "outline", Title: "搭大纲", Kind: KindFixed})
	for i, sq := range subQuestions {
		out = append(out, Step{
			Key: "claim:" + sq.ID, Title: "论点 " + itoa(i+1), Kind: KindSubq, SubQuestionID: sq.ID,
		})
	}
	out = append(out, essayStatementTail()...)
	return out
}
