// Command routebench measures every candidate model against every capability
// class and prints a recommended set of bindings.
//
// It is SEPARATE from the running system on purpose: no database, no server, no
// import from cmd/api. It reads the embedded catalog, replays bench cases built
// from the real production prompts, and writes a Markdown report. Applying its
// recommendation is a human editing models.json — the workbench never changes
// what production serves.
//
// That separation is the point of it existing at all: the strategy will be
// revised again, against models that do not exist yet, and revising it should
// be one command rather than a re-derivation.
//
//	DASHSCOPE_API_KEY=… go run ./cmd/routebench -config routebench.json -out report.md
//
// Flags:
//
//	-config   experiment file (default cmd/routebench/routebench.json)
//	-out      where to write the Markdown report (default stdout)
//	-cases    comma-separated substrings; restricts which cases run
//	-samples  override the config's sample count
//	-list     print the cases and candidates, call nothing, spend nothing
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/api"
	"mindimprint/api/internal/benchcase"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/pbl"
	"mindimprint/api/internal/routebench"
)

func main() {
	var (
		configPath = flag.String("config", "cmd/routebench/routebench.json", "experiment config")
		outPath    = flag.String("out", "", "write the Markdown report here (default: stdout)")
		jsonOut    = flag.String("json-out", "", "write a machine-readable artifact")
		compare    = flag.String("compare", "", "compare this run with an earlier artifact")
		check      = flag.Bool("check", false, "exit non-zero on model-call or final parse failures")
		suite      = flag.String("suite", "", "run a named evaluation suite")
		bound      = flag.Bool("bound", false, "test the current binding for the suite's class")
		caseFilter = flag.String("cases", "", "comma-separated substrings of case ids to run")
		samples    = flag.Int("samples", 0, "override the config's sample count")
		list       = flag.Bool("list", false, "print cases and candidates without calling anything")
	)
	flag.Parse()

	cases := allCases()

	cfg, err := loadConfig(*configPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	if *caseFilter != "" {
		cfg.CaseFilter = strings.Split(*caseFilter, ",")
	}
	if *samples > 0 {
		cfg.Samples = *samples
	}
	if *suite != "" {
		cfg.Suite = *suite
	}

	cat, err := gateway.DefaultCatalog()
	if err != nil {
		log.Fatalf("catalog: %v", err)
	}
	if *bound {
		if cfg.Suite == "" {
			log.Fatal("-bound requires -suite")
		}
		boundModel := cat.Lanes[gateway.ClassDialogue].Model
		if boundModel == "" {
			log.Fatal("dialogue has no current binding")
		}
		if boundModel == cfg.JudgeModel {
			log.Fatal("current dialogue binding cannot judge itself")
		}
		cfg.Candidates = map[string][]string{gateway.ClassDialogue: {boundModel}}
	}

	if *list {
		printPlan(cat, cfg, wantedCases(cases, cfg))
		return
	}

	// One HTTP client for the run. Generous timeout: a flagship model on a long
	// review case legitimately takes minutes, and a client timeout would be
	// recorded as a model failure, which is a lie the report would carry.
	httpc := &http.Client{Timeout: 10 * time.Minute}
	rn := &routebench.Runner{
		Cat: cat,
		Provider: gateway.NewMuxProvider(map[string]gateway.Provider{
			gateway.KindOpenAICompatible: gateway.NewCatalogProvider(httpc),
			gateway.KindAnthropic:        gateway.NewAnthropicProvider(httpc),
		}),
		Keys: gateway.OSEnvKeyLookup,
		Cfg:  cfg,
		Log:  func(f string, a ...any) { fmt.Fprintf(os.Stderr, f+"\n", a...) },
	}

	started := time.Now()
	ctx := context.Background()
	write := func(results []routebench.Result) {
		md := routebench.Markdown(results, cat, cfg, started)
		if *outPath == "" {
			return
		}
		if werr := os.WriteFile(*outPath, []byte(md), 0o644); werr != nil {
			fmt.Fprintf(os.Stderr, "write report: %v\n", werr)
		}
	}
	// Checkpoint after every case. A full run is minutes of PAID calls; on
	// 2026-09-03 one was killed inside the assess class, which spends ~100
	// seconds a call, after every other class had already been measured — and
	// all of it was lost because the report was written only at the end.
	rn.OnProgress = write

	caseCount := len(cases)
	if cfg.Suite != "" {
		caseCount = 0
		for _, c := range cases {
			if c.Suite == cfg.Suite {
				caseCount++
			}
		}
	}
	fmt.Fprintf(os.Stderr, "routebench: %d cases, n=%d\n", caseCount, cfg.Samples)
	results := rn.Run(ctx, cases)
	fmt.Fprintf(os.Stderr, "\njudging with %s …\n", cfg.JudgeModel)
	rn.Judge(ctx, cases, results)

	artifact := routebench.NewArtifact(cases, cfg, results, started)
	if *jsonOut != "" {
		if err := routebench.WriteArtifact(*jsonOut, artifact); err != nil {
			log.Fatalf("write artifact: %v", err)
		}
	}
	comparison := ""
	if *compare != "" {
		base, err := routebench.ReadArtifact(*compare)
		if err != nil {
			log.Fatalf("read comparison run: %v", err)
		}
		comparison = routebench.CompareArtifactMarkdown(base, artifact)
	}
	md := routebench.Markdown(results, cat, cfg, started) + comparison
	if *outPath == "" {
		fmt.Print(md)
	} else {
		if err := os.WriteFile(*outPath, []byte(md), 0o644); err != nil {
			log.Fatalf("write report: %v", err)
		}
		fmt.Fprintf(os.Stderr, "\nwrote %s (%s)\n", *outPath, time.Since(started).Round(time.Second))
	}
	if failures := routebench.TechnicalFailures(artifact); len(failures) > 0 {
		fmt.Fprintln(os.Stderr, "routebench technical failures:\n- "+strings.Join(failures, "\n- "))
		if *check {
			os.Exit(1)
		}
	}
	return
}

func wantedCases(cases []benchcase.Case, cfg routebench.Config) []benchcase.Case {
	var out []benchcase.Case
	for _, c := range cases {
		if cfg.Suite != "" && c.Suite != cfg.Suite {
			continue
		}
		if len(cfg.CaseFilter) == 0 {
			out = append(out, c)
			continue
		}
		for _, f := range cfg.CaseFilter {
			if strings.Contains(c.ID, f) {
				out = append(out, c)
				break
			}
		}
	}
	return out
}

// allCases gathers the cases from the packages that own the prompts. Adding a
// case is adding it next to the prompt it measures — never here, and never as
// a copy.
func allCases() []benchcase.Case {
	var out []benchcase.Case
	out = append(out, agent.BenchCases()...)
	out = append(out, pbl.BenchCases()...)
	out = append(out, api.BenchCases()...)
	return out
}

func loadConfig(path string) (routebench.Config, error) {
	var cfg routebench.Config
	b, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	if uerr := json.Unmarshal(b, &cfg); uerr != nil {
		return cfg, uerr
	}
	if cfg.Samples < 1 {
		cfg.Samples = 3
	}
	// 🚨 A judge that is also a candidate cannot score itself, so that
	// candidate reaches the recommendation with NO quality score — and then
	// wins on token count alone, which is exactly a cheap answer beating a good
	// one. This happened on 2026-09-03 with qwen3.8-max in both roles, and the
	// rule had been written down in the README the whole time. A rule that only
	// lives in a README gets broken; this one now stops the run.
	for class, models := range cfg.Candidates {
		for _, m := range models {
			if m == cfg.JudgeModel {
				return cfg, fmt.Errorf("judgeModel %q is also a candidate for class %q — "+
					"it cannot grade its own homework, and would reach the recommendation unscored",
					cfg.JudgeModel, class)
			}
		}
	}
	return cfg, nil
}

func printPlan(cat *gateway.Catalog, cfg routebench.Config, cases []benchcase.Case) {
	fmt.Printf("catalog %s\n", cat.Version)
	if cfg.Suite != "" {
		fmt.Printf("suite %s\n", cfg.Suite)
	}
	fmt.Printf("\nCASES\n")
	for _, c := range cases {
		judged := ""
		if strings.TrimSpace(c.Judge) != "" {
			judged = " [judged]"
		}
		checks := ""
		if c.Parse != nil {
			checks += " [parsed]"
		}
		gold := ""
		if c.GoldCheck != nil {
			gold = " [gold]"
		}
		if c.Validate != nil {
			checks += " [checked]"
		}
		fmt.Printf("  %-34s %-10s%s%s%s\n    %s\n", c.ID, c.Class, checks, gold, judged, c.Site)
	}
	fmt.Printf("\nCANDIDATES (n=%d each)\n", cfg.Samples)
	for _, class := range gateway.Classes {
		if list := cfg.Candidates[class]; len(list) > 0 {
			fmt.Printf("  %-11s %s\n", class, strings.Join(list, ", "))
		}
	}
	fmt.Printf("\njudge: %s\n", cfg.JudgeModel)
}
