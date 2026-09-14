package evalbench

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mindimprint/api/internal/evalreport"
	"mindimprint/api/internal/gateway"
)

func TestPersonaAdapterExcludesPreviousAssessment(t *testing.T) {
	base := `{"project":{"title":"T","qualification":"IB"},"chat":[{"at":"2026-08-01T00:00:00Z","surface":"project","role":"assistant","content":"ask"},{"at":"2026-08-01T00:01:00Z","surface":"project","role":"user","content":"answer"}],"actions":[{"at":"2026-08-01T00:02:00Z","surface":"project","type":"coach_turn","payload":{}}],"outputs":{"snapshots":[{"seq":2,"doc_kind":"essay","at":"2026-08-01T00:00:00Z","content":"two words"},{"seq":1,"doc_kind":"essay","at":"2026-08-01T00:00:00Z","content":"one"}],"assessment":{"secret":"leak"}}}`
	changed := `{"project":{"title":"T","qualification":"IB"},"chat":[{"at":"2026-08-01T00:00:00Z","surface":"project","role":"assistant","content":"ask"},{"at":"2026-08-01T00:01:00Z","surface":"project","role":"user","content":"answer"}],"actions":[{"at":"2026-08-01T00:02:00Z","surface":"project","type":"coach_turn","payload":{}}],"outputs":{"snapshots":[{"seq":2,"doc_kind":"essay","at":"2026-08-01T00:00:00Z","content":"two words"},{"seq":1,"doc_kind":"essay","at":"2026-08-01T00:00:00Z","content":"one"}],"assessment":{"secret":"different"},"mirror":{"x":1},"summary":"bad"}}`
	a := mustAdapt(t, base)
	b := mustAdapt(t, changed)
	if a.Hash != b.Hash {
		t.Fatalf("forbidden output changed input hash: %s != %s", a.Hash, b.Hash)
	}
	if !strings.Contains(a.Input.Context.Prompts, "answer") || a.Input.Basics.Counters.WordsWritten != 2 {
		t.Fatalf("unexpected adapted input: %#v", a.Input)
	}
}

func TestPersonaAdapterBuildsStableAnonymousMaterialIDs(t *testing.T) {
	raw := `{"project":{"id":"private-id","title":"T","qualification":"IB"},"materials":[{"id":"private-material","title":"Source","at":"2026-08-01T00:00:00Z","blocks":[{"text":"finding"}]}]}`
	path := filepath.Join(t.TempDir(), "persona.json")
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := LoadPersona(path)
	if err != nil {
		t.Fatal(err)
	}
	a, err := AdaptPersona("case-a", p)
	if err != nil {
		t.Fatal(err)
	}
	if a.Input.ProjectID != "evalbench:project:case-a" || len(a.Input.Materials) != 1 || a.Input.Materials[0].MaterialID != "material:export:001" {
		t.Fatalf("unexpected anonymous material adaptation: %#v", a.Input)
	}
}

func mustAdapt(t *testing.T, raw string) AdaptedInput {
	t.Helper()
	path := filepath.Join(t.TempDir(), "persona.json")
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := LoadPersona(path)
	if err != nil {
		t.Fatal(err)
	}
	a, err := AdaptPersona("case", p)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func TestObservedProviderRecordsUsageAndCompletion(t *testing.T) {
	in, out, reasoning := 12, 14, 8
	rec := &CallRecorder{}
	observed := &ObservedProvider{Inner: gateway.NewStubProvider([]gateway.StreamEvent{{Kind: gateway.EventTextDelta, TextDelta: "hi"}, {Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: in, OutputTokens: out, ReasoningTokens: &reasoning}}, {Kind: gateway.EventDone, StopReason: gateway.StopStop}}), Recorder: rec, Purpose: "candidate-assessment"}
	stream, err := observed.Stream(context.Background(), gateway.Resolved{Provider: "deepseek", Model: "test", Tier: FlagshipTier, APIKey: "must-not-record"}, gateway.ChatRequest{})
	if err != nil {
		t.Fatal(err)
	}
	for range stream {
	}
	calls := rec.Calls()
	if len(calls) != 1 {
		t.Fatalf("calls=%d", len(calls))
	}
	c := calls[0]
	if c.FirstOutputMs == nil || c.InputTokens == nil || *c.InputTokens != 12 || c.ReasoningTokens == nil || *c.ReasoningTokens != 8 || c.ContentTokens == nil || *c.ContentTokens != 6 || c.Incomplete {
		t.Fatalf("bad observation: %#v", c)
	}
	if c.Request.Messages != nil {
		t.Fatal("request unexpectedly changed")
	}
}

type waitingProvider struct{}

func (waitingProvider) Stream(ctx context.Context, _ gateway.Resolved, _ gateway.ChatRequest) (<-chan gateway.StreamEvent, error) {
	out := make(chan gateway.StreamEvent)
	go func() {
		defer close(out)
		<-ctx.Done()
	}()
	return out, nil
}

func TestObservedProviderReportsCallTimeout(t *testing.T) {
	rec := &CallRecorder{}
	observed := &ObservedProvider{Inner: waitingProvider{}, Recorder: rec, Purpose: "comparator", CallTimeout: 5 * time.Millisecond}
	stream, err := observed.Stream(context.Background(), gateway.Resolved{Provider: "deepseek", Model: "test"}, gateway.ChatRequest{})
	if err != nil {
		t.Fatal(err)
	}
	for range stream {
	}
	calls := rec.Calls()
	if len(calls) != 1 || !calls[0].Incomplete || calls[0].Error != context.DeadlineExceeded.Error() {
		t.Fatalf("timeout call record = %#v", calls)
	}
}

func TestCollectEvalbenchRejectsIncompleteDone(t *testing.T) {
	p := gateway.NewStubProvider([]gateway.StreamEvent{{Kind: gateway.EventTextDelta, TextDelta: "partial"}, {Kind: gateway.EventDone, Incomplete: true}})
	if _, err := collectEvalbench(context.Background(), p, gateway.Resolved{}, gateway.ChatRequest{}); err == nil {
		t.Fatal("incomplete stream unexpectedly accepted")
	}
}

func TestSuccessfulRunsRequireComparison(t *testing.T) {
	complete := attemptResult{report: &evalreport.Report{}, comparison: &Comparison{}, complete: true}
	candidateOnly := attemptResult{report: &evalreport.Report{}, complete: true}
	incomplete := attemptResult{report: &evalreport.Report{}, comparison: &Comparison{}, complete: true, candidate: []CallRecord{{Incomplete: true}}}
	state := &variantState{config: VariantConfig{ID: "v"}, attempts: []attemptResult{complete, candidateOnly, incomplete}}
	if got := successes(state); got != 1 {
		t.Fatalf("successful runs = %d, want 1", got)
	}
	summary := buildSummary("exp-1", map[string][]*variantState{"case": {state}}, Config{SuccessfulRuns: 1, Variants: []VariantConfig{{ID: "v"}}})
	if summary.ExperimentID != "exp-1" || summary.Variants[0].SuccessfulRuns != 1 || summary.Variants[0].Status != "completed" {
		t.Fatalf("unexpected summary: %#v", summary)
	}
}

func TestAddCallsKeepsInputAndOutputTotalsSeparate(t *testing.T) {
	in1, out1, reasoning1, content1 := 12, 14, 8, 6
	in2, out2, reasoning2, content2 := 8, 16, 10, 6
	pricing := buildPricingSnapshot(map[string]ModelProfile{"flagship": {Provider: "deepseek", Model: "deepseek-v4-pro"}})
	totals := addCalls(CallTotals{}, []CallRecord{{Provider: "deepseek", Model: "deepseek-v4-pro", InputTokens: &in1, OutputTokens: &out1, ReasoningTokens: &reasoning1, ContentTokens: &content1}, {Provider: "deepseek", Model: "deepseek-v4-pro", InputTokens: &in2, OutputTokens: &out2, ReasoningTokens: &reasoning2, ContentTokens: &content2}}, pricing)
	if totals.InputTokens == nil || totals.OutputTokens == nil || *totals.InputTokens != 20 || *totals.OutputTokens != 30 || totals.ReasoningTokens == nil || *totals.ReasoningTokens != 18 || totals.ContentTokens == nil || *totals.ContentTokens != 12 {
		t.Fatalf("unexpected token totals: %#v", totals)
	}
	// Derive the expectation from the catalog instead of pinning a literal: the
	// RATE is data that vendors change, and a repriced row should land as a
	// price change, not as a red test. What is worth holding is that the two
	// calls are summed at the catalog rate — so compare with a tolerance, since
	// summing floats call-by-call is not bit-identical to one multiplication.
	wantCost, _ := gateway.EstimateCost("deepseek", "deepseek-v4-pro", 20, 30)
	if totals.CostUSD == nil || totals.CostedCalls != 2 || totals.UnpricedCalls != 0 || totals.UsageMissing != 0 || !nearlyEqualUSD(*totals.CostUSD, wantCost) {
		t.Fatalf("unexpected cost totals: %#v (want %v)", totals, wantCost)
	}
}

func TestAddCallsDoesNotInventCostForMissingOrUnpricedUsage(t *testing.T) {
	in, out := 10, 20
	pricing := buildPricingSnapshot(map[string]ModelProfile{"flagship": {Provider: "deepseek", Model: "deepseek-v4-pro"}})
	totals := addCalls(CallTotals{}, []CallRecord{
		{Provider: "deepseek", Model: "deepseek-v4-pro", InputTokens: &in, OutputTokens: &out},
		{Provider: "deepseek", Model: "unknown", InputTokens: &in, OutputTokens: &out},
		{Provider: "deepseek", Model: "deepseek-v4-pro"},
	}, pricing)
	if totals.CostUSD != nil || totals.CostedCalls != 1 || totals.UnpricedCalls != 1 || totals.UsageMissing != 1 || totals.CostedCalls+totals.UnpricedCalls+totals.UsageMissing != totals.Calls {
		t.Fatalf("unexpected partial cost coverage: %#v", totals)
	}
	zero := addCalls(CallTotals{}, nil, pricing)
	if zero.CostUSD == nil || *zero.CostUSD != 0 {
		t.Fatalf("zero calls must have known zero cost: %#v", zero)
	}
}

func TestBuildPricingSnapshotKeepsPricedAndUnpricedModelsDistinct(t *testing.T) {
	snapshot := buildPricingSnapshot(map[string]ModelProfile{
		"known":     {Provider: "deepseek", Model: "deepseek-v4-pro"},
		"duplicate": {Provider: "deepseek", Model: "deepseek-v4-pro"},
		"unknown":   {Provider: "anthropic", Model: "future-model"},
	})
	if len(snapshot) != 2 || !snapshot["deepseek/deepseek-v4-pro"].Priced || snapshot["anthropic/future-model"].Priced || snapshot["anthropic/future-model"].Currency != "USD" {
		t.Fatalf("unexpected pricing snapshot: %#v", snapshot)
	}
}

func TestGLMProfileResolvesWithZAIKey(t *testing.T) {
	rt := &Runtime{profiles: map[string]ModelProfile{"glm": {Provider: "glm", Model: "glm-5.3-flash", ReasoningEffort: "low"}}}
	t.Setenv("ZAI_API_KEY", "zai-key")
	resolved, profile, err := rt.Resolve("glm")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if profile.Provider != "glm" || resolved.Provider != "glm" || resolved.BaseURL != "https://open.bigmodel.cn/api/paas/v4" || resolved.Model != "glm-5.3-flash" || resolved.APIKey != "zai-key" || resolved.Tier != FlagshipTier || resolved.DefaultReasoningEffort != "low" {
		t.Fatalf("resolved = %#v, profile = %#v", resolved, profile)
	}
	t.Setenv("ZAI_API_KEY", "")
	if _, _, err := rt.Resolve("glm"); err == nil || !strings.Contains(err.Error(), "missing API key") {
		t.Fatalf("missing GLM key error = %v", err)
	}
}

func TestValidateModelProfileRestrictsReasoningEffortToGLMValues(t *testing.T) {
	for _, p := range []ModelProfile{
		{Provider: "glm", Model: "glm-5.3-flash", ReasoningEffort: "low"},
		{Provider: "glm", Model: "glm-5.3-flash", ReasoningEffort: "high"},
		{Provider: "glm", Model: "glm-5.3-flash", ReasoningEffort: "max"},
	} {
		if err := ValidateModelProfile(p); err != nil {
			t.Fatalf("valid GLM profile rejected: %v", err)
		}
	}
	if err := ValidateModelProfile(ModelProfile{Provider: "glm", Model: "glm-5.3-flash", ReasoningEffort: "medium"}); err == nil {
		t.Fatal("unsupported GLM effort accepted")
	}
	if err := ValidateModelProfile(ModelProfile{Provider: "deepseek", Model: "deepseek-v4-pro", ReasoningEffort: "low"}); err == nil {
		t.Fatal("non-GLM effort accepted")
	}
}

func TestWriteCallsReturnsArtifactFailure(t *testing.T) {
	root := filepath.Join(t.TempDir(), "blocked")
	if err := os.WriteFile(root, []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeCalls(filepath.Join(root, "calls"), []CallRecord{{}}); err == nil {
		t.Fatal("call artifact failure unexpectedly ignored")
	}
}

func TestRunAttemptReturnsArtifactDirectoryFailure(t *testing.T) {
	root := t.TempDir()
	blocked := filepath.Join(root, "cases", "case", "variant")
	if err := os.MkdirAll(filepath.Dir(blocked), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(blocked, []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := runAttempt(context.Background(), &Runtime{}, Config{}, CaseConfig{ID: "case"}, AdaptedInput{}, evalreport.Report{}, VariantConfig{ID: "variant"}, 1, root)
	if err == nil {
		t.Fatal("attempt directory failure unexpectedly ignored")
	}
}

func TestPersistFinalArtifactsReturnsWriteFailures(t *testing.T) {
	state := map[string][]*variantState{"case": {{config: VariantConfig{ID: "variant"}}}}
	cases := []struct {
		name  string
		block func(root string) error
	}{
		{"manifest", func(root string) error { return os.Mkdir(filepath.Join(root, "manifest.json"), 0o755) }},
		{"stability", func(root string) error {
			if err := os.MkdirAll(filepath.Join(root, "cases", "case", "variant"), 0o755); err != nil {
				return err
			}
			return os.Mkdir(filepath.Join(root, "cases", "case", "variant", "stability.json"), 0o755)
		}},
		{"summary", func(root string) error { return os.Mkdir(filepath.Join(root, "summary.json"), 0o755) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if err := tc.block(root); err != nil {
				t.Fatal(err)
			}
			if err := persistFinalArtifacts(root, Manifest{}, Summary{}, state); err == nil {
				t.Fatal("final artifact failure unexpectedly ignored")
			}
		})
	}
}

func TestSummaryCostIncludesFailedAttemptsAndRetries(t *testing.T) {
	in, out := 1_000, 2_000
	call := func(purpose string) CallRecord {
		return CallRecord{Purpose: purpose, Provider: "deepseek", Model: "deepseek-v4-pro", InputTokens: &in, OutputTokens: &out}
	}
	complete := attemptResult{status: AttemptStatus{Attempt: 1}, report: &evalreport.Report{}, comparison: &Comparison{}, complete: true, candidate: []CallRecord{call("candidate")}, comparator: []CallRecord{call("comparator")}}
	failed := attemptResult{status: AttemptStatus{Attempt: 2}, candidate: []CallRecord{call("candidate"), call("candidate")}, comparator: []CallRecord{call("comparator")}}
	c := Config{SuccessfulRuns: 1, Models: map[string]ModelProfile{"flagship": {Provider: "deepseek", Model: "deepseek-v4-pro"}}, Variants: []VariantConfig{{ID: "v"}}}
	summary := buildSummary("exp-cost", map[string][]*variantState{"case": {{config: VariantConfig{ID: "v"}, attempts: []attemptResult{complete, failed}}}}, c)
	variant := summary.Variants[0]
	if variant.Candidate.Calls != 3 || variant.Comparator.Calls != 2 || variant.CandidateCostPerSuccessUSD == nil || variant.TotalCostPerSuccessUSD == nil {
		t.Fatalf("unexpected cost summary: %#v", variant)
	}
	perCall, _ := gateway.EstimateCost("deepseek", "deepseek-v4-pro", in, out)
	if !nearlyEqualUSD(*variant.CandidateCostPerSuccessUSD, perCall*3) || !nearlyEqualUSD(*variant.TotalCostPerSuccessUSD, perCall*5) {
		t.Fatalf("cost must include failed/retried calls: %#v", variant)
	}
}

// nearlyEqualUSD compares two dollar amounts without demanding bit-identical
// floats. The summary adds one call at a time while the expectation multiplies,
// and those two routes to the same money differ in the last bits — a difference
// that says nothing about whether failed attempts were counted, which is what
// these tests are actually about. A hundredth of a cent is far below any amount
// a report renders.
func nearlyEqualUSD(got, want float64) bool {
	d := got - want
	if d < 0 {
		d = -d
	}
	return d < 1e-9
}

func TestComparisonRequiresFullMatrix(t *testing.T) {
	if err := ValidateComparison(Comparison{}); err == nil {
		t.Fatal("empty matrix unexpectedly valid")
	}
}

func TestDefaultConfigIncludesProductionAndSinglePromptVariants(t *testing.T) {
	c, err := LoadConfig(filepath.Join("..", "..", "tools", "evalbench", "config.json"))
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if profile := c.Models["deepseek-flagship"]; len(c.Models) != 2 || profile.Provider != "deepseek" || profile.Model != "deepseek-v4-pro" {
		t.Fatalf("default models changed: %#v", c.Models)
	}
	if profile := c.Models["glm-5.3-flash"]; profile.Provider != "glm" || profile.Model != "glm-5.3-flash" || profile.ReasoningEffort != "max" {
		t.Fatalf("default GLM profile changed: %#v", c.Models)
	}
	got := map[string]string{}
	for _, v := range c.Variants {
		got[v.ID] = v.Evaluator + ":" + v.Model
	}
	if len(got) != 2 || got["production-current"] != "production-evalreport-v1:deepseek-flagship" || got["single-prompt-v1"] != singlePromptEvaluatorID+":deepseek-flagship" {
		t.Fatalf("variants = %#v", got)
	}
	descriptors := evaluatorDescriptors(c.Variants)
	if descriptors["single-prompt-v1"].ImplementationVersion != singlePromptImplementationV1 || descriptors["single-prompt-v1"].PromptSHA256 == "" || descriptors["production-current"].ImplementationVersion != "evalbench-production-baseline-v1" {
		t.Fatalf("descriptors = %#v", descriptors)
	}
}
