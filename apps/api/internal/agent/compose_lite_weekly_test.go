package agent_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/liteweekly"
)

const liteWeekLabel = "9 月 1 日–9 月 7 日"

func liteReply(text string) []gateway.StreamEvent {
	return []gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: text},
		{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 100, OutputTokens: 50}},
		{Kind: gateway.EventDone, StopReason: gateway.StopStop},
	}
}

var liteResolved = gateway.Resolved{Provider: "stub", Model: "m"}

// liteLin has an overdue watch card and a new_interest praise card.
func liteLin() liteweekly.StudentWeek {
	return liteweekly.StudentWeek{
		UserID: "u1", Name: "林知遥",
		ActiveDays: 3, Minutes: 95, Turns: 14, PrevActiveDays: 5,
		Finished:           []liteweekly.Item{{Kind: "reading", Title: "城市里的雨水花园"}},
		AssignmentsOverdue: 1,
		Stalled:            []liteweekly.Item{{Kind: "writing", Title: "雨水花园调查报告"}},
		NewKeywords:        []string{"海绵城市"},
		Moments:            []liteweekly.Moment{{Quote: "雨水不是废水，是没被接住的资源", ItemTitle: "城市里的雨水花园"}},
	}
}

// liteZhou has only a stalled watch card.
func liteZhou() liteweekly.StudentWeek {
	return liteweekly.StudentWeek{
		UserID: "u2", Name: "周子墨",
		ActiveDays: 1, Minutes: 20, Turns: 4, PrevActiveDays: 1,
		Stalled: []liteweekly.Item{{Kind: "project", Title: "校园垃圾分类访谈"}},
	}
}

// liteChen has no cards.
func liteChen() liteweekly.StudentWeek {
	return liteweekly.StudentWeek{UserID: "u3", Name: "陈一", ActiveDays: 2, Minutes: 40, Turns: 6, PrevActiveDays: 2}
}

func liteCardsOf(s liteweekly.StudentWeek) []liteweekly.Card {
	var out []liteweekly.Card
	w, p := liteweekly.Cards(s)
	if w != nil {
		out = append(out, *w)
	}
	if p != nil {
		out = append(out, *p)
	}
	return out
}

const liteValidStudent = `{"summary":"本周活跃 3 天，对话 14 轮，读完《城市里的雨水花园》，写下「雨水不是废水，是没被接住的资源」。有 1 份作业逾期。","suggestions":[{"text":"请她说明逾期作业卡在哪一步，并约定完成时间。","evidenceCode":"overdue"},{"text":"请她讲一讲海绵城市和她读的文章有什么关系。","evidenceCode":"new_interest"}]}`

func composeLin(prov gateway.Provider) (agent.LiteStudentWeeklyProse, []agent.Attempt, error) {
	s := liteLin()
	return agent.ComposeLiteStudentWeekly(context.Background(), prov, liteResolved, s, liteWeekLabel, liteCardsOf(s), []string{"周子墨", "陈一"})
}

// (a)
func TestComposeLiteStudentWeeklyValidFirstReply(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(liteReply(liteValidStudent))
	got, attempts, err := composeLin(prov)
	if err != nil {
		t.Fatalf("ComposeLiteStudentWeekly: %v", err)
	}
	if len(attempts) != 1 || prov.Calls != 1 {
		t.Fatalf("attempts=%d calls=%d, want 1", len(attempts), prov.Calls)
	}
	if attempts[0].Err != nil || attempts[0].Usage.InputTokens != 100 || attempts[0].Text != liteValidStudent {
		t.Fatalf("attempt = %+v", attempts[0])
	}
	if len(got.Suggestions) != 2 || got.Suggestions[0].EvidenceCode != "overdue" {
		t.Fatalf("prose = %+v", got)
	}
}

// (b)
func TestComposeLiteStudentWeeklyRetriesFabricatedQuote(t *testing.T) {
	bad := `{"summary":"她写下「雨是天空的眼泪」。","suggestions":[{"text":"请她说明逾期作业卡在哪一步。","evidenceCode":"overdue"}]}`
	prov := gateway.NewSequenceStubProvider(liteReply(bad), liteReply(liteValidStudent))
	got, attempts, err := composeLin(prov)
	if err != nil {
		t.Fatalf("ComposeLiteStudentWeekly: %v", err)
	}
	if len(attempts) != 2 {
		t.Fatalf("attempts = %d, want 2", len(attempts))
	}
	if attempts[0].Err == nil || !strings.Contains(attempts[0].Err.Error(), "quote not in corpus: 雨是天空的眼泪") {
		t.Fatalf("first attempt err = %v", attempts[0].Err)
	}
	if attempts[1].Err != nil || attempts[1].Usage.OutputTokens != 50 {
		t.Fatalf("second attempt = %+v", attempts[1])
	}
	if !strings.HasPrefix(got.Summary, "本周活跃 3 天") {
		t.Fatalf("want the second reply's prose, got %+v", got)
	}
	msgs := prov.LastRequest.Messages
	if len(msgs) != 4 {
		t.Fatalf("retry request has %d messages, want system+user+assistant+user", len(msgs))
	}
	if msgs[2].Role != gateway.RoleAssistant || msgs[2].Content != bad {
		t.Fatalf("retry request must replay the first reply as an assistant turn, got %+v", msgs[2])
	}
	last := msgs[3]
	if last.Role != gateway.RoleUser || !strings.HasPrefix(last.Content, "上一次输出未通过校验：") ||
		!strings.Contains(last.Content, "quote not in corpus: 雨是天空的眼泪") || !strings.HasSuffix(last.Content, "。请重新输出。") {
		t.Fatalf("retry line = %q", last.Content)
	}
}

// (c)
func TestComposeLiteStudentWeeklyTwoInvalidReplies(t *testing.T) {
	bad := `{"summary":"她写下「雨是天空的眼泪」。","suggestions":[{"text":"请她说明逾期作业卡在哪一步。","evidenceCode":"overdue"}]}`
	prov := gateway.NewSequenceStubProvider(liteReply(bad))
	got, attempts, err := composeLin(prov)
	if err == nil {
		t.Fatalf("want error, got prose %+v", got)
	}
	if len(attempts) != 2 || prov.Calls != 2 {
		t.Fatalf("attempts=%d calls=%d, want 2", len(attempts), prov.Calls)
	}
	for i, a := range attempts {
		if a.Err == nil || a.Text != bad {
			t.Fatalf("attempt %d = %+v, want the rejected text and its error", i, a)
		}
	}
	if got.Summary != "" || got.Suggestions != nil {
		t.Fatalf("a failed compose must not return prose, got %+v", got)
	}
}

// (d)
func TestComposeLiteStudentWeeklyParsesFencedJSON(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(liteReply("```json\n" + liteValidStudent + "\n```"))
	got, attempts, err := composeLin(prov)
	if err != nil {
		t.Fatalf("ComposeLiteStudentWeekly: %v", err)
	}
	if len(attempts) != 1 || got.Summary == "" {
		t.Fatalf("attempts=%d prose=%+v", len(attempts), got)
	}
}

// (e)
func TestComposeLiteStudentWeeklyNoCards(t *testing.T) {
	s := liteChen()
	if len(liteCardsOf(s)) != 0 {
		t.Fatal("fixture must have no cards")
	}
	compose := func(reply string) ([]agent.Attempt, error) {
		_, attempts, err := agent.ComposeLiteStudentWeekly(context.Background(), gateway.NewSequenceStubProvider(liteReply(reply)), liteResolved, s, liteWeekLabel, nil, []string{"林知遥"})
		return attempts, err
	}

	if attempts, err := compose(`{"summary":"本周活跃 2 天，对话 6 轮。","suggestions":[]}`); err != nil || len(attempts) != 1 {
		t.Fatalf("empty suggestions with no cards: attempts=%d err=%v", len(attempts), err)
	}
	attempts, err := compose(`{"summary":"本周活跃 2 天。","suggestions":[{"text":"请继续保持。","evidenceCode":""}]}`)
	if err == nil || len(attempts) != 2 {
		t.Fatalf("a suggestion without cards must fail twice: attempts=%d err=%v", len(attempts), err)
	}
	if !strings.Contains(err.Error(), "suggestions must be empty") {
		t.Fatalf("err = %v", err)
	}
}

// Single replies that must be rejected on both attempts, including (g).
func TestComposeLiteStudentWeeklyRejects(t *testing.T) {
	sug := `[{"text":"请她说明逾期作业卡在哪一步。","evidenceCode":"overdue"}]`
	cases := []struct {
		name, reply, want string
	}{
		{"title used as her quote", `{"summary":"她读完了「城市里的雨水花园」。","suggestions":` + sug + `}`, "quote not in corpus: 城市里的雨水花园"},
		{"title not in her week", `{"summary":"她读完了《咖啡的历史》。","suggestions":` + sug + `}`, "title not in titles"},
		{"digit not in facts", `{"summary":"本周对话 20 轮。","suggestions":` + sug + `}`, "digit not in facts: 20"},
		{"other student named", `{"summary":"她和周子墨一起讨论了文章。","suggestions":` + sug + `}`, "mentions other student: 周子墨"},
		{"code not among cards", `{"summary":"本周活跃 3 天。","suggestions":[{"text":"请跟进。","evidenceCode":"stalled"}]}`, "unknown evidence code: stalled"},
		{"cards but no suggestions", `{"summary":"本周活跃 3 天。","suggestions":[]}`, "suggestions must have 1 to 3"},
		{"four suggestions", `{"summary":"本周活跃 3 天。","suggestions":[{"text":"一","evidenceCode":"overdue"},{"text":"二","evidenceCode":"overdue"},{"text":"三","evidenceCode":"overdue"},{"text":"四","evidenceCode":"overdue"}]}`, "over the 3 limit"},
		{"summary over 150 characters", `{"summary":"` + strings.Repeat("很", 151) + `","suggestions":` + sug + `}`, "summary has 151 characters"},
		{"suggestion over 120 characters", `{"summary":"本周活跃 3 天。","suggestions":[{"text":"` + strings.Repeat("很", 121) + `","evidenceCode":"overdue"}]}`, "suggestions[0].text has 121 characters"},
		{"empty summary", `{"summary":" ","suggestions":` + sug + `}`, "summary is empty"},
		{"unclosed quote", `{"summary":"她写下「雨水不是废水","suggestions":` + sug + `}`, "unclosed quote"},
		{"not JSON", `本周表现不错。`, "not valid JSON"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, attempts, err := composeLin(gateway.NewSequenceStubProvider(liteReply(tc.reply)))
			if err == nil {
				t.Fatal("want rejection")
			}
			if len(attempts) != 2 {
				t.Fatalf("attempts = %d, want 2", len(attempts))
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want it to contain %q", err, tc.want)
			}
		})
	}
}

// 150 CJK characters is 450 bytes; the cap counts characters.
func TestComposeLiteStudentWeeklySummaryAtLimitPasses(t *testing.T) {
	reply := `{"summary":"` + strings.Repeat("很", 150) + `","suggestions":[{"text":"请她说明逾期作业卡在哪一步。","evidenceCode":"overdue"}]}`
	if _, _, err := composeLin(gateway.NewSequenceStubProvider(liteReply(reply))); err != nil {
		t.Fatalf("a 150-character summary must pass: %v", err)
	}
}

type liteFailingProvider struct{ calls int }

func (p *liteFailingProvider) Stream(context.Context, gateway.Resolved, gateway.ChatRequest) (<-chan gateway.StreamEvent, error) {
	p.calls++
	return nil, errors.New("upstream 502")
}

// A transport error is not something the model can fix by retrying with a
// validation message; it surfaces at once and no prose is invented.
func TestComposeLiteStudentWeeklyTransportErrorNoRetry(t *testing.T) {
	prov := &liteFailingProvider{}
	got, attempts, err := composeLin(prov)
	if err == nil || !strings.Contains(err.Error(), "upstream 502") {
		t.Fatalf("err = %v", err)
	}
	if prov.calls != 1 || len(attempts) != 1 || attempts[0].Err == nil {
		t.Fatalf("calls=%d attempts=%+v", prov.calls, attempts)
	}
	if got.Summary != "" {
		t.Fatalf("no prose on failure, got %+v", got)
	}
}

// ---- class ----

var liteStats = liteweekly.ClassWeekStats{ClassSize: 3, ActiveStudents: 3, Minutes: 155, Turns: 24, Finished: 1, AssignmentRate: 50}

func liteClassInputs() ([]liteweekly.StudentWeek, map[string][]liteweekly.Card) {
	students := []liteweekly.StudentWeek{liteLin(), liteZhou(), liteChen()}
	cards := map[string][]liteweekly.Card{
		"u1": liteCardsOf(liteLin()),
		"u2": liteCardsOf(liteZhou()),
		"u3": {}, // present but empty: not flagged
	}
	return students, cards
}

func composeClass(prov gateway.Provider) (agent.LiteClassWeeklyProse, []agent.Attempt, error) {
	students, cards := liteClassInputs()
	return agent.ComposeLiteClassWeekly(context.Background(), prov, liteResolved, "IBDP 一年级", liteWeekLabel, liteStats, students, cards, []string{"林知遥", "周子墨", "陈一"})
}

const (
	liteClassComment = `"comment":"本周 3 名学生都有学习记录，全班对话 24 轮，作业完成率 50%。"`
	liteCardU1       = `{"userId":"u1","lead":"林知遥读完《城市里的雨水花园》，写下「雨水不是废水，是没被接住的资源」，有 1 份作业未完成。","action":"请线下问她逾期作业卡在哪一步。"}`
	liteCardU2       = `{"userId":"u2","lead":"周子墨的《校园垃圾分类访谈》超过 7 天没有进展。","action":"请线下了解访谈遇到了什么困难。"}`
	liteValidClass   = `{` + liteClassComment + `,"cards":[` + liteCardU1 + `,` + liteCardU2 + `]}`
)

// (a) — also: the user message carries exactly the facts text the digit
// check uses, so the model sees every number it is allowed to write.
func TestComposeLiteClassWeeklyValidFirstReply(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(liteReply(liteValidClass))
	got, attempts, err := composeClass(prov)
	if err != nil {
		t.Fatalf("ComposeLiteClassWeekly: %v", err)
	}
	if len(attempts) != 1 || len(got.Cards) != 2 {
		t.Fatalf("attempts=%d prose=%+v", len(attempts), got)
	}
	students, cards := liteClassInputs()
	facts := liteweekly.ClassFactsText("IBDP 一年级", liteWeekLabel, liteStats, students[:2], cards)
	user := prov.LastRequest.Messages[1].Content
	if !strings.HasPrefix(user, facts) {
		t.Fatalf("user message must start with ClassFactsText:\n%s\n--- want prefix ---\n%s", user, facts)
	}
	if !strings.Contains(user, "userId=u1") || !strings.Contains(user, "userId=u2") || strings.Contains(user, "userId=u3") {
		t.Fatalf("student list must name exactly the flagged students:\n%s", user)
	}
}

// (b)
func TestComposeLiteClassWeeklyRetriesFabricatedQuote(t *testing.T) {
	bad := `{` + liteClassComment + `,"cards":[{"userId":"u1","lead":"林知遥写下「雨是天空的眼泪」。","action":"请线下问她。"},` + liteCardU2 + `]}`
	prov := gateway.NewSequenceStubProvider(liteReply(bad), liteReply(liteValidClass))
	_, attempts, err := composeClass(prov)
	if err != nil {
		t.Fatalf("ComposeLiteClassWeekly: %v", err)
	}
	if len(attempts) != 2 || attempts[0].Err == nil || attempts[1].Err != nil {
		t.Fatalf("attempts = %+v", attempts)
	}
	last := prov.LastRequest.Messages[len(prov.LastRequest.Messages)-1]
	if !strings.Contains(last.Content, "上一次输出未通过校验") || !strings.Contains(last.Content, "雨是天空的眼泪") {
		t.Fatalf("retry line = %q", last.Content)
	}
}

// (c)
func TestComposeLiteClassWeeklyTwoInvalidReplies(t *testing.T) {
	bad := `{` + liteClassComment + `,"cards":[{"userId":"u1","lead":"林知遥写下「雨是天空的眼泪」。","action":"请线下问她。"},` + liteCardU2 + `]}`
	prov := gateway.NewSequenceStubProvider(liteReply(bad))
	_, attempts, err := composeClass(prov)
	if err == nil || len(attempts) != 2 || prov.Calls != 2 {
		t.Fatalf("err=%v attempts=%d calls=%d", err, len(attempts), prov.Calls)
	}
}

// (d)
func TestComposeLiteClassWeeklyParsesFencedJSON(t *testing.T) {
	_, attempts, err := composeClass(gateway.NewSequenceStubProvider(liteReply("```json\n" + liteValidClass + "\n```")))
	if err != nil || len(attempts) != 1 {
		t.Fatalf("err=%v attempts=%d", err, len(attempts))
	}
}

// (f)
func TestComposeLiteClassWeeklyRetriesMissingCard(t *testing.T) {
	missing := `{` + liteClassComment + `,"cards":[` + liteCardU1 + `]}`
	prov := gateway.NewSequenceStubProvider(liteReply(missing), liteReply(liteValidClass))
	got, attempts, err := composeClass(prov)
	if err != nil {
		t.Fatalf("ComposeLiteClassWeekly: %v", err)
	}
	if len(attempts) != 2 || len(got.Cards) != 2 {
		t.Fatalf("attempts=%d prose=%+v", len(attempts), got)
	}
	last := prov.LastRequest.Messages[len(prov.LastRequest.Messages)-1]
	if !strings.Contains(last.Content, `cards is missing userId "u2"`) {
		t.Fatalf("retry line = %q", last.Content)
	}
}

func TestComposeLiteClassWeeklyRejects(t *testing.T) {
	cases := []struct {
		name, reply, want string
	}{
		{"unflagged student", `{` + liteClassComment + `,"cards":[` + liteCardU1 + `,` + liteCardU2 + `,{"userId":"u3","lead":"陈一本周活跃。","action":"请鼓励她。"}]}`, `userId "u3", which is not in the student list`},
		{"repeated student", `{` + liteClassComment + `,"cards":[` + liteCardU1 + `,` + liteCardU1 + `,` + liteCardU2 + `]}`, "more than once"},
		{"title used as her quote", `{` + liteClassComment + `,"cards":[` + liteCardU1 + `,{"userId":"u2","lead":"周子墨的「校园垃圾分类访谈」没有进展。","action":"请线下了解。"}]}`, "quote not in corpus: 校园垃圾分类访谈"},
		{"digit not in facts", `{"comment":"全班对话 31 轮。","cards":[` + liteCardU1 + `,` + liteCardU2 + `]}`, "digit not in facts: 31"},
		{"empty action", `{` + liteClassComment + `,"cards":[` + liteCardU1 + `,{"userId":"u2","lead":"周子墨没有进展。","action":""}]}`, `card "u2" action is empty`},
		{"lead over 120 characters", `{` + liteClassComment + `,"cards":[` + liteCardU1 + `,{"userId":"u2","lead":"` + strings.Repeat("很", 121) + `","action":"请线下了解。"}]}`, "lead has 121 characters"},
		{"comment over 300 characters", `{"comment":"` + strings.Repeat("很", 301) + `","cards":[` + liteCardU1 + `,` + liteCardU2 + `]}`, "comment has 301 characters"},
		{"empty comment", `{"comment":"","cards":[` + liteCardU1 + `,` + liteCardU2 + `]}`, "comment is empty"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, attempts, err := composeClass(gateway.NewSequenceStubProvider(liteReply(tc.reply)))
			if err == nil {
				t.Fatal("want rejection")
			}
			if len(attempts) != 2 {
				t.Fatalf("attempts = %d, want 2", len(attempts))
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want it to contain %q", err, tc.want)
			}
		})
	}
}

// With nobody flagged, cards must be empty and the name check stays off:
// every student in the class may be named.
func TestComposeLiteClassWeeklyNobodyFlagged(t *testing.T) {
	reply := `{"comment":"林知遥、周子墨和陈一本周都有学习记录。","cards":[]}`
	_, attempts, err := agent.ComposeLiteClassWeekly(context.Background(), gateway.NewSequenceStubProvider(liteReply(reply)), liteResolved,
		"IBDP 一年级", liteWeekLabel, liteStats, []liteweekly.StudentWeek{liteChen()}, map[string][]liteweekly.Card{}, []string{"林知遥", "周子墨", "陈一"})
	if err != nil || len(attempts) != 1 {
		t.Fatalf("err=%v attempts=%d", err, len(attempts))
	}
}
