package api

// atom_report_internal_test.go — validateMoments is unexported, so its test
// lives here (package api), not in atom_report_test.go (package api_test).

import "testing"

func TestValidateMoments(t *testing.T) {
	corpus := "我觉得人均排放更能说明责任。\n作者只讲了总量，没有讲人口。"
	got := validateMoments([]reportMoment{
		{Quote: "人均排放更能说明责任", Where: "我的收获"},
		{Quote: "她展现了批判性思维", Where: "我的收获"}, // AI's own prose — dropped
		{Quote: "中国碳排放世界第一", Where: "批注"},   // the article's — dropped
		{Quote: "  ", Where: "批注"},          // empty — dropped
	}, corpus)
	if len(got) != 1 {
		t.Fatalf("kept %d, want 1: %+v", len(got), got)
	}
	if got[0].Quote != "人均排放更能说明责任" {
		t.Errorf("kept the wrong moment: %+v", got[0])
	}
}
