package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"mindimprint/api/internal/coachwalk"
	"mindimprint/api/internal/gateway"
)

type walkJudgeProvider struct {
	replies  []string
	requests []gateway.ChatRequest
	calls    int
}

type artifactHashDriver struct{ prompt string }

func (d *artifactHashDriver) Site() string { return "hash" }
func (d *artifactHashDriver) Request() gateway.ChatRequest {
	return gateway.ChatRequest{Messages: []gateway.ChatMessage{{Role: gateway.RoleSystem, Content: d.prompt}}}
}
func (d *artifactHashDriver) Parse(string) (string, []coachwalk.Violation, error) {
	return "", nil, nil
}
func (d *artifactHashDriver) Advance(string, string, string) {}
func (d *artifactHashDriver) HerWords() string               { return "" }
func (d *artifactHashDriver) Persona() string                { return "" }
func (d *artifactHashDriver) Screen(string) string           { return "" }

func (p *walkJudgeProvider) Stream(_ context.Context, _ gateway.Resolved, req gateway.ChatRequest) (<-chan gateway.StreamEvent, error) {
	ch := make(chan gateway.StreamEvent, 1)
	p.requests = append(p.requests, req)
	ch <- gateway.StreamEvent{Kind: gateway.EventTextDelta, TextDelta: p.replies[p.calls]}
	p.calls++
	close(ch)
	return ch, nil
}

func TestWalkTechnicalFailuresOnlyIncludesRunAndParse(t *testing.T) {
	a := walkArtifact{Rows: []walkArtifactRow{{ID: "s", Violations: map[string]int{"advanced-too-early": 1}}}}
	if got := walkTechnicalFailures(a); len(got) != 0 {
		t.Fatalf("quality observation must not block: %v", got)
	}
	a.Rows[0].Violations["parse"] = 1
	if got := walkTechnicalFailures(a); len(got) != 1 {
		t.Fatalf("parse failure must block: %v", got)
	}
}

func TestWalkArtifactPreservesAttemptsAndRunFailure(t *testing.T) {
	rows := []row{{id: "s", version: 2, model: "m", rep: 1, expected: 2, err: errors.New("provider down"), log: &coachwalk.Log{Turns: []coachwalk.Turn{{
		N: 1, Reply: "first", SelectedAttempt: 1, RecoveryReason: "soft", InTokens: 14, OutTokens: 4, ReasoningTokens: 2,
		Attempts: []coachwalk.Attempt{{Raw: "first", InTokens: 7, OutTokens: 2, ReasoningTokens: 1}, {Raw: "second", InTokens: 7, OutTokens: 2, ReasoningTokens: 1}},
	}}}}}
	a := makeWalkArtifact(rows, "x", "judge", 2, 1, map[string]string{"s": "hash"})
	got := a.Rows[0]
	if got.Completed || got.Error == "" || len(got.Turns) != 1 || len(got.Turns[0].Attempts) != 2 || got.Turns[0].RecoveryReason != "soft" || got.P50InputTokens != 14 || got.P50ReasoningTokens != 2 || got.Turns[0].Attempts[0].ReasoningTokens != 1 || a.ScenarioHashes["s"] != "hash" {
		t.Fatalf("artifact lost recovery evidence: %+v", got)
	}
}

func TestScenarioHashTracksPromptAndScript(t *testing.T) {
	makeCoach := func(prompt string, script []string) coach {
		return coach{id: "s", script: script, make: func() coachwalk.Driver { return &artifactHashDriver{prompt: prompt} }}
	}
	first, err := hashCoaches([]coach{makeCoach("prompt-a", []string{"reply-a"})})
	if err != nil {
		t.Fatal(err)
	}
	promptChanged, _ := hashCoaches([]coach{makeCoach("prompt-b", []string{"reply-a"})})
	scriptChanged, _ := hashCoaches([]coach{makeCoach("prompt-a", []string{"reply-b"})})
	if first["s"] == promptChanged["s"] || first["s"] == scriptChanged["s"] {
		t.Fatal("scenario hash must track both the production request and scripted path")
	}
}

func TestWalkComparisonShowsDeltasAndIncomparableScenario(t *testing.T) {
	before := walkArtifact{Suite: "x", JudgeModel: "judge", Rows: []walkArtifactRow{{ID: "s", Model: "m", Run: 1, Version: 1, P50InputTokens: 100, Completed: true}}}
	after := walkArtifact{Suite: "x", JudgeModel: "judge", Rows: []walkArtifactRow{{ID: "s", Model: "m", Run: 1, Version: 1, P50InputTokens: 80, Completed: false}}}
	if got := compareWalkArtifacts(before, after); !strings.Contains(got, "100 → 80") || !strings.Contains(got, "true → false") {
		t.Fatalf("comparison = %s", got)
	}
	after.Rows[0].Version = 2
	if got := compareWalkArtifacts(before, after); !strings.Contains(got, "不可比") {
		t.Fatalf("version change = %s", got)
	}
}

func TestJudgeWalkRetriesMalformedOrIncompleteVerdict(t *testing.T) {
	p := &walkJudgeProvider{replies: []string{
		`{"score":2,"why":"坏掉的"引号""}`,
		`{"score":4,"why":"完整理由"}`,
	}}
	score, why := judgeWalk(context.Background(), p, gateway.Resolved{}, "rubric", "transcript")
	if score != 4 || why != "完整理由" || p.calls != 2 {
		t.Fatalf("judge retry = score %v, why %q, calls %d", score, why, p.calls)
	}
	for i, req := range p.requests {
		if len(req.Messages) != 2 || req.Messages[0].Role != gateway.RoleSystem ||
			!strings.Contains(req.Messages[0].Content, `"score"`) || !strings.Contains(req.Messages[0].Content, `"why"`) {
			t.Fatalf("attempt %d did not carry the strict judge schema: %+v", i+1, req.Messages)
		}
	}
}

func TestJudgeWalkDoesNotAcceptScoreWithoutWhy(t *testing.T) {
	p := &walkJudgeProvider{replies: []string{`{"score":2}`, `{"score":4,"why":""}`}}
	score, why := judgeWalk(context.Background(), p, gateway.Resolved{}, "rubric", "transcript")
	if score != 0 || !strings.Contains(why, "重试 2 次后仍失败") || p.calls != 2 {
		t.Fatalf("judge failure = score %v, why %q, calls %d", score, why, p.calls)
	}
}
