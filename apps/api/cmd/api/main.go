package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"mindimprint/api/internal/config"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store"
)

func main() {
	migrateUp := flag.Bool("migrate-up", false, "run database migrations then exit")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()

	pool, err := store.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "db pool: %v\n", err)
		os.Exit(1)
	}

	if *migrateUp {
		if err := store.RunMigrations(ctx, pool); err != nil {
			fmt.Fprintf(os.Stderr, "migrations: %v\n", err)
			pool.Close()
			os.Exit(1)
		}
		log.Println("migrations applied successfully")
		pool.Close()
		return
	}

	srv := httpx.NewServer(cfg, pool)

	if err := httpx.RunServer(srv, func(_ context.Context) {
		// Give pool close its own fresh deadline so a slow server shutdown
		// can't starve it (cross-task decision from Task 5 review).
		closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = closeCtx // pool.Close is synchronous; context reserved for future async close
		pool.Close()
	}); err != nil {
		fmt.Fprintf(os.Stderr, "server: %v\n", err)
		os.Exit(1)
	}
}
