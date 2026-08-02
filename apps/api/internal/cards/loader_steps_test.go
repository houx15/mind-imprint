package cards

import "testing"

func TestSpecParsesStepsAndFields(t *testing.T) {
	s, ok := ByID("money-trail")
	if !ok {
		t.Fatal("money-trail not found")
	}
	if s.InteractionType != "画布导图卡" {
		t.Fatalf("interaction_type = %q", s.InteractionType)
	}
	if len(s.Steps) != 1 {
		t.Fatalf("steps = %d, want 1", len(s.Steps))
	}
	main := s.Steps[0]
	if main.Key != "main" || main.Title != "资金链溯源" {
		t.Fatalf("step 0 = %q / %q", main.Key, main.Title)
	}
	if len(main.Fields) != 4 {
		t.Fatalf("main fields = %d, want 4", len(main.Fields))
	}
	// field 0 is the claim text field
	if main.Fields[0].Key != "claim" || main.Fields[0].Type != "text" {
		t.Fatalf("field 0 = %q / %q", main.Fields[0].Key, main.Fields[0].Type)
	}
	if main.Fields[0].Label != "要溯源的说法是什么？" {
		t.Fatalf("field 0 label = %q", main.Fields[0].Label)
	}
	// field 1 is the repeatable_group "chain" with 3 item_fields
	chain := main.Fields[1]
	if chain.Key != "chain" || chain.Type != "repeatable_group" {
		t.Fatalf("field 1 = %q / %q", chain.Key, chain.Type)
	}
	if len(chain.ItemFields) != 3 {
		t.Fatalf("item_fields = %d, want 3", len(chain.ItemFields))
	}
	if chain.ItemFields[0].Key != "who" || chain.ItemFields[0].Label != "节点（谁）" {
		t.Fatalf("item 0 = %q / %q", chain.ItemFields[0].Key, chain.ItemFields[0].Label)
	}
	if chain.ItemFields[2].Label != "它的钱 / 利益从哪来？" {
		t.Fatalf("item 2 label = %q", chain.ItemFields[2].Label)
	}
}
