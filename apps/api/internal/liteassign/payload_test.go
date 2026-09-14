package liteassign

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestValidatePayload(t *testing.T) {
	ok := []struct{ kind, raw string }{
		{"reading", `{"source":"library","slug":"coffee","tier":3}`},
		{"reading", `{"source":"library","slug":"coffee"}`},
		{"reading", `{"source":"url","url":"https://example.com/a"}`},
		{"reading", `{"source":"text","text":"一段文章"}`},
		{"writing", `{"prompt":"写一篇关于雨的记叙文","targetWords":800,"lang":"zh"}`},
		{"project", `{"drivingQuestion":"怎样让校园少用一次性杯子？","description":""}`},
	}
	for _, c := range ok {
		if _, err := ValidatePayload(c.kind, json.RawMessage(c.raw)); err != nil {
			t.Errorf("%s %s: unexpected %v", c.kind, c.raw, err)
		}
	}
	bad := []struct{ kind, raw, code string }{
		{"reading", `{"source":"library","slug":""}`, "invalid_slug"},
		{"reading", `{"source":"library","slug":"coffee","tier":6}`, "invalid_tier"},
		{"reading", `{"source":"url","url":"ftp://x"}`, "invalid_url"},
		{"reading", `{"source":"text","text":"   "}`, "empty_text"},
		{"reading", `{"source":"pdf"}`, "invalid_source"},
		{"writing", `{"prompt":"","targetWords":800,"lang":"zh"}`, "empty_prompt"},
		{"writing", `{"prompt":"x","targetWords":0,"lang":"zh"}`, "invalid_target_words"},
		{"writing", `{"prompt":"x","targetWords":800,"lang":"fr"}`, "invalid_lang"},
		{"project", `{"drivingQuestion":"  "}`, "empty_driving_question"},
		{"quiz", `{}`, "invalid_kind"},
		{"writing", `not json`, "invalid_payload"},
	}
	for _, c := range bad {
		_, err := ValidatePayload(c.kind, json.RawMessage(c.raw))
		var pe *PayloadError
		if !errors.As(err, &pe) || pe.Code != c.code {
			t.Errorf("%s %s: got %v, want code %s", c.kind, c.raw, err, c.code)
		}
	}
}

// TestValidateInstructions: the cap counts runes, not bytes. 2000 Chinese
// characters are 6000 bytes and must still pass.
func TestValidateInstructions(t *testing.T) {
	got, err := ValidateInstructions("  " + strings.Repeat("雨", 2000) + "  ")
	if err != nil || got != strings.Repeat("雨", 2000) {
		t.Fatalf("2000 runes: got len %d err %v, want trimmed and accepted", len([]rune(got)), err)
	}
	_, err = ValidateInstructions(strings.Repeat("雨", 2001))
	var pe *PayloadError
	if !errors.As(err, &pe) || pe.Code != "instructions_too_long" || pe.Message != "说明不能超过 2000 字" {
		t.Fatalf("2001 runes: got %v, want instructions_too_long", err)
	}
}

func TestValidatePayloadCanonicalises(t *testing.T) {
	out, err := ValidatePayload("writing", json.RawMessage(`{"prompt":"  写雨  ","targetWords":800,"lang":"zh","extra":1}`))
	if err != nil {
		t.Fatal(err)
	}
	var w WritingPayload
	_ = json.Unmarshal(out, &w)
	if w.Prompt != "写雨" {
		t.Fatalf("prompt not trimmed: %q", w.Prompt)
	}
	if string(out) == `{"prompt":"  写雨  ","targetWords":800,"lang":"zh","extra":1}` {
		t.Fatal("unknown field kept")
	}
}
