package agent

import "testing"

func TestParseSearchGuidance(t *testing.T) {
	out, err := parseSearchGuidance(`{"suggestions":[{"keyword":"solar capacity China","why":"补子问题一的支持证据"},{"keyword":"","why":"skip"},{"keyword":"coal share trend","why":"看反例"}]}`)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(out) != 2 || out[0].Keyword != "solar capacity China" {
		t.Fatalf("parsed wrong: %+v", out)
	}
	if _, err := parseSearchGuidance("nope"); err == nil {
		t.Fatal("expected error for no json")
	}
	if _, err := parseSearchGuidance(`{"suggestions":[]}`); err == nil {
		t.Fatal("expected error for empty")
	}
}
