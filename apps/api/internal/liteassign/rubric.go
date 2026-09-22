package liteassign

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Rubric is a writing homework's 评分标准. AI 批改 grades against it, and each
// grading row keeps a copy of the rubric it used.
type Rubric struct {
	Scale      string            `json:"scale"`
	Max        int               `json:"max,omitempty"`
	Dimensions []RubricDimension `json:"dimensions"`
	Focus      string            `json:"focus"`
}

type RubricDimension struct {
	Name string `json:"name"`
	Note string `json:"note"`
}

const (
	ScaleLetter = "letter"
	ScalePoints = "points"

	maxRubricDimensions   = 6
	maxDimensionNameRunes = 40 // the English defaults are up to 30 characters
	maxDimensionNoteRunes = 200
	maxRubricFocusRunes   = 500
	maxRubricPoints       = 100
)

// LetterGrades is the letter scale, highest first.
var LetterGrades = []string{"A+", "A", "A-", "B+", "B", "B-", "C+", "C", "C-", "D"}

// DefaultRubric is the rubric a writing homework without one is graded with.
func DefaultRubric(lang string) Rubric {
	if lang == "en" {
		return Rubric{Scale: ScaleLetter, Dimensions: []RubricDimension{
			{Name: "Task Response", Note: "是否回应题目的全部要求，观点是否展开并有依据"},
			{Name: "Coherence and Cohesion", Note: "段落安排是否清楚，句与句、段与段之间是否衔接"},
			{Name: "Lexical Resource", Note: "用词是否准确、多样，搭配是否得当"},
			{Name: "Grammatical Range and Accuracy", Note: "句式是否多样，语法是否准确"},
		}}
	}
	return Rubric{Scale: ScaleLetter, Dimensions: []RubricDimension{
		{Name: "内容", Note: "立意是否明确，材料是否能说明观点"},
		{Name: "结构", Note: "段落顺序是否清楚，段与段之间是否衔接"},
		{Name: "语言", Note: "表达是否准确、通顺"},
		{Name: "书写规范", Note: "标点、错别字与格式"},
	}}
}

// ValidateRubric returns a trimmed copy, or a PayloadError naming the first problem.
func ValidateRubric(r Rubric) (Rubric, error) {
	out := Rubric{Scale: strings.TrimSpace(r.Scale), Focus: strings.TrimSpace(r.Focus)}
	switch out.Scale {
	case ScaleLetter:
	case ScalePoints:
		if r.Max < 1 || r.Max > maxRubricPoints {
			return Rubric{}, perr("invalid_rubric_max", "满分需在 1 到 100 之间")
		}
		out.Max = r.Max
	default:
		return Rubric{}, perr("invalid_rubric_scale", "评分方式只能是等级或分数")
	}
	if len(r.Dimensions) < 1 || len(r.Dimensions) > maxRubricDimensions {
		return Rubric{}, perr("invalid_rubric_dimensions", "评分维度需有 1 到 6 项")
	}
	seen := make(map[string]bool, len(r.Dimensions))
	out.Dimensions = make([]RubricDimension, 0, len(r.Dimensions))
	for _, d := range r.Dimensions {
		name, note := strings.TrimSpace(d.Name), strings.TrimSpace(d.Note)
		if name == "" || utf8.RuneCountInString(name) > maxDimensionNameRunes {
			return Rubric{}, perr("invalid_rubric_dimension_name", "维度名称不能为空，不超过 40 字")
		}
		// Check matches the model's dimensions by name, so names must be unique.
		if seen[name] {
			return Rubric{}, perr("duplicate_rubric_dimension", "维度名称不能重复")
		}
		seen[name] = true
		if utf8.RuneCountInString(note) > maxDimensionNoteRunes {
			return Rubric{}, perr("invalid_rubric_note", "维度说明不超过 200 字")
		}
		out.Dimensions = append(out.Dimensions, RubricDimension{Name: name, Note: note})
	}
	if utf8.RuneCountInString(out.Focus) > maxRubricFocusRunes {
		return Rubric{}, perr("invalid_rubric_focus", "批改重点不超过 500 字")
	}
	return out, nil
}

// ParseRubric decodes and validates a rubric sent by the teacher.
func ParseRubric(raw json.RawMessage) (Rubric, error) {
	var r Rubric
	if err := json.Unmarshal(raw, &r); err != nil {
		return Rubric{}, perr("invalid_rubric", "评分标准格式错误")
	}
	return ValidateRubric(r)
}

// EffectiveRubric is the rubric a writing homework is graded with: the stored
// one, or the default for the payload's language (homework created before
// rubrics existed has none).
func EffectiveRubric(payload json.RawMessage) Rubric {
	var p WritingPayload
	if err := json.Unmarshal(payload, &p); err == nil && p.Rubric != nil {
		return *p.Rubric
	}
	return DefaultRubric(p.Lang)
}

// GradeInScale: a letter from LetterGrades, or an integer 0..Max written
// without leading zeros.
func GradeInScale(r Rubric, grade string) bool {
	switch r.Scale {
	case ScaleLetter:
		for _, g := range LetterGrades {
			if grade == g {
				return true
			}
		}
	case ScalePoints:
		n, err := strconv.Atoi(grade)
		return err == nil && strconv.Itoa(n) == grade && n >= 0 && n <= r.Max
	}
	return false
}

// CarryRubric puts the stored payload's rubric onto the next payload. PATCH
// uses it so the rubric changes only through the top-level rubric field.
func CarryRubric(next, stored json.RawMessage) (json.RawMessage, error) {
	var p, old WritingPayload
	if json.Unmarshal(next, &p) != nil || json.Unmarshal(stored, &old) != nil {
		return nil, perr("invalid_payload", "作业设置格式错误")
	}
	p.Rubric = old.Rubric
	return json.Marshal(p)
}

// ApplyRubric sets the rubric on a writing payload; a JSON null removes it,
// so the homework reads as the default again.
func ApplyRubric(payload, raw json.RawMessage) (json.RawMessage, error) {
	var p WritingPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil, perr("invalid_payload", "作业设置格式错误")
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		p.Rubric = nil
	} else {
		r, err := ParseRubric(raw)
		if err != nil {
			return nil, err
		}
		p.Rubric = &r
	}
	return json.Marshal(p)
}
