package liteassign

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func codeOf(err error) string {
	var pe *PayloadError
	if errors.As(err, &pe) {
		return pe.Code
	}
	return ""
}

func TestDefaultRubric(t *testing.T) {
	names := func(r Rubric) string {
		out := make([]string, 0, len(r.Dimensions))
		for _, d := range r.Dimensions {
			out = append(out, d.Name)
		}
		return strings.Join(out, "/")
	}
	if got := names(DefaultRubric("zh")); got != "内容/结构/语言/书写规范" {
		t.Fatalf("zh = %s", got)
	}
	if got := names(DefaultRubric("en")); got != "Task Response/Coherence and Cohesion/Lexical Resource/Grammatical Range and Accuracy" {
		t.Fatalf("en = %s", got)
	}
	for _, lang := range []string{"zh", "en", ""} {
		r := DefaultRubric(lang)
		if r.Scale != ScaleLetter {
			t.Fatalf("%q scale = %s", lang, r.Scale)
		}
		// The defaults must pass the same validation a teacher's rubric does.
		if _, err := ValidateRubric(r); err != nil {
			t.Fatalf("default %q invalid: %v", lang, err)
		}
	}
}

func TestValidateRubric(t *testing.T) {
	dims := func(names ...string) []RubricDimension {
		out := make([]RubricDimension, 0, len(names))
		for _, n := range names {
			out = append(out, RubricDimension{Name: n})
		}
		return out
	}
	got, err := ValidateRubric(Rubric{Scale: " points ", Max: 20, Dimensions: []RubricDimension{{Name: " 论证 ", Note: " 看证据 "}}, Focus: " 重点看论证 "})
	if err != nil || got.Scale != ScalePoints || got.Max != 20 || got.Dimensions[0].Name != "论证" || got.Dimensions[0].Note != "看证据" || got.Focus != "重点看论证" {
		t.Fatalf("trimmed = %+v err=%v", got, err)
	}
	if got, _ := ValidateRubric(Rubric{Scale: "letter", Max: 50, Dimensions: dims("内容")}); got.Max != 0 {
		t.Fatalf("letter keeps max %d, want 0", got.Max)
	}

	bad := []struct {
		name string
		r    Rubric
		code string
	}{
		{"scale", Rubric{Scale: "stars", Dimensions: dims("内容")}, "invalid_rubric_scale"},
		{"max zero", Rubric{Scale: "points", Max: 0, Dimensions: dims("内容")}, "invalid_rubric_max"},
		{"max 101", Rubric{Scale: "points", Max: 101, Dimensions: dims("内容")}, "invalid_rubric_max"},
		{"no dimensions", Rubric{Scale: "letter"}, "invalid_rubric_dimensions"},
		{"seven dimensions", Rubric{Scale: "letter", Dimensions: dims("1", "2", "3", "4", "5", "6", "7")}, "invalid_rubric_dimensions"},
		{"blank name", Rubric{Scale: "letter", Dimensions: dims("  ")}, "invalid_rubric_dimension_name"},
		{"41-rune name", Rubric{Scale: "letter", Dimensions: dims(strings.Repeat("字", 41))}, "invalid_rubric_dimension_name"},
		{"duplicate", Rubric{Scale: "letter", Dimensions: dims("内容", " 内容")}, "duplicate_rubric_dimension"},
		{"long note", Rubric{Scale: "letter", Dimensions: []RubricDimension{{Name: "内容", Note: strings.Repeat("字", 201)}}}, "invalid_rubric_note"},
		{"long focus", Rubric{Scale: "letter", Dimensions: dims("内容"), Focus: strings.Repeat("字", 501)}, "invalid_rubric_focus"},
	}
	for _, c := range bad {
		if _, err := ValidateRubric(c.r); codeOf(err) != c.code {
			t.Errorf("%s: got %v, want %s", c.name, err, c.code)
		}
	}
	if _, err := ParseRubric(json.RawMessage(`{"scale":"points","max":20.5,"dimensions":[{"name":"x"}]}`)); codeOf(err) != "invalid_rubric" {
		t.Fatalf("fractional max = %v, want invalid_rubric", err)
	}
}

func TestGradeInScale(t *testing.T) {
	letter := DefaultRubric("zh")
	for _, g := range []string{"A+", "B-", "D"} {
		if !GradeInScale(letter, g) {
			t.Errorf("letter %q should be in scale", g)
		}
	}
	for _, g := range []string{"", "E", "a", "A++", "90"} {
		if GradeInScale(letter, g) {
			t.Errorf("letter %q should be out of scale", g)
		}
	}
	points := Rubric{Scale: ScalePoints, Max: 20, Dimensions: []RubricDimension{{Name: "论证"}}}
	for _, g := range []string{"0", "13", "20"} {
		if !GradeInScale(points, g) {
			t.Errorf("points %q should be in scale", g)
		}
	}
	for _, g := range []string{"21", "-1", "08", "13.5", "B"} {
		if GradeInScale(points, g) {
			t.Errorf("points %q should be out of scale", g)
		}
	}
}

func TestEffectiveCarryApplyRubric(t *testing.T) {
	noRubric := json.RawMessage(`{"prompt":"写雨","targetWords":800,"lang":"en"}`)
	if r := EffectiveRubric(noRubric); r.Dimensions[0].Name != "Task Response" {
		t.Fatalf("missing rubric reads as the en default, got %+v", r)
	}
	withRubric, err := ApplyRubric(noRubric, json.RawMessage(`{"scale":"points","max":10,"dimensions":[{"name":"论证","note":""}],"focus":""}`))
	if err != nil {
		t.Fatal(err)
	}
	if r := EffectiveRubric(withRubric); r.Scale != ScalePoints || r.Max != 10 {
		t.Fatalf("applied = %+v", r)
	}
	// CarryRubric keeps the stored rubric whatever the next payload says.
	next := json.RawMessage(`{"prompt":"写雨","targetWords":800,"lang":"en","rubric":{"scale":"letter","dimensions":[{"name":"x","note":""}],"focus":""}}`)
	carried, err := CarryRubric(next, withRubric)
	if err != nil {
		t.Fatal(err)
	}
	if r := EffectiveRubric(carried); r.Scale != ScalePoints {
		t.Fatalf("carried = %+v, want the stored points rubric", r)
	}
	reset, err := ApplyRubric(withRubric, json.RawMessage(`null`))
	if err != nil || strings.Contains(string(reset), "rubric") {
		t.Fatalf("null reset = %s err=%v", reset, err)
	}
	if _, err := ApplyRubric(noRubric, json.RawMessage(`{"scale":"points","max":0,"dimensions":[{"name":"x"}]}`)); codeOf(err) != "invalid_rubric_max" {
		t.Fatalf("invalid apply = %v", err)
	}
}

func TestValidatePayloadWritingRubric(t *testing.T) {
	out, err := ValidatePayload("writing", json.RawMessage(`{"prompt":"写雨","targetWords":800,"lang":"zh","rubric":{"scale":"letter","dimensions":[{"name":" 内容 ","note":""}],"focus":""}}`))
	if err != nil || !strings.Contains(string(out), `"name":"内容"`) {
		t.Fatalf("rubric kept and trimmed: %s err=%v", out, err)
	}
	if _, err := ValidatePayload("writing", json.RawMessage(`{"prompt":"写雨","targetWords":800,"lang":"zh","rubric":{"scale":"x","dimensions":[{"name":"内容"}]}}`)); codeOf(err) != "invalid_rubric_scale" {
		t.Fatalf("bad rubric in payload = %v", err)
	}
	out, _ = ValidatePayload("writing", json.RawMessage(`{"prompt":"写雨","targetWords":800,"lang":"zh"}`))
	if strings.Contains(string(out), "rubric") {
		t.Fatalf("no rubric must not add one: %s", out)
	}
}
