package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"mindimprint/api/internal/gateway"
)

// WeeklyFacts is the deterministic fact sheet the weekly-report composer is
// allowed to see. teacher.BuildWeeklyFacts (internal/teacher/weekly.go) is the
// only producer: every name, tag, and number in here was decided by the rule
// layer, never by the model that will later turn it into prose. Defined here
// (not in internal/teacher) because teacher imports agent, and the reverse
// would be an import cycle.
type WeeklyFacts struct {
	ClassName string           `json:"className"`
	ClassSize int              `json:"classSize"`
	WeekLabel string           `json:"weekLabel"`
	Cards     []WeeklyFactCard `json:"cards"`
}

// WeeklyFactCard is one 值得表扬 / 需要建议 card as the rule layer produced it:
// the student, the tag, and the verbatim evidence quote.
type WeeklyFactCard struct {
	UserID   string `json:"userId"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	TagCode  string `json:"tagCode"`
	TagLabel string `json:"tagLabel"`
	Evidence string `json:"evidence"`
}

// WeeklyProse is the ONLY thing the model contributes to the weekly report:
// wording. It cannot add, drop, reorder, or reclassify a card — the rule
// layer (Task 6, internal/teacher/weekly.go) already decided all of that.
type WeeklyProse struct {
	Comment string            `json:"comment"`
	Cards   []WeeklyCardProse `json:"cards"`
}

// WeeklyCardProse is the wording for one WeeklyFactCard, keyed back to it by
// UserID so the caller can zip prose to fact without trusting model ordering.
type WeeklyCardProse struct {
	UserID string `json:"userId"`
	Lead   string `json:"lead"`
	Action string `json:"action"`
}

// Length caps are rune counts (not bytes): the product is Chinese-first,
// where a byte-based cap would let a handful of CJK characters silently
// blow past what looks like a generous limit. Mirrors the existing
// utf8.RuneCountInString convention in card_lifecycle.go / card_completion.go.
const (
	weeklyCommentMax = 300
	weeklyLeadMax    = 120
	weeklyActionMax  = 200
)

// bareCode matches an internal axis code standing alone (D3, A5), including
// obvious evasions: case (d3) and a space between the letter and the digit
// (D 3). It deliberately stays case-insensitive and whitespace-tolerant even
// though that widens an existing false-positive surface (e.g. "打印纸用A6",
// a paper size, already matched before this change and still does): the fact
// sheet this prose is generated from is Chinese-only class/student/axis
// content, so a real collision with an unrelated alphanumeric token is rare,
// and a false rejection only fails this one-shot compose call — it does not
// corrupt or leak data. That cost is worth paying to close an evasion of the
// 说人话 rule the reviewer could reproduce with a single space. Teachers DO
// see the codes — labelled — on the deep report screen; this screen's prose
// names the behaviour instead.
var bareCode = regexp.MustCompile(`(?i)\b[DA]\s*[1-6]\b`)

func weeklySystemPrompt() string {
	return strings.Join([]string{
		"你为中学教师撰写班级周报。内容直接展示给教师，使用学生姓名或“该生”，不预设性别。",
		"你只负责措辞。需要列出的学生、对应标签与证据，已经由系统判定完毕，你不得增删、调换或重新归类。",
		"规则：",
		"1. 只使用给你的事实。不得引入任何未给出的学生、数字或行为，不从参与次数推断能力、态度或习惯。",
		"2. 用清楚具体的行为描述。不要出现 D1–D6 / A1–A6 这类内部代码。",
		"3. 每张卡写两句：lead 用一句话说清发生了什么；action 写教师线下可以怎么开口（需要建议）或怎么鼓励（值得表扬）。",
		"4. 具体沟通在线下进行，不要建议教师在平台上给学生发消息或打分。",
		"5. 只输出 JSON：{\"comment\":\"\",\"cards\":[{\"userId\":\"\",\"lead\":\"\",\"action\":\"\"}]}",
	}, "\n")
}

// WeeklyFactsPrompt renders the fact sheet the model sees. Exported so a test
// can assert what it does and does not carry — deliberately excludes
// class-level usage counters (turn counts, active-student counts): the prose
// is written once per class per week and never refreshed, so citing a number
// that keeps ticking would be wrong within days.
func WeeklyFactsPrompt(f WeeklyFacts) string {
	var b strings.Builder
	fmt.Fprintf(&b, "班级：%s（%d 名学生）\n周次：%s\n", f.ClassName, f.ClassSize, f.WeekLabel)
	b.WriteString("需要写措辞的卡片：\n")
	for _, c := range f.Cards {
		kind := "需要建议"
		if c.Kind == "praise" {
			kind = "值得表扬"
		}
		fmt.Fprintf(&b, "- userId=%s 姓名=%s 类别=%s 标签=%s 证据=%s\n", c.UserID, c.Name, kind, c.TagLabel, c.Evidence)
	}
	return b.String()
}

// ComposeWeekly makes ONE flagship call turning the fact sheet into wording.
// Usage is returned even when the output is rejected, so the caller records
// the spend either way (mirrors AssessReport, assess_report.go:369).
func ComposeWeekly(ctx context.Context, prov gateway.Provider, r gateway.Resolved, f WeeklyFacts) (WeeklyProse, gateway.ChatUsage, error) {
	res, err := gateway.Collect(ctx, prov, r, gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: weeklySystemPrompt()},
			{Role: gateway.RoleUser, Content: WeeklyFactsPrompt(f)},
		},
	})
	if err != nil {
		return WeeklyProse{}, gateway.ChatUsage{}, err
	}
	usage := res.Usage

	var out WeeklyProse
	if err := json.Unmarshal([]byte(strings.TrimSpace(res.Text)), &out); err != nil {
		return WeeklyProse{}, usage, fmt.Errorf("agent: weekly prose not JSON: %w", err)
	}
	if err := validateWeeklyProse(out, f); err != nil {
		return WeeklyProse{}, usage, err
	}
	return out, usage, nil
}

// validateWeeklyProse enforces that the model contributed wording only: every
// fact-sheet card gets exactly one piece of prose, keyed to a real student,
// that prose is actually non-empty (a card left blank is still a card left
// without wording), nothing runs over its length cap, and no bare internal
// axis code leaks through (说人话).
func validateWeeklyProse(p WeeklyProse, f WeeklyFacts) error {
	want := map[string]bool{}
	for _, c := range f.Cards {
		want[c.UserID] = false
	}
	for _, c := range p.Cards {
		seen, known := want[c.UserID]
		if !known {
			return fmt.Errorf("agent: weekly prose names an unknown student %q", c.UserID)
		}
		if seen {
			return fmt.Errorf("agent: weekly prose repeats student %q", c.UserID)
		}
		want[c.UserID] = true
		if strings.TrimSpace(c.Lead) == "" || strings.TrimSpace(c.Action) == "" {
			return fmt.Errorf("agent: weekly prose leaves card %q without wording", c.UserID)
		}
		if utf8.RuneCountInString(c.Lead) > weeklyLeadMax || utf8.RuneCountInString(c.Action) > weeklyActionMax {
			return fmt.Errorf("agent: weekly card prose too long for %q", c.UserID)
		}
	}
	for id, seen := range want {
		if !seen {
			return fmt.Errorf("agent: weekly prose is missing card %q", id)
		}
	}
	if utf8.RuneCountInString(p.Comment) > weeklyCommentMax {
		return fmt.Errorf("agent: weekly prose exceeds its length cap")
	}
	texts := []string{p.Comment}
	for _, c := range p.Cards {
		texts = append(texts, c.Lead, c.Action)
	}
	for _, t := range texts {
		if bareCode.MatchString(t) {
			return fmt.Errorf("agent: weekly prose contains a bare internal code")
		}
	}
	return nil
}
