package studio

import (
	"testing"

	"mindimprint/api/internal/store/sqlc"
)

func TestProjectOnboarding_ReadsAssignmentAndLatestRestate(t *testing.T) {
	// ProjectData.Nodes is []sqlc.GraphNode (projection.go:21); Body is []byte.
	d := ProjectData{Nodes: []sqlc.GraphNode{
		{Type: "assignment_brief", Body: []byte(`{"text":"讨论 X"}`)},
		{Type: "rubric_translation", Body: []byte(`{"restate_prompt":"P","rows":[{"official":"O","plain":"p","weak":true}]}`)},
		{Type: "task_restatement", Body: []byte(`{"restate":"第一版","weak_picks":[0]}`)},
		{Type: "task_restatement", Body: []byte(`{"restate":"最终版","weak_picks":[1,2]}`)},
	}}
	ob := projectOnboarding(d)
	if ob.AssignmentText != "讨论 X" {
		t.Errorf("AssignmentText = %q, want 讨论 X", ob.AssignmentText)
	}
	if ob.StudentRestate != "最终版" {
		t.Errorf("StudentRestate = %q, want 最终版 (latest node wins)", ob.StudentRestate)
	}
	if len(ob.StudentWeakPicks) != 2 || ob.StudentWeakPicks[0] != 1 {
		t.Errorf("StudentWeakPicks = %v, want [1 2]", ob.StudentWeakPicks)
	}
	if ob.RestatePrompt != "P" || len(ob.RubricRows) != 1 {
		t.Errorf("existing rubric fields regressed: %+v", ob)
	}
}
