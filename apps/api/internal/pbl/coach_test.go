package pbl

import (
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
	if strings.Contains(assigned, "他一开始是这么说的") {
		t.Errorf("assigned context introduces the teacher's question as her words:\n%s", assigned)
	}

	own := buildCoachContext(CoachInput{Idea: "下课没人去操场"})
	if !strings.Contains(own, "他一开始是这么说的：下课没人去操场") {
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
	if strings.Contains(assigned, "他一开始是这么说的") {
		t.Errorf("assigned lookback introduces the teacher's question as her words:\n%s", assigned)
	}
	// No brief: no 补充说明 line at all, not an empty one.
	if noBrief := buildLookbackContext(LookbackInput{Idea: "怎样让校园少用一次性杯子？", Assigned: true}); strings.Contains(noBrief, "老师补充说明") {
		t.Errorf("empty brief still rendered:\n%s", noBrief)
	}

	own := buildLookbackContext(LookbackInput{Name: "操场", Idea: "下课没人去操场"})
	if !strings.Contains(own, "他一开始是这么说的：下课没人去操场") {
		t.Errorf("her own project lost its opening line:\n%s", own)
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

// Inside a session the prompt must say so, and must tell 印记 not to hang
// another hook — nesting a dig inside a dig inside a dig is how a student
// loses the thread.
func TestBuildCoachContext_InsideASession(t *testing.T) {
	ctx := buildCoachContext(CoachInput{
		Idea: "剩饭", SessionKind: "free", SessionQuestion: "剩的是米饭还是菜？",
	})
	if !strings.Contains(ctx, "支线") {
		t.Fatalf("session context does not say it is a side thread:\n%s", ctx)
	}
	if !strings.Contains(ctx, "不要再给钩子") {
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
	if !strings.Contains(ctx, "还没有计划") {
		t.Fatalf("context does not say there is no plan yet:\n%s", ctx)
	}
}
