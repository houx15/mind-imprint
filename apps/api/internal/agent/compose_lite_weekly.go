package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/liteweekly"
)

// LiteStudentWeeklyProse is the model-written part of one student's weekly
// summary for a lite teacher. Everything it may say comes from
// liteweekly.FactsText and the rule cards; liteweekly.CheckProse enforces
// that before the prose is returned.
type LiteStudentWeeklyProse struct {
	Summary     string                 `json:"summary"`
	Suggestions []LiteWeeklySuggestion `json:"suggestions"`
}

// LiteWeeklySuggestion is one thing the teacher can do next week, tied to
// the rule card that justifies it.
type LiteWeeklySuggestion struct {
	Text         string `json:"text"`
	EvidenceCode string `json:"evidenceCode"`
}

// LiteClassWeeklyProse is the model-written part of the class weekly
// summary: one class comment plus wording for each flagged student.
type LiteClassWeeklyProse struct {
	Comment string                     `json:"comment"`
	Cards   []LiteClassWeeklyCardProse `json:"cards"`
}

// LiteClassWeeklyCardProse is the wording for one flagged student, keyed by
// UserID so the caller never relies on the model's ordering.
type LiteClassWeeklyCardProse struct {
	UserID string `json:"userId"`
	Lead   string `json:"lead"`
	Action string `json:"action"`
}

// Attempt is one model call's usage and raw text, returned so the caller records every call.
type Attempt struct {
	Usage gateway.ChatUsage
	Text  string
	Err   error
}

// Length caps are rune counts: a byte cap would let a few CJK characters
// pass a limit that looks generous.
const (
	liteSummaryMax      = 150
	liteSuggestionMax   = 120
	liteSuggestionsMax  = 3
	liteClassCommentMax = 300
	liteClassLeadMax    = 120
	liteClassActionMax  = 200

	// liteWeeklyMaxAttempts is the first call plus one retry.
	liteWeeklyMaxAttempts = 2
)

const liteStudentWeeklySystemPrompt = `你在给老师写一名学生上一周的学习总结。只使用给出的事实，不补充事实。输出 JSON {"summary":"","suggestions":[{"text":"","evidenceCode":""}]}。summary 不超过 150 字；suggestions 1 到 3 条，每条是老师下周可以做的一件具体的事，evidenceCode 必须是给出的卡片代码之一；没有卡片时 suggestions 为空数组。引用学生原话时用「」且逐字照抄给出的金句。作品标题用《》，只有学生原话用「」。不使用给出事实里没有的数字。不写其他学生的名字。说明文，不用比喻和抒情。`

const liteClassWeeklySystemPrompt = `你在给老师写一个班级上一周的学习总结。
哪些学生需要写卡片、每张卡片的类别、标签和证据，已经由系统判定。你只负责措辞，不增加、不删除、不调换、不重新归类。
规则：
1. 只使用给出的事实，不补充事实。不使用给出事实里没有的数字。不写学生名单以外的学生。
2. comment 是全班的总结，不超过 300 字。
3. cards 给学生名单里的每名学生写一条，且只写一条，userId 照抄名单里的 userId，不多写，不漏写；学生名单为无时 cards 为空数组。
4. lead 用一句话说明这名学生上一周发生了什么，不超过 120 字；action 写老师线下可以怎么沟通（需要建议）或怎么鼓励（值得表扬），不超过 200 字。
5. 具体沟通在线下进行，不建议老师在平台上给学生发消息或打分。
6. 引用学生原话时用「」且逐字照抄给出的金句。作品标题用《》，只有学生原话用「」。
7. 说明文，不用比喻和抒情。
8. 只输出 JSON：{"comment":"","cards":[{"userId":"","lead":"","action":""}]}`

// liteRetryLine is the user turn appended before the second attempt.
func liteRetryLine(err error) string {
	return "上一次输出未通过校验：" + err.Error() + "。请重新输出。"
}

// ComposeLiteStudentWeekly writes one student's weekly summary and 1–3
// suggestions (none when there are no cards). Invalid output gets one retry
// with the validation error; every call is returned in the attempts so the
// caller can record its usage. On failure it returns an error, never
// substitute prose.
func ComposeLiteStudentWeekly(ctx context.Context, prov gateway.Provider, r gateway.Resolved, s liteweekly.StudentWeek, weekLabel string, cards []liteweekly.Card, otherNames []string) (LiteStudentWeeklyProse, []Attempt, error) {
	msgs := []gateway.ChatMessage{
		{Role: gateway.RoleSystem, Content: liteStudentWeeklySystemPrompt},
		{Role: gateway.RoleUser, Content: liteStudentWeeklyUserPrompt(s, weekLabel, cards)},
	}
	out, attempts, err := composeLiteWeekly(ctx, prov, r, msgs, func(p LiteStudentWeeklyProse) error {
		return validateLiteStudentWeekly(p, s, weekLabel, cards, otherNames)
	})
	if err != nil {
		return LiteStudentWeeklyProse{}, attempts, fmt.Errorf("agent: lite student weekly prose: %w", err)
	}
	return out, attempts, nil
}

// ComposeLiteClassWeekly writes the class comment and one lead/action pair
// per flagged student (a student whose cards entry is non-empty).
//
// allNames is the class roster. It is not used for the name check: every
// student in the class may be named in the class summary, so
// ProseCheck.OtherNames stays empty. The handler passes it to keep that
// decision visible at the call site.
func ComposeLiteClassWeekly(ctx context.Context, prov gateway.Provider, r gateway.Resolved, className string, weekLabel string, stats liteweekly.ClassWeekStats, students []liteweekly.StudentWeek, cards map[string][]liteweekly.Card, allNames []string) (LiteClassWeeklyProse, []Attempt, error) {
	_ = allNames
	flagged := liteFlaggedStudents(students, cards)
	facts := liteweekly.ClassFactsText(className, weekLabel, stats, flagged, cards)
	msgs := []gateway.ChatMessage{
		{Role: gateway.RoleSystem, Content: liteClassWeeklySystemPrompt},
		{Role: gateway.RoleUser, Content: liteClassWeeklyUserPrompt(facts, flagged)},
	}
	out, attempts, err := composeLiteWeekly(ctx, prov, r, msgs, func(p LiteClassWeeklyProse) error {
		return validateLiteClassWeekly(p, flagged, facts)
	})
	if err != nil {
		return LiteClassWeeklyProse{}, attempts, fmt.Errorf("agent: lite class weekly prose: %w", err)
	}
	return out, attempts, nil
}

// composeLiteWeekly runs at most liteWeeklyMaxAttempts non-streaming calls.
// A parse or validation failure is sent back to the model once; a transport
// error returns immediately.
func composeLiteWeekly[T any](ctx context.Context, prov gateway.Provider, r gateway.Resolved, base []gateway.ChatMessage, validate func(T) error) (T, []Attempt, error) {
	var zero T
	var attempts []Attempt
	msgs := base
	for i := 0; i < liteWeeklyMaxAttempts; i++ {
		res, err := gateway.Collect(ctx, prov, r, gateway.ChatRequest{Messages: msgs})
		if err != nil {
			attempts = append(attempts, Attempt{Usage: res.Usage, Text: res.Text, Err: err})
			return zero, attempts, err
		}
		out, verr := parseLiteWeeklyJSON[T](res.Text)
		if verr == nil {
			verr = validate(out)
		}
		attempts = append(attempts, Attempt{Usage: res.Usage, Text: res.Text, Err: verr})
		if verr == nil {
			return out, attempts, nil
		}
		msgs = append(append(make([]gateway.ChatMessage, 0, len(base)+2), base...),
			gateway.ChatMessage{Role: gateway.RoleAssistant, Content: res.Text},
			gateway.ChatMessage{Role: gateway.RoleUser, Content: liteRetryLine(verr)},
		)
	}
	return zero, attempts, fmt.Errorf("rejected after %d attempts: %w", len(attempts), attempts[len(attempts)-1].Err)
}

// parseLiteWeeklyJSON decodes the model reply, accepting a ```json fence or
// prose around the object. Each decode goes into a fresh value so a failed
// first decode cannot leave fields behind.
func parseLiteWeeklyJSON[T any](text string) (T, error) {
	var v T
	err := json.Unmarshal([]byte(stripFences(text)), &v)
	if err == nil {
		return v, nil
	}
	if obj := extractJSONObject(text); obj != "" {
		var w T
		if json.Unmarshal([]byte(obj), &w) == nil {
			return w, nil
		}
	}
	return v, fmt.Errorf("output is not valid JSON: %v", err)
}

func liteStudentWeeklyUserPrompt(s liteweekly.StudentWeek, weekLabel string, cards []liteweekly.Card) string {
	var b strings.Builder
	fmt.Fprintf(&b, "学生：%s\n", s.Name)
	fmt.Fprintf(&b, "事实：%s\n", liteweekly.FactsText(s, weekLabel))
	b.WriteString("卡片：")
	if len(cards) == 0 {
		b.WriteString("无")
	}
	for _, c := range cards {
		fmt.Fprintf(&b, "\n- %s · %s · %s", c.Code, c.Label, c.Evidence)
	}
	return b.String()
}

// liteClassWeeklyUserPrompt is the class facts text followed by the flagged
// students' IDs and the words the prose may quote: titles as 《》, her words
// as 「」. IDs are kept out of the facts text so their digits do not widen the
// digit check.
func liteClassWeeklyUserPrompt(facts string, flagged []liteweekly.StudentWeek) string {
	var b strings.Builder
	b.WriteString(facts)
	b.WriteString("\n学生名单：")
	if len(flagged) == 0 {
		b.WriteString("无")
	}
	for _, s := range flagged {
		parts := []string{"userId=" + s.UserID, "姓名=" + s.Name}
		for _, it := range s.Finished {
			parts = append(parts, "完成《"+it.Title+"》")
		}
		for _, it := range s.Stalled {
			parts = append(parts, "停滞《"+it.Title+"》")
		}
		for _, k := range s.NewKeywords {
			parts = append(parts, "新关键词 "+k)
		}
		for _, m := range s.Moments {
			parts = append(parts, "「"+m.Quote+"」（《"+m.ItemTitle+"》）")
		}
		b.WriteString("\n- " + strings.Join(parts, "；"))
	}
	return b.String()
}

// liteFlaggedStudents returns the students with at least one card, in
// roster order. A cards key with no matching student is still flagged (the
// model must write its card), with an empty name, after the roster, sorted
// by ID.
func liteFlaggedStudents(students []liteweekly.StudentWeek, cards map[string][]liteweekly.Card) []liteweekly.StudentWeek {
	var flagged []liteweekly.StudentWeek
	seen := map[string]bool{}
	for _, s := range students {
		if len(cards[s.UserID]) > 0 && !seen[s.UserID] {
			flagged = append(flagged, s)
			seen[s.UserID] = true
		}
	}
	var extra []string
	for id, cs := range cards {
		if len(cs) > 0 && !seen[id] {
			extra = append(extra, id)
		}
	}
	sort.Strings(extra)
	for _, id := range extra {
		flagged = append(flagged, liteweekly.StudentWeek{UserID: id})
	}
	return flagged
}

func validateLiteStudentWeekly(p LiteStudentWeeklyProse, s liteweekly.StudentWeek, weekLabel string, cards []liteweekly.Card, otherNames []string) error {
	if strings.TrimSpace(p.Summary) == "" {
		return errors.New("summary is empty")
	}
	if n := utf8.RuneCountInString(p.Summary); n > liteSummaryMax {
		return fmt.Errorf("summary has %d characters, over the %d limit", n, liteSummaryMax)
	}
	switch {
	case len(cards) == 0 && len(p.Suggestions) > 0:
		return fmt.Errorf("suggestions must be empty when there are no cards, got %d", len(p.Suggestions))
	case len(cards) > 0 && len(p.Suggestions) == 0:
		return fmt.Errorf("suggestions must have 1 to %d items when there are cards, got 0", liteSuggestionsMax)
	case len(p.Suggestions) > liteSuggestionsMax:
		return fmt.Errorf("suggestions has %d items, over the %d limit", len(p.Suggestions), liteSuggestionsMax)
	}
	for i, sg := range p.Suggestions {
		if strings.TrimSpace(sg.Text) == "" {
			return fmt.Errorf("suggestions[%d].text is empty", i)
		}
		if n := utf8.RuneCountInString(sg.Text); n > liteSuggestionMax {
			return fmt.Errorf("suggestions[%d].text has %d characters, over the %d limit", i, n, liteSuggestionMax)
		}
	}

	allowed := make([]string, 0, len(cards))
	for _, c := range cards {
		allowed = append(allowed, c.Code)
	}
	check := liteweekly.ProseCheck{
		AllowedCodes: allowed,
		Corpus:       liteweekly.Corpus(s),
		Titles:       liteweekly.TitleCorpus(s),
		FactsText:    liteweekly.FactsText(s, weekLabel),
		OtherNames:   otherNames,
	}
	// Each field is checked on its own so a quote mark opened in one field
	// cannot pair with a close in the next.
	if err := liteweekly.CheckProse(p.Summary, nil, check); err != nil {
		return fmt.Errorf("summary: %w", err)
	}
	for i, sg := range p.Suggestions {
		if err := liteweekly.CheckProse(sg.Text, []string{sg.EvidenceCode}, check); err != nil {
			return fmt.Errorf("suggestions[%d]: %w", i, err)
		}
	}
	return nil
}

func validateLiteClassWeekly(p LiteClassWeeklyProse, flagged []liteweekly.StudentWeek, facts string) error {
	if strings.TrimSpace(p.Comment) == "" {
		return errors.New("comment is empty")
	}
	if n := utf8.RuneCountInString(p.Comment); n > liteClassCommentMax {
		return fmt.Errorf("comment has %d characters, over the %d limit", n, liteClassCommentMax)
	}

	want := make(map[string]bool, len(flagged))
	for _, s := range flagged {
		want[s.UserID] = false
	}
	for i, c := range p.Cards {
		done, known := want[c.UserID]
		if !known {
			return fmt.Errorf("cards[%d] has userId %q, which is not in the student list", i, c.UserID)
		}
		if done {
			return fmt.Errorf("cards has userId %q more than once", c.UserID)
		}
		want[c.UserID] = true
		if strings.TrimSpace(c.Lead) == "" {
			return fmt.Errorf("card %q lead is empty", c.UserID)
		}
		if strings.TrimSpace(c.Action) == "" {
			return fmt.Errorf("card %q action is empty", c.UserID)
		}
		if n := utf8.RuneCountInString(c.Lead); n > liteClassLeadMax {
			return fmt.Errorf("card %q lead has %d characters, over the %d limit", c.UserID, n, liteClassLeadMax)
		}
		if n := utf8.RuneCountInString(c.Action); n > liteClassActionMax {
			return fmt.Errorf("card %q action has %d characters, over the %d limit", c.UserID, n, liteClassActionMax)
		}
	}
	for _, s := range flagged {
		if !want[s.UserID] {
			return fmt.Errorf("cards is missing userId %q", s.UserID)
		}
	}

	var corpora, titles []string
	for _, s := range flagged {
		if c := liteweekly.Corpus(s); c != "" {
			corpora = append(corpora, c)
		}
		if t := liteweekly.TitleCorpus(s); t != "" {
			titles = append(titles, t)
		}
	}
	check := liteweekly.ProseCheck{
		Corpus:    strings.Join(corpora, "\n"),
		Titles:    strings.Join(titles, "\n"),
		FactsText: facts,
	}
	if err := liteweekly.CheckProse(p.Comment, nil, check); err != nil {
		return fmt.Errorf("comment: %w", err)
	}
	for _, c := range p.Cards {
		if err := liteweekly.CheckProse(c.Lead, nil, check); err != nil {
			return fmt.Errorf("card %q lead: %w", c.UserID, err)
		}
		if err := liteweekly.CheckProse(c.Action, nil, check); err != nil {
			return fmt.Errorf("card %q action: %w", c.UserID, err)
		}
	}
	return nil
}
