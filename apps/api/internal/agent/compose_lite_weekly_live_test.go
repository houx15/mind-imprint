package agent

// compose_lite_weekly_live_test.go — the two weekly composers against a real
// model. Skipped unless LIVE_LLM=1 and a provider key is set.
//
//	set -a; . .deploy-local/env.prod; set +a
//	LIVE_LLM=1 CGO_ENABLED=0 go test ./internal/agent -run TestLiveLiteWeekly -v -count=1 -timeout 600s
//
// The stub tests prove the validators read JSON that the tests wrote. Only a
// real model shows whether the prompt gets prose past CheckProse on the first
// attempt: quotes copied exactly, 《》 for titles, no digits that are not in
// the facts, no classmate named. The facts include a stalled item, so the
// 《title》 check and the 「超过 7 天」 evidence digits are exercised.
//
// The class used is ClassAssess, the same one the Task 5 handlers route.
// Each attempt is logged (usage, error, raw text) so the report can record
// how many attempts each compose took and whether the first one passed.

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"

	"mindimprint/api/internal/config"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/liteweek"
	"mindimprint/api/internal/liteweekly"
)

func liveLiteWeeklyModel(t *testing.T) (gateway.Provider, gateway.Resolved) {
	t.Helper()
	if os.Getenv("LIVE_LLM") != "1" {
		t.Skip("set LIVE_LLM=1 to run live prompt checks")
	}
	// The same path cmd/api takes: config.Load (every provider key, the legacy
	// MODEL_CHAT / MODEL_FAST_CHAT / MODEL_EVAL aliases), then
	// gateway.NewResolvers, which reads MODEL_ASSESS and the other per-class
	// MODEL_<CLASS> variables itself. DATABASE_URL is required by config.Load
	// but never used here, so a placeholder fills it when the environment has
	// none.
	if os.Getenv("DATABASE_URL") == "" {
		t.Setenv("DATABASE_URL", "postgres://unused-by-live-weekly-test")
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	if cfg.DashScopeKey == "" && cfg.DeepSeekKey == "" && cfg.AnthropicKey == "" && cfg.ZAIKey == "" {
		t.Skip("no provider key in the environment")
	}
	rs, err := gateway.NewResolvers(cfg)
	if err != nil {
		t.Fatalf("resolvers: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	r, err := rs.For(gateway.ClassAssess)(ctx)
	if err != nil {
		t.Fatalf("class %s has no model here: %v", gateway.ClassAssess, err)
	}
	p := gateway.NewMuxProvider(map[string]gateway.Provider{
		gateway.KindOpenAICompatible: gateway.NewCatalogProvider(&http.Client{}),
		gateway.KindAnthropic:        gateway.NewAnthropicProvider(&http.Client{}),
	})
	return p, r
}

// liveLiteWeek is one realistic week, in the shape T3's loaders fill.
func liveLiteWeeks() (label string, lin, wang liteweekly.StudentWeek) {
	ws := time.Date(2026, 9, 7, 0, 0, 0, 0, liteweek.Beijing)
	label = liteweek.Label(ws)
	lin = liteweekly.StudentWeek{
		UserID: "7d7c2a8e-1f00-4c2b-9a51-000000000011", Name: "林小雨",
		ActiveDays: 3, Minutes: 85, Turns: 12, PrevActiveDays: 1,
		Finished:           []liteweekly.Item{{Kind: "reading", Title: "一场雨"}},
		AssignmentsDone:    0,
		AssignmentsLate:    0,
		AssignmentsOverdue: 1,
		Stalled:            []liteweekly.Item{{Kind: "writing", Title: "雨水花园调查"}},
		NewKeywords:        []string{"天气"},
		Moments:            []liteweekly.Moment{{Quote: "雨落在屋檐上像敲鼓", ItemTitle: "一场雨"}},
	}
	wang = liteweekly.StudentWeek{
		UserID: "7d7c2a8e-1f00-4c2b-9a51-000000000012", Name: "王思远",
		ActiveDays: 4, Minutes: 120, Turns: 20, PrevActiveDays: 2,
		Finished:        []liteweekly.Item{{Kind: "writing", Title: "雨停之前"}},
		AssignmentsDone: 1,
		Stalled:         []liteweekly.Item{},
		NewKeywords:     []string{},
		Moments:         []liteweekly.Moment{},
	}
	return label, lin, wang
}

func liveCards(s liteweekly.StudentWeek) []liteweekly.Card {
	watch, praise := liteweekly.Cards(s)
	out := []liteweekly.Card{}
	if watch != nil {
		out = append(out, *watch)
	}
	if praise != nil {
		out = append(out, *praise)
	}
	return out
}

func logAttempts(t *testing.T, attempts []Attempt) {
	t.Helper()
	t.Logf("attempts: %d", len(attempts))
	for i, at := range attempts {
		verdict := "passed checks"
		if at.Err != nil {
			verdict = "failed: " + at.Err.Error()
		}
		t.Logf("attempt %d: %s; usage %+v\n%s", i+1, verdict, at.Usage, at.Text)
	}
}

func TestLiveLiteWeeklyStudent(t *testing.T) {
	prov, resolved := liveLiteWeeklyModel(t)
	label, lin, _ := liveLiteWeeks()
	cards := liveCards(lin)
	t.Logf("model: %s; cards: %+v", resolved.Model, cards)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	prose, attempts, err := ComposeLiteStudentWeekly(ctx, prov, resolved, lin, label, cards, []string{"王思远"})
	logAttempts(t, attempts)
	if err != nil {
		t.Fatalf("compose: %v", err)
	}
	t.Logf("summary: %s", prose.Summary)
	for i, sg := range prose.Suggestions {
		t.Logf("suggestion %d [%s]: %s", i+1, sg.EvidenceCode, sg.Text)
	}
}

func TestLiveLiteWeeklyClass(t *testing.T) {
	prov, resolved := liveLiteWeeklyModel(t)
	label, lin, wang := liveLiteWeeks()
	students := []liteweekly.StudentWeek{lin, wang}
	cards := map[string][]liteweekly.Card{}
	for _, s := range students {
		if cs := liveCards(s); len(cs) > 0 {
			cards[s.UserID] = cs
		}
	}
	// As liteClassWeekStats computes it: done+late over done+late+overdue.
	stats := liteweekly.ClassWeekStats{
		ClassSize: 2, ActiveStudents: 2, Minutes: 205, Turns: 32, Finished: 2, AssignmentRate: 50,
	}
	t.Logf("model: %s; cards: %+v", resolved.Model, cards)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	prose, attempts, err := ComposeLiteClassWeekly(ctx, prov, resolved, "高一（3）班 · 阅读写作", label, stats, students, cards, []string{"林小雨", "王思远"})
	logAttempts(t, attempts)
	if err != nil {
		t.Fatalf("compose: %v", err)
	}
	t.Logf("comment: %s", prose.Comment)
	for _, c := range prose.Cards {
		t.Logf("card %s\n  lead: %s\n  action: %s", c.UserID, c.Lead, c.Action)
	}
}
