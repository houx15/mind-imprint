// Package litegrade holds the rules of lite AI 批改: the content shape, the
// prompt, and Check. Pure functions: no database, no HTTP. The model call,
// the retry and the grading rows live in internal/api (lite_grading_*.go).
package litegrade

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
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
	// Dimension 是这条意见挂在 rubric 的哪一维上（逐字来自 Rubric.Dimensions
	// 的 Name）。Symptom 是它命中了毛病闭表里的哪一条（和学生端
	// CommentPoint.Symptom 同一套 id）。
	//
	// 这两样是给**老师**看的：批改卡上点开一条意见，她要看到这条是从哪一维、
	// 哪条毛病来的、引的是学生哪一句。产品负责人 2026-09-22：
	// 「we also need to tell teacher the rationale or the real logic of our
	// comment there」。
	//
	// 🚨 两个都允许为空 —— 模型给不出来时宁可没有，不要编一个。校验时
	// 不在闭表里的直接清空（照 CommentPoint.Symptom 的 lookupWritingSymptom 那套）。
	Dimension string `json:"dimension"`
	Symptom   string `json:"symptom"`
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
	// SymptomLookup resolves a Point.Symptom id to the writing room's closed
	// symptom table's teacher-facing name (writingSymptom.Name), for this
	// writing's language. internal/api wires it to lookupWritingSymptom —
	// litegrade cannot import internal/api itself (internal/api already
	// imports litegrade; the reverse would be a cycle), the same reason
	// PersonJudging above is a callback rather than a direct call.
	//
	// 🚨 The model is asked to write the table's id (reliable exact match,
	// same discipline CommentPoint.Symptom already uses) but a teacher
	// reading the 依据 modal needs a name she can read, not a code like
	// `topic_without_question` — so SanitizeProvenance below rewrites a
	// valid id to its name once, rather than asking every reader of a
	// grading to resolve it again. ok=false (unknown id, or SymptomLookup
	// nil) clears the field instead of showing her a code or a guess.
	SymptomLookup func(id string) (name string, ok bool)
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
	body := text[start : end+1]
	err := json.Unmarshal([]byte(body), &c)
	if err == nil {
		return c, nil
	}
	// 2026-09-18 写作入口走查：一份中文批改两次都在数组里写成了
	// `},"{"name":…` —— 对象前面多了一个引号，整份批改因此失败。
	// 只修这一种形状（引号紧贴在 `},` 与 `{` 之间），修完仍不是合法 JSON
	// 就照旧报原来的错。
	if fixed := strayQuoteBeforeObject.ReplaceAllString(body, "},{"); fixed != body {
		var c2 Content
		if json.Unmarshal([]byte(fixed), &c2) == nil {
			return c2, nil
		}
	}
	return Content{}, fmt.Errorf("%w: %v", ErrUnparseable, err)
}

var strayQuoteBeforeObject = regexp.MustCompile(`\}\s*,\s*"\s*\{`)

// NormalizeAI trims a model result, marks every point as the AI's, drops the
// action from good points and orders dimensions as the rubric lists them.
func NormalizeAI(c Content, r liteassign.Rubric) Content { return normalize(c, r, true) }

// NormalizeTeacher is NormalizeAI for a teacher's edit: a point keeps source
// "ai" only if it already had it; everything else is the teacher's.
func NormalizeTeacher(c Content, r liteassign.Rubric) Content { return normalize(c, r, false) }

// BlankContent is the empty grading a 人工批改 starts from: one row per rubric
// dimension, nothing filled in. The teacher chooses every grade herself, so no
// grade is guessed here — CheckTeacherEdit rejects an empty grade, which is
// what asks her to pick one before the first save.
func BlankContent(r liteassign.Rubric) Content {
	dims := make([]Dimension, 0, len(r.Dimensions))
	for _, d := range r.Dimensions {
		dims = append(dims, Dimension{Name: d.Name})
	}
	return Content{Dimensions: dims, Points: []Point{}}
}

func normalize(c Content, r liteassign.Rubric, fromAI bool) Content {
	out := Content{Overall: Overall{Grade: strings.TrimSpace(c.Overall.Grade), Comment: strings.TrimSpace(c.Overall.Comment)}}
	dims := make([]Dimension, 0, len(c.Dimensions))
	for _, d := range c.Dimensions {
		name := strings.TrimSpace(d.Name)
		if fromAI {
			name = rubricDimensionName(name, r)
		}
		dims = append(dims, Dimension{Name: name, Grade: strings.TrimSpace(d.Grade), Comment: strings.TrimSpace(d.Comment)})
	}
	out.Dimensions = orderDimensions(dims, r)
	out.Points = make([]Point, 0, len(c.Points))
	for _, p := range c.Points {
		q := Point{
			Kind: strings.TrimSpace(p.Kind), Quote: trimPtr(p.Quote), Text: strings.TrimSpace(p.Text), Action: trimPtr(p.Action),
			Dimension: strings.TrimSpace(p.Dimension), Symptom: strings.TrimSpace(p.Symptom),
		}
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
	if fromAI {
		out.Points = capPoints(out.Points)
	}
	return out
}

// capPoints keeps at most MaxPoints of a model's points, in their order.
//
// 2026-09-18 写作入口走查：一次英文批改两次都回了 6 条意见，整份批改因此
// 失败，老师什么都拿不到。多出来的那一条是最不要紧的一条（模型按重要性排），
// 删掉它不改任何一条留下的话。至少各留一条优点和问题，否则删完又会因为
// 「没有优点意见」再失败一次。
func capPoints(ps []Point) []Point {
	if len(ps) <= MaxPoints {
		return ps
	}
	keep := make([]bool, len(ps))
	n := 0
	for _, kind := range []string{KindGood, KindIssue} {
		for i, p := range ps {
			if p.Kind == kind {
				keep[i] = true
				n++
				break
			}
		}
	}
	for i := range ps {
		if n >= MaxPoints {
			break
		}
		if !keep[i] {
			keep[i] = true
			n++
		}
	}
	out := make([]Point, 0, MaxPoints)
	for i, p := range ps {
		if keep[i] {
			out = append(out, p)
		}
	}
	return out
}

// rubricDimensionName maps a dimension name the model wrote with the rubric's
// note attached (「内容：立意是否明确，材料是否支撑观点」) back to the rubric's
// name. Measured 2026-09-17: both attempts of a grading came back that way
// and the grading failed with 「评分维度与评分标准不一致」. The name is the
// rubric's, not the student's text, so this repair changes nothing she wrote.
func rubricDimensionName(name string, r liteassign.Rubric) string {
	for _, d := range r.Dimensions {
		if name == d.Name {
			return name
		}
	}
	for _, d := range r.Dimensions {
		for _, sep := range []string{"：", ":", "（", "(", " - ", "——", " "} {
			if strings.HasPrefix(name, d.Name+sep) {
				return d.Name
			}
		}
	}
	return name
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
