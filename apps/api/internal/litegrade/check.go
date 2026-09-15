package litegrade

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"mindimprint/api/internal/liteassign"
	"mindimprint/api/internal/quotematch"
)

// Reason codes. Every rejection Check makes has a name, a test and a message
// the teacher reads after 「批改失败：」.
const (
	ReasonQuoteMissing       = "quote_missing"
	ReasonQuoteNotInBody     = "quote_not_in_body"
	ReasonQuotationNotInBody = "quotation_not_in_body"
	ReasonQuoteFromPrompt    = "quote_from_prompt"
	ReasonGradeOutOfScale    = "grade_out_of_scale"
	ReasonDimensionNames     = "dimension_names_mismatch"
	ReasonPointCount         = "point_count"
	ReasonNoGoodPoint        = "no_good_point"
	ReasonNoIssuePoint       = "no_issue_point"
	ReasonIssueWithoutAction = "issue_without_action"
	ReasonPersonJudging      = "person_judging"
	ReasonPointKind          = "invalid_point_kind"
	ReasonEmptyText          = "empty_text"
	ReasonLanguageMismatch   = "language_mismatch"
	ReasonUnparseable        = "unparseable"
	ReasonModelCall          = "model_call_failed"
)

type Reason struct {
	Code   string
	Where  string // 总评 / 维度「内容」 / 第 2 条意见 …
	Detail string
}

func (r Reason) Message() string {
	switch r.Code {
	case ReasonQuoteMissing:
		return r.Where + "没有引用原文"
	case ReasonQuoteNotInBody:
		return r.Where + "的引文不在正文中：「" + r.Detail + "」"
	case ReasonQuotationNotInBody:
		return r.Where + "里引号内的文字不在正文中：「" + r.Detail + "」"
	case ReasonQuoteFromPrompt:
		return r.Where + "引用的是作业题目，不是学生的原文：「" + r.Detail + "」"
	case ReasonGradeOutOfScale:
		return r.Where + "的等级不在评分标准内：" + r.Detail
	case ReasonDimensionNames:
		return "评分维度与评分标准不一致：" + r.Detail
	case ReasonPointCount:
		return "意见共 " + r.Detail + " 条，需要 3 到 5 条"
	case ReasonNoGoodPoint:
		return "没有优点意见"
	case ReasonNoIssuePoint:
		return "没有问题意见"
	case ReasonIssueWithoutAction:
		return r.Where + "没有修改建议"
	case ReasonPersonJudging:
		return r.Where + "评价的是学生本人，不是文字"
	case ReasonPointKind:
		return r.Where + "的类型无效：" + r.Detail
	case ReasonEmptyText:
		return r.Where + "为空"
	case ReasonLanguageMismatch:
		return "批改的语言与作文不一致，应主要用" + r.Detail + "写"
	case ReasonUnparseable:
		return "回复不是有效的 JSON"
	case ReasonModelCall:
		return "模型调用失败：" + r.Detail
	}
	return r.Code
}

// JoinReasons is the grading row's error text: messages joined by 「；」,
// duplicates once.
func JoinReasons(rs []Reason) string { return strings.Join(reasonMessages(rs), "；") }

func reasonMessages(rs []Reason) []string {
	seen := make(map[string]bool, len(rs))
	out := make([]string, 0, len(rs))
	for _, r := range rs {
		m := r.Message()
		if !seen[m] {
			seen[m] = true
			out = append(out, m)
		}
	}
	return out
}

// languageThreshold is how much of the model's own prose (overall comment,
// dimension comments, point text and point action — her quoted spans
// excluded) must be in the writing's language: at least 60% Han characters
// for a zh writing, at least 60% Latin letters for an en writing. 60% rather
// than 100% leaves room for the odd technical term or number without
// flagging every grading that mentions one.
const languageThreshold = 0.6

// Check is the gate for a model result. A result with any reason is retried
// once and then marked failed; it never reaches the teacher as a draft.
//
// Check verifies only some of what SystemPrompt asks for: the grade scale
// (ReasonGradeOutOfScale), the rubric's dimension names (ReasonDimensionNames),
// the point count and the good/issue mix (ReasonPointCount, ReasonNoGoodPoint,
// ReasonNoIssuePoint), a required action on every issue
// (ReasonIssueWithoutAction), every quote and every 「」/『』/“” quotation
// being a match of her body once normalized — or, if not, of the assigned
// prompt (ReasonQuoteMissing, ReasonQuoteNotInBody, ReasonQuotationNotInBody,
// ReasonQuoteFromPrompt; see package quotematch for what "normalized" means
// and why a short quotation isn't checked at all), sentences that judge her
// instead of her writing (ReasonPersonJudging, only for the phrases
// PersonJudging recognises), and the feedback being mostly written in the
// writing's language (ReasonLanguageMismatch, an approximate character-ratio
// check). The prompt's "不要重写、不要润色、不要续写" and "不写客套话" are
// not verified here — there is no code check for either.
func Check(c Content, in Input) []Reason {
	cp := newCorpus(in)
	rs := shapeReasons(c, in)
	rs = append(rs, textReasons("总评", c.Overall.Comment, in, cp, true)...)
	for _, d := range c.Dimensions {
		rs = append(rs, textReasons("维度「"+d.Name+"」的评语", d.Comment, in, cp, true)...)
	}
	if n := len(c.Points); n < MinPoints || n > MaxPoints {
		rs = append(rs, Reason{Code: ReasonPointCount, Detail: strconv.Itoa(n)})
	}
	good, issue := false, false
	for i, p := range c.Points {
		where := fmt.Sprintf("第 %d 条意见", i+1)
		switch p.Kind {
		case KindGood:
			good = true
		case KindIssue:
			issue = true
			if blank(p.Action) {
				rs = append(rs, Reason{Code: ReasonIssueWithoutAction, Where: where})
			}
		}
		if blank(p.Quote) {
			rs = append(rs, Reason{Code: ReasonQuoteMissing, Where: where})
		} else if r := cp.reason(where, *p.Quote, ReasonQuoteNotInBody); r != nil {
			rs = append(rs, *r)
		}
		rs = append(rs, textReasons(where+"的说明", p.Text, in, cp, true)...)
		if p.Action != nil {
			rs = append(rs, textReasons(where+"的修改建议", *p.Action, in, cp, false)...)
		}
	}
	if !good {
		rs = append(rs, Reason{Code: ReasonNoGoodPoint})
	}
	if !issue {
		rs = append(rs, Reason{Code: ReasonNoIssuePoint})
	}
	if r := languageReason(c, in); r != nil {
		rs = append(rs, *r)
	}
	return rs
}

// CheckTeacherEdit is the shape check on a teacher's PATCH: grades in scale,
// the rubric's dimension names, known point kinds, non-empty point text, and a
// quote that is null or hers. No 3–5 limit, no action requirement, and no
// language check — this is the teacher's own edit, not a model result.
func CheckTeacherEdit(c Content, in Input) []Reason {
	cp := newCorpus(in)
	rs := shapeReasons(c, in)
	for i, p := range c.Points {
		where := fmt.Sprintf("第 %d 条意见", i+1)
		if strings.TrimSpace(p.Text) == "" {
			rs = append(rs, Reason{Code: ReasonEmptyText, Where: where + "的说明"})
		}
		if !blank(p.Quote) {
			if r := cp.reason(where, *p.Quote, ReasonQuoteNotInBody); r != nil {
				rs = append(rs, *r)
			}
		}
	}
	return rs
}

func shapeReasons(c Content, in Input) []Reason {
	var rs []Reason
	if !liteassign.GradeInScale(in.Rubric, c.Overall.Grade) {
		rs = append(rs, Reason{Code: ReasonGradeOutOfScale, Where: "总评", Detail: c.Overall.Grade})
	}
	for _, d := range c.Dimensions {
		if !liteassign.GradeInScale(in.Rubric, d.Grade) {
			rs = append(rs, Reason{Code: ReasonGradeOutOfScale, Where: "维度「" + d.Name + "」", Detail: d.Grade})
		}
	}
	if !sameNames(c.Dimensions, in.Rubric.Dimensions) {
		names := make([]string, 0, len(c.Dimensions))
		for _, d := range c.Dimensions {
			names = append(names, d.Name)
		}
		rs = append(rs, Reason{Code: ReasonDimensionNames, Detail: strings.Join(names, "、")})
	}
	for i, p := range c.Points {
		if p.Kind != KindGood && p.Kind != KindIssue {
			rs = append(rs, Reason{Code: ReasonPointKind, Where: fmt.Sprintf("第 %d 条意见", i+1), Detail: p.Kind})
		}
	}
	return rs
}

// textReasons checks one piece of the model's prose: present (when required),
// every long-enough 「」/『』/“” quotation hers, and not about her as a person.
// A quotation shorter than quotematch.MinRunes is skipped — it's usually a
// term or a symptom-catalog name (「让步」「只有主题」), not a copied
// sentence, and checking it against her body produces false positives.
func textReasons(where, s string, in Input, cp corpus, required bool) []Reason {
	var rs []Reason
	if strings.TrimSpace(s) == "" {
		if required {
			rs = append(rs, Reason{Code: ReasonEmptyText, Where: where})
		}
		return rs
	}
	for _, span := range quotematch.ExtractQuotedSpans(s) {
		if len([]rune(quotematch.Normalize(span))) < quotematch.MinRunes {
			continue
		}
		if r := cp.reason(where, span, ReasonQuotationNotInBody); r != nil {
			rs = append(rs, *r)
		}
	}
	if in.PersonJudging != nil && in.PersonJudging(s) {
		rs = append(rs, Reason{Code: ReasonPersonJudging, Where: where})
	}
	return rs
}

// corpus is her body and the teacher's assigned prompt, normalized once so
// every quote check in one Check/CheckTeacherEdit call reuses the same
// normalization instead of repeating it per quote.
type corpus struct {
	normBody   string
	normPrompt string
	hasPrompt  bool
}

func newCorpus(in Input) corpus {
	return corpus{
		normBody:   quotematch.Normalize(in.Body),
		normPrompt: quotematch.Normalize(in.AssignedPrompt),
		hasPrompt:  in.AssignedPrompt != "",
	}
}

// reason: nil when q is part of her body once both sides are normalized —
// so a half-width comma, a trailing 。, or a quote that spans a paragraph
// break in the source no longer causes a false rejection. Text found only
// in the teacher's assigned prompt gets its own reason: the teacher's words
// are never hers, however exactly they're copied.
func (cp corpus) reason(where, q string, notInBody string) *Reason {
	q = strings.TrimSpace(q)
	if q == "" {
		return nil
	}
	n := quotematch.Normalize(q)
	if strings.Contains(cp.normBody, n) {
		return nil
	}
	if cp.hasPrompt && strings.Contains(cp.normPrompt, n) {
		return &Reason{Code: ReasonQuoteFromPrompt, Where: where, Detail: q}
	}
	return &Reason{Code: notInBody, Where: where, Detail: q}
}

func sameNames(got []Dimension, want []liteassign.RubricDimension) bool {
	if len(got) != len(want) {
		return false
	}
	left := make(map[string]int, len(want))
	for _, d := range want {
		left[d.Name]++
	}
	for _, d := range got {
		if left[d.Name] == 0 {
			return false
		}
		left[d.Name]--
	}
	return true
}

func blank(s *string) bool { return s == nil || strings.TrimSpace(*s) == "" }

// languageReason compares Han characters against Latin letters across the
// model's own prose (her quoted 「」/『』/“” spans stripped out first,
// regardless of length — this is about who wrote the surrounding words, not
// about verifying the quote) and rejects when the writing's language is not
// the clear majority. Content with no Han or Latin letters at all (say,
// every field is blank, which textReasons already catches) is skipped
// rather than guessed at.
func languageReason(c Content, in Input) *Reason {
	if in.Lang != "zh" && in.Lang != "en" {
		return nil
	}
	var han, latin int
	tally := func(s string) {
		for _, r := range quotematch.StripQuotedSpans(s) {
			switch {
			case unicode.Is(unicode.Han, r):
				han++
			case unicode.Is(unicode.Latin, r):
				latin++
			}
		}
	}
	tally(c.Overall.Comment)
	for _, d := range c.Dimensions {
		tally(d.Comment)
	}
	for _, p := range c.Points {
		tally(p.Text)
		if p.Action != nil {
			tally(*p.Action)
		}
	}
	total := han + latin
	if total == 0 {
		return nil
	}
	switch in.Lang {
	case "zh":
		if float64(han)/float64(total) < languageThreshold {
			return &Reason{Code: ReasonLanguageMismatch, Detail: "中文"}
		}
	case "en":
		if float64(latin)/float64(total) < languageThreshold {
			return &Reason{Code: ReasonLanguageMismatch, Detail: "英文"}
		}
	}
	return nil
}
