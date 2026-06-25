package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"

	"mindimprint/api/internal/api"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/config"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store"
	"mindimprint/api/internal/store/sqlc"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

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

	queries := sqlc.New(pool)

	catalog, err := cards.Catalog()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cards: %v\n", err)
		pool.Close()
		os.Exit(1)
	}
	specIndex := make(map[string]cards.Spec, len(catalog))
	for _, s := range catalog {
		specIndex[s.ID] = s
	}
	specByID := func(id string) (cards.Spec, bool) { s, ok := specIndex[id]; return s, ok }

	// No global timeout on the HTTP client: streaming is governed by request ctx.
	httpClient := &http.Client{}
	provider := gateway.NewMuxProvider(map[string]gateway.Provider{
		"deepseek":  gateway.NewDeepSeekProvider(httpClient),
		"anthropic": gateway.NewAnthropicProvider(httpClient),
	})

	apiHandler := api.New(api.Deps{
		Queries:      queries,
		Provider:     provider,
		ChatResolver: gateway.NewKeyResolver(cfg),
		EvalResolver: gateway.NewEvalKeyResolver(cfg),
		Catalog:      catalog,
		SpecByID:     specByID,
		Pool:         pool,
		CookieSecure: cfg.CookieSecure,
	}).Handler()

	srv := httpx.NewServer(cfg, pool, apiHandler)

	if err := httpx.RunServer(srv, func(_ context.Context) {
		pool.Close()
	}); err != nil {
		fmt.Fprintf(os.Stderr, "server: %v\n", err)
		os.Exit(1)
	}
}
