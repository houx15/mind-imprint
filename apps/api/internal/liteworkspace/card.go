package liteworkspace

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// card.go — rules for what the assignment AI writes on the homework card and
// says about it. Each one comes from the 2026-09-17 real-user walk.

// wrappingQuotes are the marks the model puts around a title it names. The
// prompt asks for 《》 when a title is mentioned in a reply, and the model
// applied that to the title field too: the homework was published as
// 「《AI 当数学家教的正确用法》」.
var wrappingQuotes = [][2]string{{"《", "》"}, {"「", "」"}, {"『", "』"}, {"“", "”"}, {"\"", "\""}}

// CleanTitle trims a title and removes one pair of quote marks around the
// whole of it. Marks inside the title stay.
func CleanTitle(s string) string {
	s = strings.TrimSpace(s)
	for _, q := range wrappingQuotes {
		if strings.HasPrefix(s, q[0]) && strings.HasSuffix(s, q[1]) && len(s) > len(q[0])+len(q[1]) {
			inner := strings.TrimSpace(s[len(q[0]) : len(s)-len(q[1])])
			if !strings.Contains(inner, q[0]) && !strings.Contains(inner, q[1]) {
				return inner
			}
		}
	}
	return s
}

var weekdayNames = [...]string{"周日", "周一", "周二", "周三", "周四", "周五", "周六"}

// DueLabel renders a card's datetime-input value (2006-01-02T15:04, Beijing
// wall clock) the way a teacher reads it: 9月18日（周五）21:00. A value it
// cannot read comes back unchanged.
func DueLabel(input string) string {
	t, err := time.ParseInLocation("2006-01-02T15:04", strings.TrimSpace(input), BeijingOffset)
	if err != nil {
		return input
	}
	return fmt.Sprintf("%d月%d日（%s）%s", t.Month(), t.Day(), weekdayNames[t.Weekday()], t.Format("15:04"))
}

var wireDatetime = regexp.MustCompile(`(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2})`)

// HumanizeDatetimes rewrites every 2006-01-02T15:04 in text as 9月18日 21:00.
// Measured: the model echoed the card's input value to the teacher
// (「截止时间：2026-09-18T21:00」) although the prompt said to write dates in
// words.
func HumanizeDatetimes(text string) string {
	return wireDatetime.ReplaceAllStringFunc(text, func(m string) string {
		t, err := time.Parse("2006-01-02T15:04", m)
		if err != nil {
			return m
		}
		return fmt.Sprintf("%d月%d日 %s", t.Month(), t.Day(), t.Format("15:04"))
	})
}

// CardField is one cell of the homework card the model may say it filled.
type CardField struct {
	Label string
	Value string
}

// clauseSplit cuts at clause ends too: 「标题已经填好，驱动问题还需要您定」
// claims the first cell only.
var clauseSplit = regexp.MustCompile(`[。！？!?\n，,；;]`)

// writtenMarker is a completed write: 已加上, 已经写好, 填进去了, 已写入.
var writtenMarker = regexp.MustCompile(
	`已(经)?(帮您|为您|给您)?(把)?[^，。！？\n]{0,8}?(加上|加好|填上|填好|填入|填进|写上|写好|写入|写进|设好|设置|设为|更新|改好|换成|放进)` +
		`|(加|填|写|放|设)(好|进去|上去|上)了`)

var writtenNegation = regexp.MustCompile(`(还没|没有|尚未|未|不能|无法|没)`)

// UnfilledClaim returns the first field that text says was filled while the
// card shows it empty; "" when there is none.
//
// Measured 2026-09-17: asked five times to write the 驱动问题, the model
// answered 「已加上驱动问题」, 「驱动问题已写入」, 「我刚才已经把驱动问题填进去了」
// while no tool could write that field and the card stayed empty.
func UnfilledClaim(text string, fields []CardField) string {
	for _, sentence := range clauseSplit.Split(text, -1) {
		if !writtenMarker.MatchString(sentence) {
			continue
		}
		loc := writtenMarker.FindStringIndex(sentence)
		if writtenNegation.MatchString(sentence[:loc[0]]) {
			continue
		}
		for _, f := range fields {
			if f.Label != "" && strings.Contains(sentence, f.Label) && strings.TrimSpace(f.Value) == "" {
				return f.Label
			}
		}
	}
	return ""
}
