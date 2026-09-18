// Package routebench is the routing workbench: it measures what each candidate
// model costs, how fast it answers, and whether its answer is usable, for every
// capability class.
//
// It is deliberately SEPARATE from the running system. Nothing here is imported
// by cmd/api; nothing here touches a database; it never runs on a request path.
// It is a tool you point at the catalog when you want to revise the routing
// strategy — this quarter, or next year against models that do not exist yet —
// and it answers with a table and a recommended set of bindings that a human
// then applies to models.json.
//
// What it measures, and why in this order:
//
//	structural validity — the output is fed to the REAL production parser for
//	  that call site. Free, objective, and unarguable: if the parser refuses it,
//	  production shows the student an error. A model that fails here is out,
//	  whatever else it scores.
//	latency — time to first token and total, median of n. A single sample
//	  swings by a third, which on 2026-09-02 was enough to invent a reasoning
//	  control that did not exist.
//	tokens — including reasoning tokens, which is where a "cheap" model on a
//	  thinking route stops being cheap.
//	quality — a judging model scores the cases whose merit cannot be read off
//	  the structure. A chaperone turn is either well-pitched or it is not, and
//	  no parser can tell.
package routebench

import (
	"context"
	"fmt"
	"sort"
	"time"

	"mindimprint/api/internal/benchcase"
	"mindimprint/api/internal/gateway"
)

// Config is the experiment: which models to try for which classes, and how
// many times. It is a file so that re-running the strategy later is an edit
// plus one command, not a code change.
type Config struct {
	// Candidates maps a class to the catalog model ids to try for it. A class
	// absent here is skipped. "*" means every chat-capable model in the catalog
	// that the class would accept.
	Candidates map[string][]string `json:"candidates"`
	// Samples is how many times each (case, model) pair runs. Below 3 the
	// medians are not worth printing.
	Samples int `json:"samples"`
	// JudgeModel is the catalog model id that scores quality. It must be a
	// flagship model and it must NOT be one of the candidates — a model grading
	// its own homework is not a measurement.
	JudgeModel string `json:"judgeModel"`
	// Suite restricts a run to cases with this stable suite identifier.
	Suite string `json:"suite,omitempty"`
	// CaseFilter, when non-empty, restricts the run to case ids containing any
	// of these substrings.
	CaseFilter []string `json:"caseFilter,omitempty"`
	// TimeoutSeconds bounds a single call.
	TimeoutSeconds int `json:"timeoutSeconds"`
}

// Sample is one call.
type Sample struct {
	TTFT    time.Duration
	Total   time.Duration
	In, Out int
	Reason  int
	// FirstText and RetryText preserve both model attempts when a parser retry occurs.
	FirstText       string
	RetryText       string
	RetryErr        string
	SelectedAttempt int
	Text            string
	// Parsed is the result of the production wire parser. Valid below is the
	// fixture expectation result; keeping them separate prevents a teaching
	// error from being reported as malformed JSON.
	Parsed   bool
	ParseErr string
	Valid    bool
	ValidErr string
	Gold     bool
	GoldErr  string
	Err      string
	// Retries is the number of production-equivalent recovery attempts made for
	// this sample. In/Out/Reason/Total include those attempts.
	Retries  int
	Judge    float64
	JudgeWhy string
}

// Result is one (case × model) cell.
type Result struct {
	Suite    string
	Version  int
	CaseID   string
	Class    string
	Site     string
	ModelID  string
	Samples  []Sample
	Judge    float64 // mean 1–5; 0 when not judged
	JudgeWhy string
	// Skipped records why this pair was never run — most usefully, a catalog
	// rule that refused the binding (a model that cannot stop reasoning on a
	// class that must not reason). A refusal is a finding, not an absence.
	Skipped string
}

// JudgeScores returns the scores recorded for this cell. It is separate from
// Result.Judge so old routing reports keep their mean score semantics.
func (r Result) JudgeScores() []float64 {
	var out []float64
	for _, s := range r.Samples {
		if s.Judge > 0 {
			out = append(out, s.Judge)
		}
	}
	return out
}

func (r Result) JudgeMin() float64 {
	xs := r.JudgeScores()
	if len(xs) == 0 {
		return 0
	}
	m := xs[0]
	for _, x := range xs[1:] {
		if x < m {
			m = x
		}
	}
	return m
}

func (r Result) JudgeMedian() float64 {
	xs := r.JudgeScores()
	if len(xs) == 0 {
		return 0
	}
	sort.Float64s(xs)
	return xs[len(xs)/2]
}

// Median helpers. Medians, not means: one slow sample from a cold route should
// not move the number that a routing decision gets made on.
func medianDur(xs []time.Duration) time.Duration {
	if len(xs) == 0 {
		return 0
	}
	s := append([]time.Duration(nil), xs...)
	sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
	return s[len(s)/2]
}

func medianInt(xs []int) int {
	if len(xs) == 0 {
		return 0
	}
	s := append([]int(nil), xs...)
	sort.Ints(s)
	return s[len(s)/2]
}

// P50TTFT, P50Total, MedOut, MedReasoning, ParseRate and ExpectedRate
// summarise a cell.
func (r Result) P50TTFT() time.Duration {
	var xs []time.Duration
	for _, s := range r.Samples {
		if s.Err == "" {
			xs = append(xs, s.TTFT)
		}
	}
	return medianDur(xs)
}

func (r Result) P50Total() time.Duration {
	var xs []time.Duration
	for _, s := range r.Samples {
		if s.Err == "" {
			xs = append(xs, s.Total)
		}
	}
	return medianDur(xs)
}

func (r Result) MedOut() int {
	var xs []int
	for _, s := range r.Samples {
		if s.Err == "" {
			xs = append(xs, s.Out)
		}
	}
	return medianInt(xs)
}

func (r Result) MedIn() int {
	var xs []int
	for _, s := range r.Samples {
		if s.Err == "" {
			xs = append(xs, s.In)
		}
	}
	return medianInt(xs)
}

func (r Result) MedReasoning() int {
	var xs []int
	for _, s := range r.Samples {
		if s.Err == "" {
			xs = append(xs, s.Reason)
		}
	}
	return medianInt(xs)
}

// ParseRate is the share of successful calls whose selected output crossed the
// production parser boundary. Cases with no parser report -1.
func (r Result) ParseRate() float64 {
	ok, n := 0, 0
	for _, s := range r.Samples {
		if s.Err != "" && s.ParseErr == "" {
			continue
		}
		if s.ParseErr == "n/a" {
			return -1
		}
		n++
		if s.Parsed {
			ok++
		}
	}
	if n == 0 {
		return -1
	}
	return float64(ok) / float64(n)
}

// ExpectedRate is the share of completed calls that meet the fixture's
// deterministic teaching and state expectations. Cases with no expectation
// contract report -1.
func (r Result) ExpectedRate() float64 {
	ok, n := 0, 0
	for _, s := range r.Samples {
		if s.Err != "" {
			continue
		}
		if s.ValidErr == "n/a" {
			return -1
		}
		n++
		if s.Valid {
			ok++
		}
	}
	if n == 0 {
		return -1
	}
	return float64(ok) / float64(n)
}

// GoldRate is the share of structurally valid outputs that met this case's
// objective semantic expectation. Cases without GoldCheck report -1 (not
// applicable), keeping a missing gold label distinct from a failed one.
func (r Result) GoldRate() float64 {
	ok, n := 0, 0
	for _, s := range r.Samples {
		if s.Err != "" || !s.Valid {
			continue
		}
		if !s.Gold && s.GoldErr == "" {
			continue // no GoldCheck on this case
		}
		n++
		if s.Gold {
			ok++
		}
	}
	if n == 0 {
		return -1
	}
	return float64(ok) / float64(n)
}

// ValidRate is kept as a compatibility alias for older routing callers. It is
// expectation validity, not parser validity; new reports must name it as such.
func (r Result) ValidRate() float64 { return r.ExpectedRate() }

// ErrRate is the share of calls that never came back at all.
func (r Result) ErrRate() float64 {
	if len(r.Samples) == 0 {
		return 0
	}
	bad := 0
	for _, s := range r.Samples {
		if s.Err != "" {
			bad++
		}
	}
	return float64(bad) / float64(len(r.Samples))
}

func (r Result) RetryRate() float64 {
	if len(r.Samples) == 0 {
		return 0
	}
	n := 0
	for _, s := range r.Samples {
		if s.Retries > 0 {
			n++
		}
	}
	return float64(n) / float64(len(r.Samples))
}

// Runner holds everything one experiment needs. No database, no server.
type Runner struct {
	Cat      *gateway.Catalog
	Provider gateway.Provider
	Keys     gateway.KeyLookup
	Cfg      Config
	// Log receives progress lines. A run takes minutes; silence looks like a hang.
	Log func(format string, args ...any)
	// OnProgress, when set, receives the results so far after each case, so the
	// caller can checkpoint a partial report. See Run.
	OnProgress func([]Result)
}

func (rn *Runner) logf(format string, args ...any) {
	if rn.Log != nil {
		rn.Log(format, args...)
	}
}

// Run executes the whole experiment and returns one Result per (case × model).
//
// OnProgress is called after every case with everything measured so far. A full
// run is minutes of PAID calls, and on 2026-09-03 one was killed inside the
// assess class — which spends 100 seconds a call — after every other class had
// already been measured. All of it was lost, because the report was only
// written at the end. Handing the caller partial results lets it checkpoint,
// so an interrupted run costs the remaining calls rather than all of them.
func (rn *Runner) Run(ctx context.Context, cases []benchcase.Case) []Result {
	var out []Result
	for _, c := range cases {
		if !rn.wanted(c) {
			continue
		}
		for _, modelID := range rn.candidates(c.Class) {
			out = append(out, rn.runCell(ctx, c, modelID))
		}
		if rn.OnProgress != nil {
			rn.OnProgress(out)
		}
	}
	return out
}

func (rn *Runner) wanted(c benchcase.Case) bool {
	if rn.Cfg.Suite != "" && c.Suite != rn.Cfg.Suite {
		return false
	}
	if len(rn.Cfg.CaseFilter) == 0 {
		return true
	}
	for _, f := range rn.Cfg.CaseFilter {
		if contains(c.ID, f) {
			return true
		}
	}
	return false
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// candidates resolves the configured candidate list for a class, expanding "*"
// to every chat model in the catalog.
func (rn *Runner) candidates(class string) []string {
	list := rn.Cfg.Candidates[class]
	if len(list) == 1 && list[0] == "*" {
		var all []string
		for _, id := range rn.Cat.ModelIDs() {
			if m := rn.Cat.Models[id]; m.Has(gateway.CapChat) {
				all = append(all, id)
			}
		}
		return all
	}
	return list
}

// runCell runs one (case × model) pair.
//
// Resolution goes through the catalog's own Resolve with the model as a class
// override, NOT through a hand-built Resolved. That is what makes the bench
// measure production: the class's reasoning requirement, the provider's
// thinking knob, and every boot-time refusal all apply here exactly as they
// would at a student's turn. A binding the catalog refuses is reported as a
// refusal with its reason — which is itself a result worth having.
func (rn *Runner) runCell(ctx context.Context, c benchcase.Case, modelID string) Result {
	res := Result{Suite: c.Suite, Version: c.Version, CaseID: c.ID, Class: c.Class, Site: c.Site, ModelID: modelID}
	resolved, err := rn.Cat.Resolve(c.Class, modelID, rn.Keys)
	if err != nil {
		res.Skipped = err.Error()
		rn.logf("  %-34s %-30s SKIP %v", c.ID, modelID, err)
		return res
	}
	for i := 0; i < rn.Cfg.Samples; i++ {
		s := rn.once(ctx, resolved, c)
		res.Samples = append(res.Samples, s)
	}
	rn.logf("  %-34s %-30s p50 %5s  out %4d (think %4d)  parse %s expected %s gold %s",
		c.ID, modelID, res.P50Total().Round(100*time.Millisecond), res.MedOut(), res.MedReasoning(), pct(res.ParseRate()), pct(res.ExpectedRate()), pct(res.GoldRate()))
	return res
}

func pct(v float64) string {
	if v < 0 {
		return "—"
	}
	return fmt.Sprintf("%.0f%%", v*100)
}

// once issues one streamed call and times the first visible token separately
// from the whole answer. On a reasoning route those two differ by tens of
// seconds, and it is the FIRST one the student experiences as "it is alive".
func (rn *Runner) once(ctx context.Context, r gateway.Resolved, c benchcase.Case) Sample {
	s := rn.attempt(ctx, r, c)
	s.FirstText = s.Text
	if s.Err != "" {
		return s
	}
	s.SelectedAttempt = 1
	if c.Parse != nil {
		if err := c.Parse(s.Text); err != nil {
			again := rn.attempt(ctx, r, c)
			s.Retries = 1
			s.RetryText, s.RetryErr = again.Text, again.Err
			s.Total += again.Total
			s.In += again.In
			s.Out += again.Out
			s.Reason += again.Reason
			if again.Err != "" {
				s.Err = "recovery attempt failed: " + again.Err
				s.ParseErr = err.Error()
				return s
			}
			s.Text = again.Text
			s.SelectedAttempt = 2
			if retryErr := c.Parse(s.Text); retryErr != nil {
				s.ParseErr = retryErr.Error()
				s.Err = "production parser rejected recovery reply: " + retryErr.Error()
				return s
			}
		}
		s.Parsed = true
	} else {
		s.ParseErr = "n/a"
	}
	if c.Validate == nil {
		s.ValidErr = "n/a"
		return s
	}
	if err := c.Validate(s.Text); err != nil {
		s.ValidErr = err.Error()
		return s
	}
	s.Valid = true
	return s
}
func (rn *Runner) attempt(ctx context.Context, r gateway.Resolved, c benchcase.Case) Sample {
	timeout := time.Duration(rn.Cfg.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	start := time.Now()
	stream, err := rn.Provider.Stream(cctx, r, c.Request)
	if err != nil {
		return Sample{Err: err.Error(), Total: time.Since(start)}
	}
	var s Sample
	var text []byte
	for ev := range stream {
		switch ev.Kind {
		case gateway.EventTextDelta:
			if s.TTFT == 0 {
				s.TTFT = time.Since(start)
			}
			text = append(text, ev.TextDelta...)
		case gateway.EventUsage:
			if ev.Usage != nil {
				s.In, s.Out = ev.Usage.InputTokens, ev.Usage.OutputTokens
				if ev.Usage.ReasoningTokens != nil {
					s.Reason = *ev.Usage.ReasoningTokens
				}
			}
		}
	}
	s.Total = time.Since(start)
	s.Text = string(text)
	if len(text) == 0 {
		s.Err = "empty completion"
		return s
	}
	// The gold check is independent of the production parser and fixture
	// expectations, but only runs on a student-visible result that passed them.
	if c.GoldCheck != nil {
		if gerr := c.GoldCheck(s.Text); gerr != nil {
			s.GoldErr = gerr.Error()
			return s
		}
		s.Gold = true
	}
	return s
}
