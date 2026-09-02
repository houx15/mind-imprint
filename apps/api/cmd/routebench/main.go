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

	cat, err := gateway.DefaultCatalog()
	if err != nil {
		log.Fatalf("catalog: %v", err)
	}

	if *list {
		printPlan(cat, cfg, cases)
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
	fmt.Fprintf(os.Stderr, "routebench: %d cases, n=%d\n", len(cases), cfg.Samples)
	results := rn.Run(ctx, cases)
	fmt.Fprintf(os.Stderr, "\njudging with %s …\n", cfg.JudgeModel)
	rn.Judge(ctx, cases, results)

	md := routebench.Markdown(results, cat, cfg, started)
	if *outPath == "" {
		fmt.Print(md)
	} else if werr := os.WriteFile(*outPath, []byte(md), 0o644); werr != nil {
		log.Fatalf("write report: %v", werr)
	} else {
		fmt.Fprintf(os.Stderr, "\nwrote %s (%s)\n", *outPath, time.Since(started).Round(time.Second))
	}
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
	return cfg, nil
}

func printPlan(cat *gateway.Catalog, cfg routebench.Config, cases []benchcase.Case) {
	fmt.Printf("catalog %s\n\nCASES\n", cat.Version)
	for _, c := range cases {
		judged := ""
		if strings.TrimSpace(c.Judge) != "" {
			judged = " [judged]"
		}
		structural := ""
		if c.Validate != nil {
			structural = " [structural]"
		}
		fmt.Printf("  %-34s %-10s%s%s\n    %s\n", c.ID, c.Class, structural, judged, c.Site)
	}
	fmt.Printf("\nCANDIDATES (n=%d each)\n", cfg.Samples)
	for _, class := range gateway.Classes {
		if list := cfg.Candidates[class]; len(list) > 0 {
			fmt.Printf("  %-11s %s\n", class, strings.Join(list, ", "))
		}
	}
	fmt.Printf("\njudge: %s\n", cfg.JudgeModel)
}
