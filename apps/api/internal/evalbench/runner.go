package evalbench

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"mindimprint/api/internal/evalreport"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/rubric"
)

type Manifest struct {
	ExperimentID string                         `json:"experimentId"`
	Name         string                         `json:"name"`
	ConfigHash   string                         `json:"configHash"`
	GitCommit    string                         `json:"gitCommit,omitempty"`
	Dirty        bool                           `json:"dirty"`
	RubricHash   string                         `json:"rubricHash"`
	StartedAt    time.Time                      `json:"startedAt"`
	CompletedAt  *time.Time                     `json:"completedAt,omitempty"`
	Status       string                         `json:"status"`
	Seed         int64                          `json:"seed"`
	Inputs       map[string]string              `json:"inputs"`
	Gold         map[string]string              `json:"gold"`
	Execution    []string                       `json:"executionOrder"`
	Models       map[string]ModelProfile        `json:"models"`
	Pricing      map[string]PriceSnapshot       `json:"pricing"`
	Versions     map[string]string              `json:"versions"`
	Evaluators   map[string]EvaluatorDescriptor `json:"evaluators"`
}

// PriceSnapshot captures the shared gateway rate table at experiment start.
// The map key is provider/model so it can be matched directly to call records.
type PriceSnapshot struct {
	Provider            string  `json:"provider"`
	Model               string  `json:"model"`
	Currency            string  `json:"currency"`
	Priced              bool    `json:"priced"`
	InputPerMillionUSD  float64 `json:"inputPerMillionUsd,omitempty"`
	OutputPerMillionUSD float64 `json:"outputPerMillionUsd,omitempty"`
}

type AttemptStatus struct {
	Attempt       int       `json:"attempt"`
	Status        string    `json:"status"`
	StartedAt     time.Time `json:"startedAt"`
	WallMs        int64     `json:"wallMs"`
	Error         string    `json:"error,omitempty"`
	CallCount     int       `json:"callCount"`
	InternalRetry int       `json:"internalRetries"`
	InputHash     string    `json:"inputHash"`
	Comparison    string    `json:"comparisonStatus,omitempty"`
}

type attemptResult struct {
	status     AttemptStatus
	report     *evalreport.Report
	comparison *Comparison
	complete   bool
	candidate  []CallRecord
	comparator []CallRecord
}

type variantState struct {
	config   VariantConfig
	attempts []attemptResult
}

type Distribution struct {
	Min, Median, Mean, Max *float64 `json:",omitempty"`
}
type CallTotals struct {
	Calls             int      `json:"calls"`
	InputTokens       *int     `json:"inputTokens,omitempty"`
	OutputTokens      *int     `json:"outputTokens,omitempty"`
	ReasoningTokens   *int     `json:"reasoningTokens,omitempty"`
	ContentTokens     *int     `json:"contentTokens,omitempty"`
	UsageMissing      int      `json:"usageMissing"`
	CostedCalls       int      `json:"costedCalls"`
	UnpricedCalls     int      `json:"unpricedCalls"`
	KnownCostUSD      float64  `json:"knownCostUsd"`
	CostUSD           *float64 `json:"costUsd,omitempty"`
	ReasoningMissing  int      `json:"reasoningMissing"`
	IncompleteStreams int      `json:"incompleteStreams"`
	TotalMs           int64    `json:"totalMs"`
}
type ComparisonSummary struct {
	Area              string   `json:"area"`
	Aspect            string   `json:"aspect"`
	Total             int      `json:"total"`
	Comparable        int      `json:"comparable"`
	Aligned           int      `json:"aligned"`
	Overstates        int      `json:"overstates"`
	Understates       int      `json:"understates"`
	NotComparable     int      `json:"notComparable"`
	ManualReview      int      `json:"manualReview"`
	AlignedRate       *float64 `json:"alignedRate,omitempty"`
	NotComparableRate *float64 `json:"notComparableRate,omitempty"`
}
type VariantSummary struct {
	ID                         string              `json:"id"`
	Attempts                   int                 `json:"attempts"`
	SuccessfulRuns             int                 `json:"successfulRuns"`
	SuccessRate                float64             `json:"successRate"`
	Status                     string              `json:"status"`
	Candidate                  CallTotals          `json:"candidate"`
	Comparator                 CallTotals          `json:"comparator"`
	CandidateCostPerSuccessUSD *float64            `json:"candidateCostPerSuccessUsd,omitempty"`
	TotalCostPerSuccessUSD     *float64            `json:"totalCostPerSuccessUsd,omitempty"`
	WallMs                     Distribution        `json:"wallMs"`
	TTFTMs                     Distribution        `json:"ttftMs"`
	Comparison                 []ComparisonSummary `json:"comparison"`
}
type Summary struct {
	ExperimentID string           `json:"experimentId"`
	Variants     []VariantSummary `json:"variants"`
}
type StabilityItem struct {
	Area          string         `json:"area"`
	Code          string         `json:"code"`
	Aspect        string         `json:"aspect"`
	Runs          int            `json:"runs"`
	AgreementRate *float64       `json:"agreementRate,omitempty"`
	Verdicts      map[string]int `json:"verdicts"`
}

func RunExperiment(ctx context.Context, c Config, resultsDir string) (string, int, error) {
	rt, err := NewRuntime(c)
	if err != nil {
		return "", 1, err
	}
	if err := preflight(c, rt); err != nil {
		return "", 1, err
	}
	configBytes, _ := json.Marshal(c)
	started := time.Now().UTC()
	id := started.Format("20060102T150405Z") + "-" + uuid.NewString()
	root := filepath.Join(resultsDir, id)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", 1, err
	}
	seed := started.UnixNano()
	pricing := buildPricingSnapshot(c.Models)
	manifest := Manifest{ExperimentID: id, Name: c.Name, ConfigHash: hashBytes(configBytes), GitCommit: git("rev-parse", "HEAD"), Dirty: git("status", "--porcelain") != "", RubricHash: hashJSON(rubric.Model()), StartedAt: started, Status: "running", Seed: seed, Inputs: map[string]string{}, Gold: map[string]string{}, Models: c.Models, Pricing: pricing, Versions: map[string]string{"adapter": PersonaAdapterVersion, "report": "evaluation-report-v1", "comparator": "comparator-v2", "pricing": gateway.PricingVersion}, Evaluators: evaluatorDescriptors(c.Variants)}
	if err := writeJSON(filepath.Join(root, "manifest.json"), manifest); err != nil {
		return "", 1, err
	}

	states := map[string][]*variantState{}
	inputs := map[string]AdaptedInput{}
	golds := map[string]string{}
	for _, cs := range c.Cases {
		p, err := LoadPersona(c.ResolvePath(cs.ProjectData))
		if err != nil {
			return root, 1, err
		}
		adapted, err := AdaptPersona(cs.ID, p)
		if err != nil {
			return root, 1, err
		}
		gold, err := os.ReadFile(c.ResolvePath(cs.GoldReport))
		if err != nil {
			return root, 1, err
		}
		if err := ValidateGoldMarkdown(string(gold)); err != nil {
			return root, 1, fmt.Errorf("evalbench: case %q: %w", cs.ID, err)
		}
		inputs[cs.ID], golds[cs.ID] = adapted, string(gold)
		manifest.Inputs[cs.ID], manifest.Gold[cs.ID] = adapted.Hash, hashBytes(gold)
		caseDir := filepath.Join(root, "cases", cs.ID)
		if err := os.MkdirAll(caseDir, 0o755); err != nil {
			return root, 1, err
		}
		if err := writeJSON(filepath.Join(caseDir, "input.json"), adapted.Input); err != nil {
			return root, 1, err
		}
		if err := os.WriteFile(filepath.Join(caseDir, "input.sha256"), []byte(adapted.Hash+"\n"), 0o644); err != nil {
			return root, 1, err
		}
		if err := os.WriteFile(filepath.Join(caseDir, "gold-report.md"), gold, 0o644); err != nil {
			return root, 1, err
		}
		if len(adapted.Warnings) > 0 {
			if err := writeJSON(filepath.Join(caseDir, "adapter-warnings.json"), adapted.Warnings); err != nil {
				return root, 1, fmt.Errorf("evalbench: write adapter warnings for case %q: %w", cs.ID, err)
			}
		}
		for _, v := range c.Variants {
			vv := v
			states[cs.ID] = append(states[cs.ID], &variantState{config: vv})
		}
	}
	if err := writeJSON(filepath.Join(root, "manifest.json"), manifest); err != nil {
		return root, 1, fmt.Errorf("evalbench: update manifest before attempts: %w", err)
	}

	rng := rand.New(rand.NewSource(seed))
	cancelled := false
	for _, cs := range c.Cases {
		for {
			active := []*variantState{}
			for _, s := range states[cs.ID] {
				if successes(s) < c.SuccessfulRuns && len(s.attempts) < c.MaxAttempts {
					active = append(active, s)
				}
			}
			if len(active) == 0 {
				break
			}
			rng.Shuffle(len(active), func(i, j int) { active[i], active[j] = active[j], active[i] })
			for _, s := range active {
				if ctx.Err() != nil {
					cancelled = true
					break
				}
				manifest.Execution = append(manifest.Execution, cs.ID+"/"+s.config.ID+fmt.Sprintf("/attempt-%03d", len(s.attempts)+1))
				result, err := runAttempt(ctx, rt, c, cs, inputs[cs.ID], golds[cs.ID], s.config, len(s.attempts)+1, root)
				if err != nil {
					return root, 1, err
				}
				s.attempts = append(s.attempts, result)
			}
			if cancelled {
				break
			}
		}
		if cancelled {
			break
		}
	}
	now := time.Now().UTC()
	manifest.CompletedAt = &now
	if cancelled {
		manifest.Status = "cancelled"
	} else {
		manifest.Status = "completed"
	}
	summary := buildSummaryWithPricing(id, states, c, manifest.Pricing)
	if err := persistFinalArtifacts(root, manifest, summary, states); err != nil {
		return root, 1, err
	}
	if err := writeMarkdownReports(root, c, manifest, summary, states); err != nil {
		return root, 1, err
	}
	code := 0
	for _, v := range summary.Variants {
		if v.Status != "completed" {
			code = 2
		}
	}
	if cancelled {
		code = 2
	}
	return root, code, nil
}

func persistFinalArtifacts(root string, manifest Manifest, summary Summary, states map[string][]*variantState) error {
	if err := writeJSON(filepath.Join(root, "manifest.json"), manifest); err != nil {
		return fmt.Errorf("evalbench: write final manifest: %w", err)
	}
	for caseID, ss := range states {
		for _, s := range ss {
			if err := writeJSON(filepath.Join(root, "cases", caseID, s.config.ID, "stability.json"), buildStability(s.attempts)); err != nil {
				return fmt.Errorf("evalbench: write stability for %s/%s: %w", caseID, s.config.ID, err)
			}
		}
	}
	if err := writeJSON(filepath.Join(root, "summary.json"), summary); err != nil {
		return fmt.Errorf("evalbench: write summary: %w", err)
	}
	return nil
}

func writeMarkdownReports(root string, c Config, manifest Manifest, summary Summary, states map[string][]*variantState) error {
	if err := os.WriteFile(filepath.Join(root, "report-details.md"), []byte(renderDetailedMarkdownReport(c, manifest, summary, states)), 0o644); err != nil {
		return fmt.Errorf("evalbench: write report-details.md: %w", err)
	}
	if err := os.WriteFile(filepath.Join(root, "report.md"), []byte(renderMarkdownReport(c, manifest, summary, states)), 0o644); err != nil {
		return fmt.Errorf("evalbench: write report.md: %w", err)
	}
	return nil
}

func preflight(c Config, rt *Runtime) error {
	for _, cs := range c.Cases {
		if _, err := os.Stat(c.ResolvePath(cs.ProjectData)); err != nil {
			return fmt.Errorf("evalbench: case %q projectData: %w", cs.ID, err)
		}
		if _, err := os.Stat(c.ResolvePath(cs.GoldReport)); err != nil {
			return fmt.Errorf("evalbench: case %q goldReport: %w", cs.ID, err)
		}
		p, err := LoadPersona(c.ResolvePath(cs.ProjectData))
		if err != nil {
			return err
		}
		if _, err := AdaptPersona(cs.ID, p); err != nil {
			return err
		}
		gold, err := os.ReadFile(c.ResolvePath(cs.GoldReport))
		if err != nil {
			return fmt.Errorf("evalbench: case %q goldReport: %w", cs.ID, err)
		}
		if err := ValidateGoldMarkdown(string(gold)); err != nil {
			return fmt.Errorf("evalbench: case %q: %w", cs.ID, err)
		}
	}
	for _, v := range c.Variants {
		e, ok := EvaluatorByID(v.Evaluator)
		if !ok {
			return fmt.Errorf("evalbench: unknown evaluator %q", v.Evaluator)
		}
		if err := e.ValidateParams(v.Params); err != nil {
			return err
		}
		if _, _, err := rt.Resolve(v.Model); err != nil {
			return err
		}
	}
	for _, u := range []ModelUse{c.Comparator} {
		if _, _, err := rt.Resolve(u.Model); err != nil {
			return err
		}
	}
	return nil
}

func runAttempt(ctx context.Context, rt *Runtime, c Config, cs CaseConfig, input AdaptedInput, gold string, v VariantConfig, n int, root string) (attemptResult, error) {
	started := time.Now().UTC()
	res := attemptResult{status: AttemptStatus{Attempt: n, Status: "failed", StartedAt: started, InputHash: input.Hash}}
	dir := filepath.Join(root, "cases", cs.ID, v.ID, fmt.Sprintf("attempt-%03d", n))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return res, fmt.Errorf("evalbench: create attempt directory %s: %w", dir, err)
	}
	evaluator, _ := EvaluatorByID(v.Evaluator)
	resolved, _, err := rt.Resolve(v.Model)
	rec := &CallRecorder{}
	if err == nil {
		attemptInput := input.Input
		attemptInput.ReportID = fmt.Sprintf("evalbench:report:%s:%s:%03d", cs.ID, v.ID, n)
		attemptInput.GeneratedAt = started.Format(time.RFC3339)
		out, runErr := evaluator.Run(ctx, RunDeps{Resolved: resolved, ProviderForPurpose: func(purpose string) gateway.Provider { return rt.Observed(purpose, rec) }}, attemptInput, v.Params)
		err = runErr
		if err == nil {
			res.report = &out.Report
			res.complete = out.Complete
		}
	}
	res.candidate = rec.Calls()
	res.status.CallCount = len(res.candidate)
	res.status.InternalRetry = internalRetries(res.candidate)
	res.status.WallMs = time.Since(started).Milliseconds()
	if err != nil {
		res.status.Error = err.Error()
	} else if err = validateCompleteReport(*res.report); err != nil {
		res.status.Error = err.Error()
		res.report = nil
	} else {
		if res.complete {
			res.status.Status = "success"
		} else {
			res.status.Status, res.status.Error = "partial", "candidate: one or more report subagents degraded"
		}
		if err := writeJSON(filepath.Join(dir, "report.json"), res.report); err != nil {
			return res, fmt.Errorf("evalbench: write candidate report for %s/%s attempt %d: %w", cs.ID, v.ID, n, err)
		}
		if hasIncompleteCall(res.candidate) {
			res.complete = false
			res.status.Status = "partial"
			res.status.Error = "candidate: incomplete provider stream"
			return finishAttemptArtifacts(res, dir)
		}
		cmp, cmpCalls, cmpErr := compareRetry(ctx, rt, c.Comparator, *res.report, gold)
		res.comparator = cmpCalls
		if cmpErr != nil {
			res.status.Status = "partial"
			res.status.Comparison = "failed"
			res.status.Error = "comparator: " + cmpErr.Error()
		} else {
			res.comparison = &cmp
			res.status.Comparison = "success"
			if err := writeJSON(filepath.Join(dir, "comparison.json"), cmp); err != nil {
				return res, fmt.Errorf("evalbench: write comparison for %s/%s attempt %d: %w", cs.ID, v.ID, n, err)
			}
		}
	}
	return finishAttemptArtifacts(res, dir)
}

func finishAttemptArtifacts(res attemptResult, dir string) (attemptResult, error) {
	if err := writeCalls(filepath.Join(dir, "evaluator-calls"), res.candidate); err != nil {
		return res, fmt.Errorf("evalbench: write evaluator calls: %w", err)
	}
	if err := writeCalls(filepath.Join(dir, "comparator-calls"), res.comparator); err != nil {
		return res, fmt.Errorf("evalbench: write comparator calls: %w", err)
	}
	if err := writeJSON(filepath.Join(dir, "status.json"), res.status); err != nil {
		return res, fmt.Errorf("evalbench: write attempt status: %w", err)
	}
	return res, nil
}

func hasIncompleteCall(calls []CallRecord) bool {
	for _, call := range calls {
		if call.Incomplete {
			return true
		}
	}
	return false
}

func compareRetry(ctx context.Context, rt *Runtime, use ModelUse, report evalreport.Report, gold string) (Comparison, []CallRecord, error) {
	var all []CallRecord
	var last error
	for i := 0; i < 2; i++ {
		r := &CallRecorder{}
		c, e := Compare(ctx, rt, use, report, gold, r)
		all = append(all, r.Calls()...)
		if e == nil {
			return c, all, nil
		}
		last = e
	}
	return Comparison{}, all, last
}
func successes(s *variantState) int {
	n := 0
	for _, a := range s.attempts {
		if a.complete && !hasIncompleteCall(a.candidate) && a.report != nil && a.comparison != nil {
			n++
		}
	}
	return n
}
func internalRetries(calls []CallRecord) int {
	byPurpose := map[string]int{}
	for _, c := range calls {
		if c.Purpose != "" && c.Purpose != "comparator" {
			byPurpose[c.Purpose]++
		}
	}
	n := 0
	for _, count := range byPurpose {
		if count > 1 {
			n += count - 1
		}
	}
	return n
}
func writeCalls(dir string, calls []CallRecord) error {
	ordered := append([]CallRecord(nil), calls...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].StartedAt.Equal(ordered[j].StartedAt) {
			return ordered[i].Purpose < ordered[j].Purpose
		}
		return ordered[i].StartedAt.Before(ordered[j].StartedAt)
	})
	for i, c := range ordered {
		d := filepath.Join(dir, fmt.Sprintf("call-%03d", i+1))
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
		if err := writeJSON(filepath.Join(d, "request.json"), c.Request); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(d, "raw-output.txt"), []byte(c.RawOutput), 0o644); err != nil {
			return err
		}
		c.Request = gateway.ChatRequest{}
		c.RawOutput = ""
		if err := writeJSON(filepath.Join(d, "metadata.json"), c); err != nil {
			return err
		}
	}
	return nil
}
func writeJSON(path string, v any) error {
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	return os.WriteFile(path, b, 0o644)
}
func hashBytes(b []byte) string { x := sha256.Sum256(b); return hex.EncodeToString(x[:]) }
func hashJSON(v any) string     { b, _ := json.Marshal(v); return hashBytes(b) }

func priceKey(provider, model string) string { return provider + "/" + model }

func buildPricingSnapshot(models map[string]ModelProfile) map[string]PriceSnapshot {
	out := make(map[string]PriceSnapshot, len(models))
	for _, profile := range models {
		key := priceKey(profile.Provider, profile.Model)
		if _, already := out[key]; already {
			continue
		}
		snapshot := PriceSnapshot{Provider: profile.Provider, Model: profile.Model, Currency: "USD"}
		if price, ok := gateway.LookupTokenPrice(profile.Provider, profile.Model); ok {
			snapshot.Priced = true
			snapshot.InputPerMillionUSD = price.InputPerMillionUSD
			snapshot.OutputPerMillionUSD = price.OutputPerMillionUSD
		}
		out[key] = snapshot
	}
	return out
}

func git(args ...string) string {
	b, e := exec.Command("git", args...).Output()
	if e != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func buildSummary(experimentID string, states map[string][]*variantState, c Config) Summary {
	return buildSummaryWithPricing(experimentID, states, c, buildPricingSnapshot(c.Models))
}

func buildSummaryWithPricing(experimentID string, states map[string][]*variantState, c Config, pricing map[string]PriceSnapshot) Summary {
	out := Summary{ExperimentID: experimentID}
	byID := map[string][]attemptResult{}
	for _, ss := range states {
		for _, s := range ss {
			byID[s.config.ID] = append(byID[s.config.ID], s.attempts...)
		}
	}
	for _, v := range c.Variants {
		a := byID[v.ID]
		s := VariantSummary{ID: v.ID, Attempts: len(a)}
		for _, x := range a {
			if x.complete && !hasIncompleteCall(x.candidate) && x.report != nil && x.comparison != nil {
				s.SuccessfulRuns++
			}
			s.Candidate = addCalls(s.Candidate, x.candidate, pricing)
			s.Comparator = addCalls(s.Comparator, x.comparator, pricing)
		}
		s.Comparison = comparisonSummary(a)
		if s.Attempts > 0 {
			s.SuccessRate = float64(s.SuccessfulRuns) / float64(s.Attempts)
		}
		if s.SuccessfulRuns >= c.SuccessfulRuns {
			s.Status = "completed"
		} else if s.SuccessfulRuns > 0 {
			s.Status = "partial"
		} else {
			s.Status = "failed"
		}
		var wall, ttft []float64
		for _, x := range a {
			if x.complete && x.report != nil && x.comparison != nil {
				wall = append(wall, float64(x.status.WallMs))
				for _, q := range x.candidate {
					if q.FirstOutputMs != nil {
						ttft = append(ttft, float64(*q.FirstOutputMs))
					}
				}
			}
		}
		s.WallMs = distribution(wall)
		s.TTFTMs = distribution(ttft)
		if s.SuccessfulRuns > 0 {
			if s.Candidate.CostUSD != nil {
				cost := *s.Candidate.CostUSD / float64(s.SuccessfulRuns)
				s.CandidateCostPerSuccessUSD = &cost
			}
			if total := combineCallTotals(s.Candidate, s.Comparator); total.CostUSD != nil {
				cost := *total.CostUSD / float64(s.SuccessfulRuns)
				s.TotalCostPerSuccessUSD = &cost
			}
		}
		out.Variants = append(out.Variants, s)
	}
	return out
}

func buildStability(attempts []attemptResult) []StabilityItem {
	counts := map[string]map[string]int{}
	meta := map[string]ComparisonItem{}
	for _, a := range attempts {
		if a.comparison == nil {
			continue
		}
		for _, item := range a.comparison.Items {
			key := comparisonKey(item)
			if counts[key] == nil {
				counts[key] = map[string]int{}
				meta[key] = item
			}
			counts[key][item.Comparison]++
		}
	}
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]StabilityItem, 0, len(keys))
	for _, key := range keys {
		m, total, max := counts[key], 0, 0
		for _, n := range m {
			total += n
			if n > max {
				max = n
			}
		}
		item := meta[key]
		row := StabilityItem{Area: item.Area, Code: item.Code, Aspect: item.Aspect, Runs: total, Verdicts: m}
		if total > 0 {
			rate := float64(max) / float64(total)
			row.AgreementRate = &rate
		}
		out = append(out, row)
	}
	return out
}
func addCalls(t CallTotals, c []CallRecord, pricing map[string]PriceSnapshot) CallTotals {
	t.CostUSD = nil
	for _, r := range c {
		t.Calls++
		t.TotalMs += r.TotalMs
		if r.InputTokens == nil || r.OutputTokens == nil {
			t.UsageMissing++
		} else {
			if t.InputTokens == nil {
				input, output := 0, 0
				t.InputTokens = &input
				t.OutputTokens = &output
			}
			*t.InputTokens += *r.InputTokens
			*t.OutputTokens += *r.OutputTokens
			if snapshot, ok := pricing[priceKey(r.Provider, r.Model)]; ok && snapshot.Priced {
				t.KnownCostUSD += float64(*r.InputTokens)/1e6*snapshot.InputPerMillionUSD + float64(*r.OutputTokens)/1e6*snapshot.OutputPerMillionUSD
				t.CostedCalls++
			} else {
				t.UnpricedCalls++
			}
		}
		if r.ReasoningTokens == nil || r.ContentTokens == nil {
			t.ReasoningMissing++
		} else {
			if t.ReasoningTokens == nil {
				reasoning, content := 0, 0
				t.ReasoningTokens = &reasoning
				t.ContentTokens = &content
			}
			*t.ReasoningTokens += *r.ReasoningTokens
			*t.ContentTokens += *r.ContentTokens
		}
		if r.Incomplete {
			t.IncompleteStreams++
		}
	}
	return finalizeCallTotals(t)
}

func finalizeCallTotals(t CallTotals) CallTotals {
	if t.Calls == t.CostedCalls {
		cost := t.KnownCostUSD
		t.CostUSD = &cost
	}
	return t
}

func combineCallTotals(left, right CallTotals) CallTotals {
	out := CallTotals{
		Calls:             left.Calls + right.Calls,
		UsageMissing:      left.UsageMissing + right.UsageMissing,
		CostedCalls:       left.CostedCalls + right.CostedCalls,
		UnpricedCalls:     left.UnpricedCalls + right.UnpricedCalls,
		KnownCostUSD:      left.KnownCostUSD + right.KnownCostUSD,
		ReasoningMissing:  left.ReasoningMissing + right.ReasoningMissing,
		IncompleteStreams: left.IncompleteStreams + right.IncompleteStreams,
		TotalMs:           left.TotalMs + right.TotalMs,
	}
	out.InputTokens = sumIntPointers(left.InputTokens, right.InputTokens)
	out.OutputTokens = sumIntPointers(left.OutputTokens, right.OutputTokens)
	out.ReasoningTokens = sumIntPointers(left.ReasoningTokens, right.ReasoningTokens)
	out.ContentTokens = sumIntPointers(left.ContentTokens, right.ContentTokens)
	return finalizeCallTotals(out)
}

func sumIntPointers(left, right *int) *int {
	if left == nil && right == nil {
		return nil
	}
	sum := 0
	if left != nil {
		sum += *left
	}
	if right != nil {
		sum += *right
	}
	return &sum
}

func comparisonSummary(attempts []attemptResult) []ComparisonSummary {
	rows := map[string]*ComparisonSummary{}
	for _, a := range attempts {
		if a.comparison == nil {
			continue
		}
		for _, item := range a.comparison.Items {
			key := item.Area + "/" + item.Aspect
			r := rows[key]
			if r == nil {
				r = &ComparisonSummary{Area: item.Area, Aspect: item.Aspect}
				rows[key] = r
			}
			r.Total++
			switch item.Comparison {
			case "aligned":
				r.Aligned++
				r.Comparable++
			case "overstates":
				r.Overstates++
				r.Comparable++
			case "understates":
				r.Understates++
				r.Comparable++
			case "not_comparable":
				r.NotComparable++
			}
			if item.ManualReview {
				r.ManualReview++
			}
		}
	}
	keys := make([]string, 0, len(rows))
	for k := range rows {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]ComparisonSummary, 0, len(keys))
	for _, k := range keys {
		r := rows[k]
		if r.Comparable > 0 {
			x := float64(r.Aligned) / float64(r.Comparable)
			r.AlignedRate = &x
		}
		if r.Total > 0 {
			x := float64(r.NotComparable) / float64(r.Total)
			r.NotComparableRate = &x
		}
		out = append(out, *r)
	}
	return out
}
func distribution(xs []float64) Distribution {
	if len(xs) == 0 {
		return Distribution{}
	}
	sort.Float64s(xs)
	sum := 0.0
	for _, x := range xs {
		sum += x
	}
	min, mean, max := xs[0], sum/float64(len(xs)), xs[len(xs)-1]
	med := xs[len(xs)/2]
	if len(xs)%2 == 0 {
		med = (xs[len(xs)/2-1] + med) / 2
	}
	return Distribution{&min, &med, &mean, &max}
}
