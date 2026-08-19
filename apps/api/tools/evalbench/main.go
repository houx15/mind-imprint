package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"mindimprint/api/internal/evalbench"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	switch os.Args[1] {
	case "run":
		run(ctx, os.Args[2:])
	default:
		usage()
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	config := fs.String("config", filepath.Join("tools", "evalbench", "config.json"), "experiment config JSON")
	results := fs.String("results-dir", "", "results directory")
	_ = fs.Parse(args)
	c, err := evalbench.LoadConfig(*config)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if *results == "" {
		*results = filepath.Join("tools", "evalbench", "results")
	}
	root, code, err := evalbench.RunExperiment(ctx, c, *results)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(root)
	os.Exit(code)
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: evalbench run [--config experiment.json]")
}
