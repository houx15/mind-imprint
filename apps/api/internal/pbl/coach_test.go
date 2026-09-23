package pbl

import (
	"context"
	"mindimprint/api/internal/gateway"
	"strings"
	"testing"
)

// An assigned project's idea is the teacher's driving question. 印记 sees it,
// labelled as the teacher's, with the teacher's 补充说明; it is never
// introduced as something she said. Her own project keeps the old line.
func TestBuildCoachContext_AssignedProjectIsTheTeachersQuestion(t *testing.T) {
	assigned := buildCoachContext(CoachInput{
		Idea: "怎样让校园少用一次性杯子？", Assigned: true, AssignedBrief: "先统计一周食堂用掉的杯子",
	})
	for _, want := range []string{"老师布置的驱动问题：怎样让校园少用一次性杯子？", "老师补充说明：先统计一周食堂用掉的杯子"} {
		if !strings.Contains(assigned, want) {
			t.Errorf("assigned context is missing %q:\n%s", want, assigned)
		}
	}
	if strings.Contains(assigned, "学生一开始是这么说的") {
		t.Errorf("assigned context introduces the teacher's question as her words:\n%s", assigned)
	}

	own := buildCoachContext(CoachInput{Idea: "下课没人去操场"})
	if !strings.Contains(own, "学生一开始是这么说的：下课没人去操场") {
		t.Errorf("her own project lost its opening line:\n%s", own)
	}
	if strings.Contains(own, "老师") {
		t.Errorf("her own project mentions a teacher:\n%s", own)
	}
}

// Same rule in the lookback, which quotes the opening back to her.
func TestBuildLookbackContext_AssignedProjectIsTheTeachersQuestion(t *testing.T) {
	assigned := buildLookbackContext(LookbackInput{
		Name: "杯子", Idea: "怎样让校园少用一次性杯子？", Assigned: true, AssignedBrief: "先统计一周食堂用掉的杯子",
	})
	for _, want := range []string{"老师布置的驱动问题：怎样让校园少用一次性杯子？", "老师补充说明：先统计一周食堂用掉的杯子"} {
		if !strings.Contains(assigned, want) {
			t.Errorf("assigned lookback is missing %q:\n%s", want, assigned)
		}
	}
	if strings.Contains(assigned, "学生一开始是这么说的") {
		t.Errorf("assigned lookback introduces the teacher's question as her words:\n%s", assigned)
	}
	// No brief: no 补充说明 line at all, not an empty one.
	if noBrief := buildLookbackContext(LookbackInput{Idea: "怎样让校园少用一次性杯子？", Assigned: true}); strings.Contains(noBrief, "老师补充说明") {
		t.Errorf("empty brief still rendered:\n%s", noBrief)
	}
	// No records either: the fallback points at the teacher's question, not
	// at "他最初那句话", which she never said.
	if noRecords := buildLookbackContext(LookbackInput{Idea: "怎样让校园少用一次性杯子？", Assigned: true}); !strings.Contains(noRecords, "就着老师布置的驱动问题问") || strings.Contains(noRecords, "最初那句话") {
		t.Errorf("assigned lookback with no records points at her words:\n%s", noRecords)
	}

	own := buildLookbackContext(LookbackInput{Name: "操场", Idea: "下课没人去操场"})
	if !strings.Contains(own, "学生一开始是这么说的：下课没人去操场") {
		t.Errorf("her own project lost its opening line:\n%s", own)
	}
}

func TestCoachRepairsContractAndCountsBothCalls(t *testing.T) {
	script := func(text string) []gateway.StreamEvent {
		return []gateway.StreamEvent{
			{Kind: gateway.EventTextDelta, TextDelta: text},
			{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 10, OutputTokens: 20}},
			{Kind: gateway.EventDone, StopReason: gateway.StopStop},
		}
	}
	provider := gateway.NewSequenceStubProvider(
		script(`{"reply":"计划已生成","plan":{"steps":[]}}`),
		script(`{"reply":"请检查计划","produce":{"kind":"plan","payload":{"steps":[{"title":"观察","decide":"记录什么"}]}}}`),
	)
	out, usage, err := Coach(context.Background(), provider, gateway.Resolved{}, CoachInput{Idea: "食堂剩餐"})
	if err != nil || out.Produce == nil || out.Produce.Kind != "plan" {
		t.Fatalf("repair failed: %+v %v", out, err)
	}
	if usage.InputTokens != 20 || usage.OutputTokens != 40 {
		t.Fatalf("retry usage lost: %+v", usage)

	}
	for _, request := range provider.Requests {
		if request.ResponseFormat != gateway.ResponseFormatJSONObject {
			t.Fatal("initial or repair request lost JSON output constraint")
		}
	}
}

func TestParseCoachOutput(t *testing.T) {
	cases := []struct {
		name     string
		in       string
		wantHook string
		wantKind string
		bad      bool
	}{
		{
			name:     "reply with a hook",
			in:       `{"reply":"先说说你看到的。","hook":"剩的到底是米饭还是菜？","hook_kind":"free"}`,
			wantHook: "剩的到底是米饭还是菜？", wantKind: "free",
		},
		{
			name: "reply with no hook",
			in:   `{"reply":"我把三天的记录整理好了。","hook":"","hook_kind":""}`,
		},
		{
			// A hook naming a kind we do not have is dropped, not fatal: the
			// reply is still worth showing, and a missing hook costs her far
			// less than a dead turn.
			name: "unknown hook kind drops the hook, keeps the reply",
			in:   `{"reply":"好。","hook":"要不要想想？","hook_kind":"retro"}`,
		},
		{
			// plan_check is opened by a staged structural change, never by a
			// hook — a different trigger entirely.
			name: "plan_check may not be opened by a hook",
			in:   `{"reply":"好。","hook":"计划要不要改？","hook_kind":"plan_check"}`,
		},
		{"fenced", "```json\n{\"reply\":\"好。\"}\n```", "", "", false},
		{"empty reply is a failure", `{"reply":"  ","hook":"x"}`, "", "", true},
		{"not json", `好的`, "", "", true},
		{"empty", ``, "", "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := parseCoachOutput(c.in)
			if c.bad {
				if err == nil {
					t.Fatalf("parseCoachOutput(%q) = %+v, want an error", c.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseCoachOutput(%q): %v", c.in, err)
			}
			if got.Reply == "" {
				t.Fatal("reply is empty")
			}
			if got.Hook != c.wantHook {
				t.Fatalf("hook = %q, want %q", got.Hook, c.wantHook)
			}
			if got.HookKind != c.wantKind {
				t.Fatalf("hookKind = %q, want %q", got.HookKind, c.wantKind)
			}
		})
	}
}

func TestCoachRejectsSilentlyLostActions(t *testing.T) {
	for _, raw := range []string{
		`{"reply":"计划已生成","plan":{"steps":[]}}`,
		`{"reply":"计划已生成","produce":{"kind":"plan","payload":null}}`,
		`{"reply":"成果已生成","produce":{"kind":"unknown","payload":{}}}`,
	} {
		if _, err := parseCoachOutput(raw); err == nil {
			t.Fatalf("accepted a reply while losing its action: %s", raw)
		}
	}
}

// Envelope failures must be distinguishable so the model can repair the
// operation without discarding it or mistaking an artifact subtype for a tool.
func TestCoachProduceEnvelopeDiagnostics(t *testing.T) {
	for _, tc := range []struct{ produce, diagnostic string }{
		{`{"kind":"draft","payload":{}}`, "payload.kind"},
		{`{"kind":"artifact"}`, "produce.payload缺失"},
		{`{"kind":"artifact","payload":null}`, "produce.payload缺失"},
		{`{"kind":"artifact","payload":[]}`, "必须是JSON对象"},
		{`{"kind":"artifact","payload":"{}"}`, "必须是JSON对象"},
	} {
		_, err := parseCoachOutput(`{"reply":"请检查成果","produce":` + tc.produce + `}`)
		if err == nil || !strings.Contains(err.Error(), tc.diagnostic) {
			t.Fatalf("produce %s: expected %s, got %v", tc.produce, tc.diagnostic, err)
		}
	}
	for _, item := range ProduceKinds {
		out, err := parseCoachOutput(`{"reply":"请检查成果","produce":{"kind":"` + item.Kind + `","payload":{}}}`)
		if err != nil || out.Produce == nil || out.Produce.Kind != item.Kind {
			t.Fatalf("registered operation %s rejected: %v", item.Kind, err)
		}
	}
}

// Inside a session the prompt must say so, and must tell 印记 not to hang
// another hook — nesting a dig inside a dig inside a dig is how a student
// loses the thread.
func TestBuildCoachContext_InsideASession(t *testing.T) {
	ctx := buildCoachContext(CoachInput{
		Idea: "剩饭", SessionKind: "free", SessionQuestion: "剩的是米饭还是菜？",
	})
	if !strings.Contains(ctx, "专题讨论") {
		t.Fatalf("session context does not say it is a side thread:\n%s", ctx)
	}
	if !strings.Contains(ctx, "不提供 hook") {
		t.Fatalf("session context does not suppress further hooks:\n%s", ctx)
	}
}

// 🚨 The main thread sees the CONCLUSIONS of closed sessions, never their turns.
func TestBuildCoachContext_CarriesWriteBacksNotWorking(t *testing.T) {
	ctx := buildCoachContext(CoachInput{
		Idea:       "剩饭",
		WriteBacks: []string{"剩的主要是米饭，不是菜"},
		Recent:     []Turn{{Role: "student", Content: "我去看了三天"}},
	})
	if !strings.Contains(ctx, "剩的主要是米饭") {
		t.Fatalf("write-back missing from the main-thread context:\n%s", ctx)
	}
}

// The window is the only bound on prompt growth in lite — there is no
// compaction layer behind it.
func TestBuildCoachContext_WindowsTheThread(t *testing.T) {
	var turns []Turn
	for i := range 40 {
		role := "student"
		if i%2 == 1 {
			role = "ai"
		}
		turns = append(turns, Turn{Role: role, Content: strings.Repeat("x", 8) + string(rune('a'+i%26))})
	}
	ctx := buildCoachContext(CoachInput{Idea: "剩饭", Recent: turns})
	// The very first turn must have fallen out of the window.
	if strings.Contains(ctx, turns[0].Content) {
		t.Fatalf("the oldest turn survived a %d-turn window", recentWindow)
	}
	if !strings.Contains(ctx, turns[len(turns)-1].Content) {
		t.Fatal("the newest turn was dropped")
	}
}

// A project with no approved plan says so, rather than pretending to have one.
func TestBuildCoachContext_NoPlanYet(t *testing.T) {
	ctx := buildCoachContext(CoachInput{Idea: "剩饭"})
	if !strings.Contains(ctx, "尚未生成计划") {
		t.Fatalf("context does not say there is no plan yet:\n%s", ctx)
	}
}

func TestCoachRejectsLeakedEnvelopeInsteadOfPretendingToSave(t *testing.T) {
	bad := `{"reply":"这两条我写进了计划里。\"mission\":[]}"}`
	if _, err := parseCoachOutput(bad); err == nil {
		t.Fatal("leaked control tail accepted")
	}
	if _, err := parseCoachOutput(`{"reply":"第一段"}{"reply":"第二段"}`); err == nil {
		t.Fatal("second JSON envelope silently ignored")
	}
	// Naming a field in an explanation is not the misplaced envelope suffix.
	if _, err := parseCoachOutput(`{"reply":"字段名是 mission，它表示观察清单。"}`); err != nil {
		t.Fatal(err)
	}
	script := func(raw string) []gateway.StreamEvent {
		return []gateway.StreamEvent{{Kind: gateway.EventTextDelta, TextDelta: raw}, {Kind: gateway.EventDone, StopReason: gateway.StopStop}}
	}
	provider := gateway.NewSequenceStubProvider(script(bad), script(`{"reply":"请检查修订计划。","produce":{"kind":"plan","payload":{"steps":[{"title":"记录实际样本数","decide":"样本能支持哪些结论"}]}}}`))
	out, _, err := Coach(context.Background(), provider, gateway.Resolved{}, CoachInput{Idea: "社区指引"})
	if err != nil || out.Produce == nil || out.Produce.Kind != "plan" || provider.Calls != 2 {
		t.Fatalf("repair did not produce plan: %+v %v calls=%d", out, err, provider.Calls)
	}
}

func TestIterationOpeningCannotMutateProjectArtifacts(t *testing.T) {
	script := func(raw string) []gateway.StreamEvent {
		return []gateway.StreamEvent{{Kind: gateway.EventTextDelta, TextDelta: raw}, {Kind: gateway.EventDone, StopReason: gateway.StopStop}}
	}
	provider := gateway.NewSequenceStubProvider(script(`{"reply":"已重写计划","produce":{"kind":"plan","payload":{"steps":[]}}}`), script(`{"reply":"这条记录尚未测试。你准备怎样判断筛选问题能否被理解？"}`))
	out, _, err := Coach(context.Background(), provider, gateway.Resolved{}, CoachInput{SessionKind: "keeping", ToolWork: []string{"所选记录：尚未试问，没有参与者"}})
	if err != nil || out.Produce != nil || provider.Calls != 2 {
		t.Fatalf("unexpected opening: %+v %v calls=%d", out, err, provider.Calls)
	}
	if strings.Contains(out.Reply, "已重写") {
		t.Fatal("rejected mutation claim escaped")
	}
}

func TestCoachMisnestedArtifactReportsExactRepairLevel(t *testing.T) {
	for _, tc := range []struct{ raw, want string }{
		{`{"reply":"已修改","title":"卡片","body":"内容"}`, "顶层字段title位置错误"},
		{`{"reply":"已修改","produce":{"kind":"artifact","title":"卡片","payload":{}}}`, "produce.title位置错误"},
	} {
		_, err := parseCoachOutput(tc.raw)
		if err == nil || !strings.Contains(err.Error(), tc.want) || !strings.Contains(err.Error(), "produce.payload") {
			t.Fatalf("wrong diagnostic: %v", err)
		}
	}
	if _, err := parseCoachOutput(`{"reply":"已修改","produce":{"kind":"artifact","payload":{"kind":"draft","title":"卡片","body":"内容"}}}`); err != nil {
		t.Fatal(err)
	}
}

func TestCoachRetriesLengthStopEvenWithValidJSON(t *testing.T) {
	script := func(body string, stop gateway.StopReason) []gateway.StreamEvent {
		return []gateway.StreamEvent{
			{Kind: gateway.EventTextDelta, TextDelta: body},
			{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 10, OutputTokens: 20}},
			{Kind: gateway.EventDone, StopReason: stop},
		}
	}
	provider := gateway.NewSequenceStubProvider(
		script(`{"reply":"不应该替你把"}`, gateway.StopLength),
		script(`{"reply":"请记录实际结果，找到、未找到或不确定都可以。"}`, gateway.StopStop),
	)
	out, usage, err := Coach(context.Background(), provider, gateway.Resolved{}, CoachInput{Idea: "图书角"})
	if err != nil || out.Reply != "请记录实际结果，找到、未找到或不确定都可以。" {
		t.Fatalf("truncated result escaped: %+v %v", out, err)
	}
	if provider.Calls != 2 || usage.InputTokens != 20 || usage.OutputTokens != 40 {
		t.Fatalf("calls or usage lost: %d %+v", provider.Calls, usage)
	}
	if !strings.Contains(provider.Requests[1].Messages[len(provider.Requests[1].Messages)-1].Content, "输出长度限制") {
		t.Fatal("repair missing truncation reason")
	}
}

func TestCurrentRequestRemainsDistinctFromHistoricalToolWork(t *testing.T) {
	in := CoachInput{Recent: []Turn{{Role: "student", Content: "请修改实际清单"}}, ToolWork: []string{"历史问题：没有候选书怎么办"}}
	ctx := buildCoachContext(in)
	current := strings.LastIndex(ctx, "请修改实际清单")
	history := strings.LastIndex(ctx, "历史问题：没有候选书怎么办")
	if current <= history {
		t.Fatal("current request is obscured by historical tool work")
	}
	in.JustHappened = "本轮结束观察任务"
	ctx = buildCoachContext(in)
	if strings.Contains(ctx, "【本轮学生请求】") || !strings.Contains(ctx, "本轮结束观察任务") {
		t.Fatal("tool completion reactivated a historical student request")
	}
}
