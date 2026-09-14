package agent_test

import (
	"context"
	"encoding/json"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/liteparent"
)

// parentFacts: the range dates (2026, 08, 17, 09, 13, 8, 9, 28) share no digit
// run with the counts (5, 64, 31, 2, 1, 0), so a count the prose invents cannot
// pass by matching a date.
func parentFacts() liteparent.Facts {
	return liteparent.Facts{
		StudentName: "林知遥", ClassName: "IBDP 一年级", TeacherName: "王老师",
		RangeStart: "2026-08-17", RangeEnd: "2026-09-13", Days: 28,
		ActiveDays: 5, Minutes: 64, Turns: 31,
		Readings:         []liteparent.Item{{Kind: "reading", Title: "城市里的雨水花园", FinishedAt: "2026-08-20"}},
		Writings:         []liteparent.Item{{Kind: "writing", Title: "雨水花园调查报告", FinishedAt: "2026-09-01"}},
		Moments:          []liteparent.Moment{{Quote: "雨水不是废水，是没被接住的资源", ItemTitle: "城市里的雨水花园"}},
		AssignmentsTotal: 2, AssignmentsOnTime: 1, AssignmentsLate: 1,
		Keywords: []liteparent.Keyword{{Text: "海绵城市", Field: "society", FieldLabel: "社会与世界"}},
	}
}

func parentValidSections() map[string]string {
	return map[string]string{
		"overview":  "林知遥在8月17日至9月13日活跃 5 天，学习 64 分钟，对话 31 轮。到期作业 2 份，按时完成 1 份，逾期完成 1 份。",
		"reading":   "她读完《城市里的雨水花园》，写下「雨水不是废水，是没被接住的资源」。",
		"writing":   "她完成了《雨水花园调查报告》。",
		"interests": "兴趣树新增关键词「海绵城市」。",
		"next":      "请她讲一讲《城市里的雨水花园》里的例子；请和她一起安排作业的完成时间。",
	}
}

func parentReply(t *testing.T, sections map[string]string) string {
	t.Helper()
	b, err := json.Marshal(sections)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func withSection(base map[string]string, key, value string) map[string]string {
	out := make(map[string]string, len(base)+1)
	for k, v := range base {
		out[k] = v
	}
	out[key] = value
	return out
}

var classmates = []string{"王小明", "周子墨"}

func composeParent(prov gateway.Provider, f liteparent.Facts, others []string) (map[string]string, []agent.Attempt, error) {
	return agent.ComposeLiteParentReport(context.Background(), prov, liteResolved, f, others)
}

func TestComposeLiteParentReportValidFirstReply(t *testing.T) {
	f := parentFacts()
	reply := parentReply(t, parentValidSections())
	prov := gateway.NewSequenceStubProvider(liteReply(reply))
	got, attempts, err := composeParent(prov, f, classmates)
	if err != nil {
		t.Fatalf("ComposeLiteParentReport: %v", err)
	}
	if len(attempts) != 1 || prov.Calls != 1 || attempts[0].Err != nil || attempts[0].Text != reply || attempts[0].Usage.InputTokens != 100 {
		t.Fatalf("attempts=%+v calls=%d", attempts, prov.Calls)
	}
	if !reflect.DeepEqual(got, parentValidSections()) {
		t.Fatalf("sections = %+v", got)
	}
	msgs := prov.LastRequest.Messages
	if len(msgs) != 2 {
		t.Fatalf("messages = %d, want system+user", len(msgs))
	}
	sys := msgs[0].Content
	for _, want := range []string{
		"键为 overview,reading,writing,interests,next（只输出这些键）",
		"作品标题用《》，只有学生原话用「」。",
		"数字一律用阿拉伯数字。",
		"不做加减和单位换算，数字照抄给出的事实。",
		"列举多条时不编号，每条单独一行。",
	} {
		if !strings.Contains(sys, want) {
			t.Fatalf("system prompt lacks %q:\n%s", want, sys)
		}
	}
	if strings.Contains(sys, "{sections}") || strings.Contains(sys, "本周") {
		t.Fatalf("system prompt = %s", sys)
	}
	// The user message is the facts text and nothing else, so every digit the
	// model sees is in the digit set.
	if msgs[1].Role != gateway.RoleUser || msgs[1].Content != liteparent.FactsText(f) {
		t.Fatalf("user message = %q, want FactsText verbatim", msgs[1].Content)
	}
}

func TestComposeLiteParentReportRetriesMissingSection(t *testing.T) {
	missing := parentValidSections()
	delete(missing, "writing")
	bad := parentReply(t, missing)
	prov := gateway.NewSequenceStubProvider(liteReply(bad), liteReply(parentReply(t, parentValidSections())))
	got, attempts, err := composeParent(prov, parentFacts(), classmates)
	if err != nil {
		t.Fatalf("ComposeLiteParentReport: %v", err)
	}
	if len(attempts) != 2 || attempts[1].Err != nil {
		t.Fatalf("attempts = %+v", attempts)
	}
	if attempts[0].Err == nil || !strings.Contains(attempts[0].Err.Error(), "missing section: writing") {
		t.Fatalf("first attempt err = %v", attempts[0].Err)
	}
	if got["writing"] == "" {
		t.Fatalf("want the second reply's sections, got %+v", got)
	}
	msgs := prov.LastRequest.Messages
	if len(msgs) != 4 || msgs[2].Role != gateway.RoleAssistant || msgs[2].Content != bad {
		t.Fatalf("retry request must replay the first reply as an assistant turn, got %+v", msgs)
	}
	if want := "上一次输出未通过校验：missing section: writing。请重新输出。"; msgs[3].Role != gateway.RoleUser || msgs[3].Content != want {
		t.Fatalf("retry line = %q, want %q", msgs[3].Content, want)
	}
}

func TestComposeLiteParentReportRetriesExtraSection(t *testing.T) {
	bad := parentReply(t, withSection(parentValidSections(), "hobbies", "她喜欢画画。"))
	prov := gateway.NewSequenceStubProvider(liteReply(bad), liteReply(parentReply(t, parentValidSections())))
	got, attempts, err := composeParent(prov, parentFacts(), classmates)
	if err != nil {
		t.Fatalf("ComposeLiteParentReport: %v", err)
	}
	if len(attempts) != 2 || attempts[0].Err == nil || !strings.Contains(attempts[0].Err.Error(), "unexpected section: hobbies") {
		t.Fatalf("attempts = %+v", attempts)
	}
	if _, ok := got["hobbies"]; ok {
		t.Fatalf("sections = %+v", got)
	}
}

func TestComposeLiteParentReportFabricatedQuoteTwice(t *testing.T) {
	bad := parentReply(t, withSection(parentValidSections(), "reading", "她写下「雨是天空的眼泪」。"))
	prov := gateway.NewSequenceStubProvider(liteReply(bad))
	got, attempts, err := composeParent(prov, parentFacts(), classmates)
	if err == nil || !strings.Contains(err.Error(), "reading: quote not in corpus: 雨是天空的眼泪") {
		t.Fatalf("err = %v", err)
	}
	if len(attempts) != 2 || prov.Calls != 2 {
		t.Fatalf("attempts=%d calls=%d, want 2", len(attempts), prov.Calls)
	}
	for i, a := range attempts {
		if a.Err == nil || a.Text != bad {
			t.Fatalf("attempt %d = %+v", i, a)
		}
	}
	if got != nil {
		t.Fatalf("a failed compose must not return sections, got %+v", got)
	}
}

// Replies rejected on both attempts, including (c): a title used as her quote.
func TestComposeLiteParentReportRejects(t *testing.T) {
	valid := parentValidSections()
	cases := []struct {
		name, reply, want string
	}{
		{"(c) title used as her quote", parentReply(t, withSection(valid, "reading", "她写下「城市里的雨水花园」。")), "reading: quote not in corpus: 城市里的雨水花园"},
		{"title not in her range", parentReply(t, withSection(valid, "reading", "她读完了《咖啡的历史》。")), "title not in titles: 咖啡的历史"},
		{"invented count", parentReply(t, withSection(valid, "overview", "这段时间对话 30 轮。")), "overview: digit not in facts: 30"},
		{"classmate named", parentReply(t, withSection(valid, "writing", "她和王小明一起完成了《雨水花园调查报告》。")), "writing: mentions other student: 王小明"},
		{"section with no facts", parentReply(t, withSection(valid, "projects", "没有完成项目。")), "unexpected section: projects"},
		{"empty section", parentReply(t, withSection(valid, "next", " ")), "section next is empty"},
		{"section over 400 characters", parentReply(t, withSection(valid, "overview", strings.Repeat("很", 401))), "section overview has 401 characters, over the 400 limit"},
		{"unclosed quote", parentReply(t, withSection(valid, "reading", "她写下「雨水不是废水")), "reading: unclosed quote"},
		{"not JSON", "这段时间表现不错。", "not valid JSON"},
		{"non-string value", `{"overview":["活跃 5 天"],"reading":"a","writing":"b","interests":"c","next":"d"}`, "not valid JSON"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, attempts, err := composeParent(gateway.NewSequenceStubProvider(liteReply(tc.reply)), parentFacts(), classmates)
			if err == nil {
				t.Fatalf("want rejection, got %+v", got)
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

// 400 CJK characters is 1200 bytes; the cap counts characters.
func TestComposeLiteParentReportSectionAtLimitPasses(t *testing.T) {
	reply := parentReply(t, withSection(parentValidSections(), "overview", strings.Repeat("很", 400)))
	if _, _, err := composeParent(gateway.NewSequenceStubProvider(liteReply(reply)), parentFacts(), classmates); err != nil {
		t.Fatalf("a 400-character section must pass: %v", err)
	}
}

// A report with only overview and next asks for exactly those two keys.
func TestComposeLiteParentReportQuietRange(t *testing.T) {
	f := liteparent.Facts{StudentName: "林知遥", RangeStart: "2026-08-17", RangeEnd: "2026-09-13", Days: 28, Minutes: -1}
	reply := `{"overview":"林知遥在这段时间没有学习记录。","next":"请和她聊一聊最近想读的文章。"}`
	prov := gateway.NewSequenceStubProvider(liteReply(reply))
	if _, attempts, err := composeParent(prov, f, nil); err != nil || len(attempts) != 1 {
		t.Fatalf("err=%v attempts=%d", err, len(attempts))
	}
	if !strings.Contains(prov.LastRequest.Messages[0].Content, "键为 overview,next（") {
		t.Fatalf("system prompt = %s", prov.LastRequest.Messages[0].Content)
	}
}

// Ruling 16: a classmate whose name is part of hers (王丽 in 王丽华) does not
// fail prose that names her.
func TestComposeLiteParentReportClassmateNameInsideHers(t *testing.T) {
	f := parentFacts()
	f.StudentName = "王丽华"
	reply := parentReply(t, withSection(parentValidSections(), "overview", "王丽华这段时间活跃 5 天，对话 31 轮。"))
	_, attempts, err := composeParent(gateway.NewSequenceStubProvider(liteReply(reply)), f, []string{"王丽", "周子墨"})
	if err != nil || len(attempts) != 1 {
		t.Fatalf("err=%v attempts=%d, want a first-attempt pass", err, len(attempts))
	}
}

// A classmate named inside her verified 金句 is her words, not the prose
// naming the classmate; the same name outside a span fails.
func TestComposeLiteParentReportClassmateInsideVerifiedQuote(t *testing.T) {
	f := parentFacts()
	f.Moments = append(f.Moments, liteparent.Moment{Quote: "周子墨说雨水能直接喝，我不同意", ItemTitle: "城市里的雨水花园"})
	compose := func(reading string) ([]agent.Attempt, error) {
		reply := parentReply(t, withSection(parentValidSections(), "reading", reading))
		_, attempts, err := composeParent(gateway.NewSequenceStubProvider(liteReply(reply)), f, classmates)
		return attempts, err
	}
	if attempts, err := compose("她写下「周子墨说雨水能直接喝，我不同意」。"); err != nil || len(attempts) != 1 {
		t.Fatalf("name inside a verified quote: err=%v attempts=%d, want a first-attempt pass", err, len(attempts))
	}
	if _, err := compose("她和周子墨讨论了《城市里的雨水花园》。"); err == nil || !strings.Contains(err.Error(), "mentions other student: 周子墨") {
		t.Fatalf("name outside spans: err=%v", err)
	}
}

// Digit-check scope: the range dates share no digit run with the counts, so a
// real count and either date spelling pass, and an invented count fails.
func TestComposeLiteParentReportDigitsComeFromFacts(t *testing.T) {
	f := parentFacts()
	facts := liteparent.FactsText(f)
	for _, real := range []string{"2026-08-17", "8月17日", "9月13日", "活跃 5 天", "学习 64 分钟", "对话 31 轮"} {
		if !strings.Contains(facts, real) {
			t.Fatalf("fixture facts text lacks %q:\n%s", real, facts)
		}
	}
	if runs := regexp.MustCompile(`[0-9]+`).FindAllString(facts, -1); slices.Contains(runs, "7") || slices.Contains(runs, "30") {
		t.Fatalf("fixture facts text must not carry the invented counts:\n%s", facts)
	}
	compose := func(overview string) error {
		reply := parentReply(t, withSection(parentValidSections(), "overview", overview))
		_, _, err := composeParent(gateway.NewSequenceStubProvider(liteReply(reply)), f, classmates)
		return err
	}
	if err := compose("2026-08-17 至 9月13日，林知遥学习 64 分钟，对话 31 轮，活跃 5 天。"); err != nil {
		t.Fatalf("real counts must pass: %v", err)
	}
	if err := compose("这段时间她读了 7 篇文章。"); err == nil || !strings.Contains(err.Error(), "digit not in facts: 7") {
		t.Fatalf("invented count: err=%v", err)
	}
}

func TestComposeLiteParentReportTransportErrorNoRetry(t *testing.T) {
	prov := &liteFailingProvider{}
	got, attempts, err := composeParent(prov, parentFacts(), classmates)
	if err == nil || !strings.Contains(err.Error(), "upstream 502") {
		t.Fatalf("err = %v", err)
	}
	if prov.calls != 1 || len(attempts) != 1 || attempts[0].Err == nil || got != nil {
		t.Fatalf("calls=%d attempts=%+v got=%+v", prov.calls, attempts, got)
	}
}

// Ruling 9: a model that numbers the next items anyway still passes when the
// facts carry no 1, 2 or 3, and the stored section keeps its numbering.
func TestComposeLiteParentReportNumberedNextPasses(t *testing.T) {
	f := liteparent.Facts{
		StudentName: "林知遥", RangeStart: "2026-08-17", RangeEnd: "2026-09-13", Days: 28,
		ActiveDays: 5, Minutes: 64, Turns: 40,
		Keywords: []liteparent.Keyword{{Text: "海绵城市", Field: "society", FieldLabel: "社会与世界"}},
	}
	runs := regexp.MustCompile(`[0-9]+`).FindAllString(liteparent.FactsText(f), -1)
	for _, d := range []string{"1", "2", "3"} {
		if slices.Contains(runs, d) {
			t.Fatalf("fixture facts text must not carry %s: %v", d, runs)
		}
	}
	next := "1. 请和她聊一聊海绵城市。\n2. 请每周陪她读一篇文章。\n3. 请和她一起安排学习时间。"
	sections := map[string]string{
		"overview":  "林知遥这段时间活跃 5 天，学习 64 分钟，对话 40 轮。",
		"interests": "兴趣树新增关键词「海绵城市」。",
		"next":      next,
	}
	got, attempts, err := composeParent(gateway.NewSequenceStubProvider(liteReply(parentReply(t, sections))), f, classmates)
	if err != nil || len(attempts) != 1 {
		t.Fatalf("err=%v attempts=%d, want a first-attempt pass", err, len(attempts))
	}
	if got["next"] != next {
		t.Fatalf("next = %q, want the model's text unchanged", got["next"])
	}
}

func TestComposeLiteParentReportParsesFencedJSON(t *testing.T) {
	reply := "```json\n" + parentReply(t, parentValidSections()) + "\n```"
	if _, attempts, err := composeParent(gateway.NewSequenceStubProvider(liteReply(reply)), parentFacts(), classmates); err != nil || len(attempts) != 1 {
		t.Fatalf("err=%v attempts=%d", err, len(attempts))
	}
}
