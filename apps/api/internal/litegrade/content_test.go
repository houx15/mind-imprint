package litegrade

import (
	"errors"
	"strings"
	"testing"

	"mindimprint/api/internal/liteassign"
)

func TestParse(t *testing.T) {
	fenced := "```json\n{\"overall\":{\"grade\":\"B\",\"comment\":\"x\"},\"dimensions\":[],\"points\":[{\"kind\":\"good\",\"quote\":\"a\",\"text\":\"b\",\"action\":null}]}\n```"
	c, err := Parse(fenced)
	if err != nil || c.Overall.Grade != "B" || len(c.Points) != 1 || c.Points[0].Action != nil {
		t.Fatalf("fenced = %+v err=%v", c, err)
	}
	for _, s := range []string{"", "抱歉，我无法批改。", "{not json}", "{\"overall\": 3}"} {
		if _, err := Parse(s); !errors.Is(err, ErrUnparseable) {
			t.Errorf("Parse(%q) err = %v, want ErrUnparseable", s, err)
		}
	}
	// A decode error keeps encoding/json's message for the server log.
	if _, err := Parse(`{"overall": 3}`); !errors.Is(err, ErrUnparseable) || !strings.Contains(err.Error(), "cannot unmarshal") {
		t.Errorf("decode error = %v, want ErrUnparseable with the json message", err)
	}
}

func TestNormalize(t *testing.T) {
	r := liteassign.DefaultRubric("zh")
	c := Content{
		Overall: Overall{Grade: " B ", Comment: " 好 "},
		Dimensions: []Dimension{
			{Name: "书写规范", Grade: "A"}, {Name: "语言", Grade: "A"}, {Name: "结构", Grade: "B"}, {Name: "内容", Grade: "B"},
		},
		Points: []Point{
			{Kind: "good", Quote: sp(" 去年秋天 "), Text: " 具体 ", Action: sp("不该有"), Source: "teacher"},
			{Kind: "issue", Quote: sp("  "), Text: "x", Action: sp("  ")},
		},
	}
	ai := NormalizeAI(c, r)
	if ai.Overall.Grade != "B" || ai.Dimensions[0].Name != "内容" || ai.Dimensions[3].Name != "书写规范" {
		t.Fatalf("overall/dimension order = %+v", ai)
	}
	if ai.Points[0].Action != nil || *ai.Points[0].Quote != "去年秋天" || ai.Points[0].Source != SourceAI {
		t.Fatalf("good point = %+v", ai.Points[0])
	}
	if ai.Points[1].Quote != nil || ai.Points[1].Action != nil {
		t.Fatalf("blank quote/action must become nil: %+v", ai.Points[1])
	}
	teacher := NormalizeTeacher(Content{Points: []Point{{Kind: "issue", Text: "a", Source: "ai"}, {Kind: "issue", Text: "b"}}}, r)
	if teacher.Points[0].Source != SourceAI || teacher.Points[1].Source != SourceTeacher {
		t.Fatalf("teacher sources = %+v", teacher.Points)
	}
	if teacher.Dimensions == nil {
		t.Fatal("dimensions must marshal as [] not null")
	}
}

// Live, 2026-09-17: the model wrote each dimension as 「名称：说明」 on both
// attempts and the grading failed.
func TestNormalizeAIRepairsDimensionNamesWithTheirNote(t *testing.T) {
	r := liteassign.DefaultRubric("zh")
	c := Content{Dimensions: []Dimension{
		{Name: "内容：立意是否明确，材料是否支撑观点", Grade: "B"},
		{Name: "结构（段落顺序是否清楚）", Grade: "B"},
		{Name: "语言", Grade: "B"},
		{Name: "书写规范：标点、错别字与格式", Grade: "B"},
	}}
	got := NormalizeAI(c, r)
	for i, want := range []string{"内容", "结构", "语言", "书写规范"} {
		if got.Dimensions[i].Name != want {
			t.Fatalf("dimension %d = %q, want %q", i, got.Dimensions[i].Name, want)
		}
	}
	// A teacher's edit is not repaired: she sees the names she saved.
	if n := NormalizeTeacher(c, r).Dimensions[0].Name; n != "内容：立意是否明确，材料是否支撑观点" {
		t.Fatalf("teacher dimension = %q", n)
	}
	// A name that only shares a prefix with no separator stays as it is.
	if n := rubricDimensionName("内容丰富", r); n != "内容丰富" {
		t.Fatalf("prefix without a separator = %q", n)
	}
}
