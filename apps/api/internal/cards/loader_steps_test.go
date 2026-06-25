package cards

import "testing"

func TestSpecParsesStepsAndFields(t *testing.T) {
	s, ok := ByID("sift_craap")
	if !ok {
		t.Fatal("sift_craap not found")
	}
	if s.InteractionType != "步骤引导卡" {
		t.Fatalf("interaction_type = %q", s.InteractionType)
	}
	if len(s.Steps) != 2 {
		t.Fatalf("steps = %d, want 2", len(s.Steps))
	}
	sift := s.Steps[0]
	if sift.Key != "sift" || sift.Title != "SIFT · 横向找更多来源" {
		t.Fatalf("step 0 = %q / %q", sift.Key, sift.Title)
	}
	if len(sift.Fields) != 4 {
		t.Fatalf("sift fields = %d, want 4", len(sift.Fields))
	}
	// field 0 is the Stop textarea
	if sift.Fields[0].Key != "stop" || sift.Fields[0].Type != "textarea" {
		t.Fatalf("field 0 = %q / %q", sift.Fields[0].Key, sift.Fields[0].Type)
	}
	if sift.Fields[0].Label != "Stop：你打算用这条信息说明什么？" {
		t.Fatalf("field 0 label = %q", sift.Fields[0].Label)
	}
	// field 1 is the repeatable_group "sources" with 3 item_fields
	sources := sift.Fields[1]
	if sources.Key != "sources" || sources.Type != "repeatable_group" {
		t.Fatalf("field 1 = %q / %q", sources.Key, sources.Type)
	}
	if len(sources.ItemFields) != 3 {
		t.Fatalf("item_fields = %d, want 3", len(sources.ItemFields))
	}
	if sources.ItemFields[0].Key != "name" || sources.ItemFields[0].Label != "来源" {
		t.Fatalf("item 0 = %q / %q", sources.ItemFields[0].Key, sources.ItemFields[0].Label)
	}
	if sources.ItemFields[2].Label != "可信？" {
		t.Fatalf("item 2 label = %q", sources.ItemFields[2].Label)
	}
}
