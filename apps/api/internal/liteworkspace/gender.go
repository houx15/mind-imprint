package liteworkspace

import "strings"

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

// PronounRule is the prompt line every surface that names students carries.
// Without it the model guessed 他 or 她 from the name, and got some wrong.
const PronounRule = "- **代词按「称谓」写。** 学生的称谓是「她」或「他」时，只用这个代词；" +
	"称谓是「未设置」时，不用「他」「她」指这名学生，重复姓名，或者写「该生」。" +
	"称呼老师时用「您」。"
