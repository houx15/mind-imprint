package routebench

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mindimprint/api/internal/benchcase"
	"mindimprint/api/internal/gateway"
)

func TestSuiteMarkdownShowsGateWithoutRoutingRecommendation(t *testing.T) {
	cat, err := gateway.DefaultCatalog()
	if err != nil {
		t.Fatal(err)
	}
	got := Markdown(nil, cat, Config{Suite: "lite-reading-coach", Samples: 2, JudgeModel: "judge"}, time.Now())
	if !strings.Contains(got, "prompt gate · 单轮体检") || !strings.Contains(got, "回复速览") {
		t.Fatalf("suite report lost gate identity:\n%s", got)
	}
	if strings.Contains(got, "推荐绑定") {
		t.Fatalf("suite report mixed prompt quality with routing recommendation:\n%s", got)
	}
}

func TestArtifactHashTracksTheWholeProductionRequest(t *testing.T) {
	base := benchcase.Case{Suite: "x", ID: "c", Request: gateway.ChatRequest{
		Messages:  []gateway.ChatMessage{{Role: gateway.RoleSystem, Content: "prompt"}},
		MaxTokens: 100,
	}}
	cfg := Config{Suite: "x"}
	first := NewArtifact([]benchcase.Case{base}, cfg, nil, time.Now())
	changed := base
	changed.Request.MaxTokens = 200
	second := NewArtifact([]benchcase.Case{changed}, cfg, nil, time.Now())
	if first.CaseHashes["c"] == second.CaseHashes["c"] {
		t.Fatal("case hash must include request controls as well as message text")
	}
}

type retryProvider struct {
	replies []string
	calls   int
}

func (p *retryProvider) Stream(_ context.Context, _ gateway.Resolved, _ gateway.ChatRequest) (<-chan gateway.StreamEvent, error) {
	ch := make(chan gateway.StreamEvent, 2)
	text := p.replies[p.calls]
	p.calls++
	ch <- gateway.StreamEvent{Kind: gateway.EventTextDelta, TextDelta: text}
	ch <- gateway.StreamEvent{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 7, OutputTokens: 3}}
	close(ch)
	return ch, nil
}

func TestRunnerRetriesUnparseableProductionReplyAndAccumulatesUsage(t *testing.T) {
	p := &retryProvider{replies: []string{"broken", "ok"}}
	rn := &Runner{Provider: p, Cfg: Config{TimeoutSeconds: 1}}
	c := benchcase.Case{Request: gateway.ChatRequest{}, Parse: func(s string) error {
		if s != "ok" {
			return fmt.Errorf("unparseable")
		}
		return nil
	}, Validate: func(string) error { return nil }}
	s := rn.once(context.Background(), gateway.Resolved{}, c)
	if !s.Parsed || !s.Valid || s.Retries != 1 || s.In != 14 || s.Out != 6 || p.calls != 2 || s.SelectedAttempt != 2 || s.FirstText != "broken" || s.RetryText != "ok" {
		t.Fatalf("recovery sample = %+v, calls = %d", s, p.calls)
	}
}

func TestRunnerDoesNotRetryFixtureExpectationFailure(t *testing.T) {
	p := &retryProvider{replies: []string{"usable"}}
	rn := &Runner{Provider: p, Cfg: Config{TimeoutSeconds: 1}}
	c := benchcase.Case{Request: gateway.ChatRequest{}, Parse: func(string) error { return nil }, Validate: func(string) error {
		return fmt.Errorf("advance mismatch")
	}}
	s := rn.once(context.Background(), gateway.Resolved{}, c)
	if s.Valid || s.Retries != 0 || p.calls != 1 || s.ValidErr != "advance mismatch" {
		t.Fatalf("expectation failure = %+v, calls = %d", s, p.calls)
	}
}

func TestSuiteRunnerSamplesTwiceWithFakeProvider(t *testing.T) {
	cat, err := gateway.DefaultCatalog()
	if err != nil {
		t.Fatal(err)
	}
	p := &retryProvider{replies: []string{"one", "two"}}
	rn := &Runner{Cat: cat, Provider: p, Keys: func(string) string { return "test-key" },
		Cfg: Config{Suite: "x", Samples: 2, Candidates: map[string][]string{gateway.ClassDialogue: {"dashscope/deepseek-v4-pro"}}}}
	results := rn.Run(context.Background(), []benchcase.Case{{Suite: "x", ID: "case", Class: gateway.ClassDialogue,
		Parse: func(string) error { return nil }, Validate: func(string) error { return nil }}})
	if len(results) != 1 || len(results[0].Samples) != 2 || p.calls != 2 {
		t.Fatalf("samples=%+v, calls=%d", results, p.calls)
	}
}

func TestSuiteReportShowsRepresentativeAndAbnormalReplies(t *testing.T) {
	cat, err := gateway.DefaultCatalog()
	if err != nil {
		t.Fatal(err)
	}
	results := []Result{{CaseID: "case", Class: gateway.ClassDialogue, ModelID: "m", Samples: []Sample{
		{Text: `{"reply":"first"}`, Parsed: true, Valid: true},
		{Text: `{"reply":"second"}`, Parsed: true, ValidErr: "wrong advance"},
	}}}
	md := Markdown(results, cat, Config{Suite: "x", Samples: 2}, time.Now())
	for _, want := range []string{"回复速览", `{"reply":"first"}`, `{"reply":"second"}`, "wrong advance"} {
		if !strings.Contains(md, want) {
			t.Fatalf("report lacks %q", want)
		}
	}
}

func TestJudgeRetriesInvalidJSONInsteadOfSalvagingScore(t *testing.T) {
	p := &retryProvider{replies: []string{
		`{"score":2,"why":"用了未转义的"引号""}`,
		`{"score":4,"why":"第二次返回了完整理由"}`,
	}}
	rn := &Runner{Provider: p}
	score, why := rn.judgeOne(context.Background(), gateway.Resolved{}, "rubric", "output")
	if score != 4 || why != "第二次返回了完整理由" || p.calls != 2 {
		t.Fatalf("judge retry = score %v, why %q, calls %d", score, why, p.calls)
	}
}

func TestJudgeRetriesEmptyWhyAndFailsExplicitly(t *testing.T) {
	p := &retryProvider{replies: []string{
		`{"score":4,"why":""}`,
		`{"score":3}`,
	}}
	rn := &Runner{Provider: p}
	score, why := rn.judgeOne(context.Background(), gateway.Resolved{}, "rubric", "output")
	if score != 0 || !isJudgeFailure(why) || p.calls != 2 {
		t.Fatalf("judge failure = score %v, why %q, calls %d", score, why, p.calls)
	}
}

func TestTechnicalFailuresOnlyIncludesCallsAndFinalParse(t *testing.T) {
	a := Artifact{Results: []Result{{CaseID: "c", Samples: []Sample{
		{Parsed: true, Valid: false, ValidErr: "wrong advance", Judge: 1},
		{Parsed: false, ParseErr: "bad JSON"},
	}}}}
	got := TechnicalFailures(a)
	if len(got) != 1 || !strings.Contains(got[0], "bad JSON") {
		t.Fatalf("technical failures = %v", got)
	}
}

func TestComparisonIsAdvisoryAndMarksIncomparableCases(t *testing.T) {
	base := Artifact{Suite: "x", JudgeModel: "judge", Samples: 2,
		Results: []Result{{CaseID: "c", ModelID: "m", Version: 1, Samples: []Sample{{In: 100, Out: 20, Parsed: true, Valid: true}}}}}
	now := base
	now.Results = []Result{{CaseID: "c", ModelID: "m", Version: 1, Samples: []Sample{{In: 70, Out: 15, Parsed: true, Valid: false, ValidErr: "wrong advance"}}}}
	got := CompareArtifactMarkdown(base, now)
	if !strings.Contains(got, "100 → 70") || !strings.Contains(got, "供人工审阅") {
		t.Fatalf("comparison = %s", got)
	}
	now.Results[0].Version = 2
	if got := CompareArtifactMarkdown(base, now); !strings.Contains(got, "不可比") {
		t.Fatalf("fixture version change should be marked incomparable: %s", got)
	}
}

func TestResultSeparatesParserAndExpectationRates(t *testing.T) {
	r := Result{Samples: []Sample{
		{Parsed: true, Valid: true},
		{Parsed: true, Valid: false, ValidErr: "advance mismatch"},
	}}
	if got := r.ParseRate(); got != 1 {
		t.Fatalf("parse rate = %v, want 1", got)
	}
	if got := r.ExpectedRate(); got != 0.5 {
		t.Fatalf("expectation rate = %v, want 0.5", got)
	}
}

func TestReadArtifactRejectsPreSplitSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "routebench-v2.json")
	if err := os.WriteFile(path, []byte(`{"schema":2,"suite":"lite-reading-coach"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadArtifact(path); err == nil {
		t.Fatal("schema 2 conflates parser and expectation validity and must not be accepted")
	}
}
