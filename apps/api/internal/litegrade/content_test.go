package litegrade

import (
	"errors"
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
