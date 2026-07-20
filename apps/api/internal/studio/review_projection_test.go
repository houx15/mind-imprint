package studio

import (
	"testing"

	"mindimprint/api/internal/skills"
	"mindimprint/api/internal/store/sqlc"
)

func testSkill(t *testing.T) skills.Skill {
	sk, ok := skills.ByID("writing-project")
	if !ok {
		t.Fatal("writing-project skill missing")
	}
	return sk
}

func TestProjectSelfScore_LatestWinsAndUnpicked(t *testing.T) {
	sk := testSkill(t)
	d := ProjectData{Nodes: []sqlc.GraphNode{
		{Type: "self_score", Body: []byte(`{"scores":[{"code":"表D","band":0}]}`)},
		{Type: "self_score", Body: []byte(`{"scores":[{"code":"表D","band":2},{"code":"表E","band":1}]}`)},
	}}
	ss := projectSelfScore(sk, d)
	if len(ss.Dims) != len(sk.ReviewCriteria) {
		t.Fatalf("dims = %d, want %d", len(ss.Dims), len(sk.ReviewCriteria))
	}
	byCode := map[string]int{}
	for _, dim := range ss.Dims {
		byCode[dim.Code] = dim.Band
	}
	if byCode["表D"] != 2 { // latest node wins
		t.Errorf("表D band = %d, want 2", byCode["表D"])
	}
	if byCode["表F"] != -1 { // never picked
		t.Errorf("表F band = %d, want -1 (unpicked)", byCode["表F"])
	}
	if len(ss.Bands) != 3 {
		t.Errorf("bands = %v, want 3 labels", ss.Bands)
	}
}

func TestProjectReflection_LatestText(t *testing.T) {
	d := ProjectData{Nodes: []sqlc.GraphNode{
		{Type: "reflection", Body: []byte(`{"text":"第一版反思"}`)},
		{Type: "reflection", Body: []byte(`{"text":"最终反思"}`)},
	}}
	rf := projectReflection(d)
	if rf.Text != "最终反思" {
		t.Errorf("text = %q, want 最终反思 (latest)", rf.Text)
	}
	if len(rf.Prompts) == 0 {
		t.Error("prompts empty")
	}
}
