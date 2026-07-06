package agent

import (
	"strings"
	"testing"
)

func TestBuildMaterialContextIncludesBlockIDs(t *testing.T) {
	mats := []Material{
		{ID: "m1", Title: "卫星图看中国变绿", Blocks: []MaterialBlock{
			{ID: "b0", Text: "最近一张 NASA 卫星图刷屏。"},
			{ID: "b1", Text: "文章称地球绿了 5%。"},
		}},
	}
	out := BuildMaterialContext(mats)
	for _, want := range []string{"卫星图看中国变绿", "b0", "b1", "地球绿了 5%", "最近一张 NASA"} {
		if !strings.Contains(out, want) {
			t.Errorf("material context missing %q\n---\n%s", want, out)
		}
	}
}

func TestBuildMaterialContextEmpty(t *testing.T) {
	if BuildMaterialContext(nil) != "" {
		t.Fatal("no materials should yield empty context")
	}
}
