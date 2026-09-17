package liteworkspace

import (
	"regexp"
	"slices"
	"strings"
)

// Gender values stored in users.gender (migration 0175). An empty string is
// 「未设置」.
const (
	GenderFemale = "female"
	GenderMale   = "male"
)

// ParseGender reads a gender a teacher chose. "" clears the setting; any other
// value than the two above is rejected.
func ParseGender(s string) (string, bool) {
	switch strings.TrimSpace(s) {
	case "":
		return "", true
	case GenderFemale:
		return GenderFemale, true
	case GenderMale:
		return GenderMale, true
	}
	return "", false
}

// GenderOf reads a nullable users.gender column.
func GenderOf(p *string) string {
	if p == nil {
		return ""
	}
	g, ok := ParseGender(*p)
	if !ok {
		return ""
	}
	return g
}

// Pronoun is the pronoun the model may use for a student: 她, 他, or
// PronounUnset when the teacher has not set a gender. The model reads this
// word next to the student's name in every tool result and fact list that
// names a student.
func Pronoun(gender string) string {
	switch gender {
	case GenderFemale:
		return "她"
	case GenderMale:
		return "他"
	}
	return PronounUnset
}

// PronounUnset is what the model reads for a student with no gender set.
const PronounUnset = "未设置"

// notAPronoun are words that contain 他 or 她 without referring to a student.
var notAPronoun = regexp.MustCompile(`其他|其她|他人|吉他|他乡|他国`)

// PronounsAllowed is which gendered pronouns a text may use: Female for 她
// (and 她们 unless Male), Male for 他 (and 他们).
type PronounsAllowed struct{ Female, Male bool }

// AllowPronounsOf adds the genders of the students in roster whose name is in
// names.
func (p *PronounsAllowed) AllowPronounsOf(roster []Student, names []string) {
	for _, s := range roster {
		if !slices.Contains(names, s.Name) {
			continue
		}
		p.Allow(s.Gender)
	}
}

// Allow adds one gender.
func (p *PronounsAllowed) Allow(gender string) {
	switch gender {
	case GenderFemale:
		p.Female = true
	case GenderMale:
		p.Male = true
	}
}

// AllowTyped adds the pronouns the teacher used herself: she knows who she
// means.
func (p *PronounsAllowed) AllowTyped(typed string) {
	t := notAPronoun.ReplaceAllString(typed, "")
	if strings.Contains(t, "她") {
		p.Female = true
	}
	if strings.Contains(t, "他") {
		p.Male = true
	}
}

// PronounProblem reports, in Chinese, a gendered pronoun in text that p does
// not allow; "" when there is none.
//
// The prompt rule alone did not hold: on production 2026-09-17 the class chat
// offered 「给她们布置作业」 for two students with no gender set, and after the
// rule named plurals the live run still produced 「需要提醒她们吗」.
func PronounProblem(text string, p PronounsAllowed) string {
	t := notAPronoun.ReplaceAllString(text, "")
	var bad string
	switch {
	case strings.Contains(t, "她们") && (!p.Female || p.Male):
		bad = "她们"
	case strings.Contains(t, "他们") && !p.Male:
		bad = "他们"
	}
	if bad == "" {
		t = strings.NewReplacer("她们", "", "他们", "").Replace(t)
		switch {
		case strings.Contains(t, "她") && !p.Female:
			bad = "她"
		case strings.Contains(t, "他") && !p.Male:
			bad = "他"
		}
	}
	if bad == "" {
		return ""
	}
	return "回复用了「" + bad + "」，但涉及的学生没有设置对应的性别。未设置性别的学生不用「他」「她」，" +
		"写姓名或「该生」；指多名学生时写「这些学生」"
}

// PronounRule is the prompt line every surface that names students carries.
// Without it the model guessed 他 or 她 from the name, and got some wrong.
const PronounRule = "- **代词按「称谓」写。** 学生的称谓是「她」或「他」时，只用这个代词；" +
	"称谓是「未设置」时，不用「他」「她」指这名学生，重复姓名，或者写「该生」。" +
	"指多名学生时写「这些学生」「这两名学生」，不用「他们」「她们」。" +
	"称呼老师时用「您」。"
