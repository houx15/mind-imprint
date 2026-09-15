// Package litegrade holds the rules of lite AI 批改: the content shape, the
// prompt, and Check. Pure functions: no database, no HTTP. The model call,
// the retry and the grading rows live in internal/api (lite_grading_*.go).
package litegrade

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"mindimprint/api/internal/liteassign"
)

const (
	KindGood      = "good"
	KindIssue     = "issue"
	SourceAI      = "ai"
	SourceTeacher = "teacher"
	MinPoints     = 3
	MaxPoints     = 5
)

// Content is one grading: what the model returns, what the teacher edits and
// what the student reads.
type Content struct {
	Overall    Overall     `json:"overall"`
	Dimensions []Dimension `json:"dimensions"`
	Points     []Point     `json:"points"`
}

type Overall struct {
	Grade   string `json:"grade"`
	Comment string `json:"comment"`
}

type Dimension struct {
	Name    string `json:"name"`
	Grade   string `json:"grade"`
	Comment string `json:"comment"`
}

// Point is one strength (good) or issue. Quote is a sentence of hers, copied
// exactly; Action is what she does next and is required on an AI issue.
type Point struct {
	Kind   string  `json:"kind"`
	Quote  *string `json:"quote"`
	Text   string  `json:"text"`
	Action *string `json:"action"`
	Source string  `json:"source"`
}

// Input is everything the prompt and Check need about one submitted version.
type Input struct {
	Lang           string
	Title          string
	Body           string // the version body: the only text that counts as hers
	AssignedPrompt string // the teacher's prompt: never counts as hers
	TargetWords    int
	VersionNumber  int
	Rubric         liteassign.Rubric
	SymptomCatalog string // the writing room's symptom table, rendered
	// PersonJudging reports a sentence that judges the student instead of the
	// text. internal/api passes personDirectedVerdict; nil skips the check.
	PersonJudging func(string) bool
}

var ErrUnparseable = errors.New("litegrade: reply is not the JSON object asked for")

// Parse reads the outermost {...} of a reply, which also strips code fences.
// Every error is ErrUnparseable (errors.Is); a JSON decode error also carries
// encoding/json's message, for the server log.
func Parse(text string) (Content, error) {
	start, end := strings.Index(text, "{"), strings.LastIndex(text, "}")
	if start < 0 || end <= start {
		return Content{}, ErrUnparseable
	}
	var c Content
	if err := json.Unmarshal([]byte(text[start:end+1]), &c); err != nil {
		return Content{}, fmt.Errorf("%w: %v", ErrUnparseable, err)
	}
	return c, nil
}

// NormalizeAI trims a model result, marks every point as the AI's, drops the
// action from good points and orders dimensions as the rubric lists them.
func NormalizeAI(c Content, r liteassign.Rubric) Content { return normalize(c, r, true) }

// NormalizeTeacher is NormalizeAI for a teacher's edit: a point keeps source
// "ai" only if it already had it; everything else is the teacher's.
func NormalizeTeacher(c Content, r liteassign.Rubric) Content { return normalize(c, r, false) }

func normalize(c Content, r liteassign.Rubric, fromAI bool) Content {
	out := Content{Overall: Overall{Grade: strings.TrimSpace(c.Overall.Grade), Comment: strings.TrimSpace(c.Overall.Comment)}}
	dims := make([]Dimension, 0, len(c.Dimensions))
	for _, d := range c.Dimensions {
		dims = append(dims, Dimension{Name: strings.TrimSpace(d.Name), Grade: strings.TrimSpace(d.Grade), Comment: strings.TrimSpace(d.Comment)})
	}
	out.Dimensions = orderDimensions(dims, r)
	out.Points = make([]Point, 0, len(c.Points))
	for _, p := range c.Points {
		q := Point{Kind: strings.TrimSpace(p.Kind), Quote: trimPtr(p.Quote), Text: strings.TrimSpace(p.Text), Action: trimPtr(p.Action)}
		if q.Kind == KindGood {
			q.Action = nil
		}
		switch {
		case fromAI, p.Source == SourceAI:
			q.Source = SourceAI
		default:
			q.Source = SourceTeacher
		}
		out.Points = append(out.Points, q)
	}
	return out
}

func trimPtr(s *string) *string {
	if s == nil {
		return nil
	}
	t := strings.TrimSpace(*s)
	if t == "" {
		return nil
	}
	return &t
}

// orderDimensions returns ds in rubric order when the names match exactly;
// otherwise ds unchanged, and Check reports the mismatch.
func orderDimensions(ds []Dimension, r liteassign.Rubric) []Dimension {
	if len(ds) != len(r.Dimensions) {
		return ds
	}
	byName := make(map[string]Dimension, len(ds))
	for _, d := range ds {
		byName[d.Name] = d
	}
	out := make([]Dimension, 0, len(ds))
	for _, rd := range r.Dimensions {
		d, ok := byName[rd.Name]
		if !ok {
			return ds
		}
		out = append(out, d)
	}
	return out
}
