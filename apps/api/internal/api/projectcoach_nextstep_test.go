package api

import (
	"testing"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/store/sqlc"
)

// This test lives in package `api` (not api_test) so it can reach the unexported
// nextPlanStep helper directly.
func TestNextPlanStep(t *testing.T) {
	items := []sqlc.PlanItem{
		{Title: "撰写研究提案：明确研究问题、文献范围与计划", Tag: "write", Col: "todo", Stage: "阶段一 · 提案", Position: 0},
		{Title: "阅读 Baddeley 工作记忆模型核心文献", Tag: "read", Col: "todo", Stage: "阶段二 · 研究", Position: 1},
		{Title: "撰写正文初稿", Tag: "write", Col: "todo", Stage: "阶段三 · 写作", Position: 5},
	}

	step, ok := nextPlanStep(items)
	if !ok {
		t.Fatal("expected a next step when todos exist")
	}
	if step.tool != "writing" {
		t.Errorf("write task should map to writing room, got %q", step.tool)
	}
	if step.title == "" || step.stageLabel != "阶段一 · 提案" {
		t.Errorf("expected first-position task surfaced, got title=%q stage=%q", step.title, step.stageLabel)
	}

	// A read task first → reading room.
	itemsRead := []sqlc.PlanItem{
		{Title: "阅读文献", Tag: "read", Col: "todo", Stage: "阶段二", Position: 0},
	}
	if step, _ := nextPlanStep(itemsRead); step.tool != "reading" {
		t.Errorf("read task should map to reading room, got %q", step.tool)
	}

	// The first not-done task is chosen even when an earlier-position task is done.
	itemsDone := []sqlc.PlanItem{
		{Title: "已完成的提案", Tag: "write", Col: "done", Stage: "阶段一", Position: 0},
		{Title: "读文献", Tag: "read", Col: "todo", Stage: "阶段二", Position: 1},
	}
	step2, ok2 := nextPlanStep(itemsDone)
	if !ok2 || step2.title != "读文献" {
		t.Errorf("expected first not-done task, got ok=%v title=%q", ok2, step2.title)
	}

	// All done → no step.
	allDone := []sqlc.PlanItem{{Title: "x", Tag: "write", Col: "done", Position: 0}}
	if _, ok := nextPlanStep(allDone); ok {
		t.Error("expected no step when every task is done")
	}
}

func TestNextStepFor(t *testing.T) {
	// framework: plan not yet → no step; plan exists → offer proposal.
	if s := nextStepFor(agent.FlowFramework, false, false); s != nil {
		t.Errorf("framework w/o plan should offer nothing, got %+v", s)
	}
	s := nextStepFor(agent.FlowFramework, true, false)
	if s == nil || s.ToStatus != "proposal" || s.Surface != "writing" {
		t.Errorf("framework w/ plan should offer proposal→writing, got %+v", s)
	}
	// proposal: only on finish_part → offer essay.
	if s := nextStepFor(agent.FlowProposal, true, false); s != nil {
		t.Errorf("proposal w/o finish should offer nothing, got %+v", s)
	}
	// slice 4a · 完成提案 → research in the reading room first (not the writing surface).
	if s := nextStepFor(agent.FlowProposal, true, true); s == nil || s.ToStatus != "essay" || s.Surface != "reading" {
		t.Errorf("proposal w/ finish should offer essay→reading (research), got %+v", s)
	}
	// essay: only on finish_part → offer review.
	if s := nextStepFor(agent.FlowEssay, true, true); s == nil || s.ToStatus != "review" || s.Surface != "reflection" {
		t.Errorf("essay w/ finish should offer review→reflection, got %+v", s)
	}
	// review: never offers.
	if s := nextStepFor(agent.FlowReview, true, true); s != nil {
		t.Errorf("review should offer nothing, got %+v", s)
	}
}
