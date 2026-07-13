package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/api"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/config"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store"
	"mindimprint/api/internal/store/sqlc"
	"mindimprint/api/internal/voice"
)

// riverEnqueuer adapts the river client to the api.Enqueuer seam.
type riverEnqueuer struct{ c *river.Client[pgx.Tx] }

func (e riverEnqueuer) EnqueueEvaluate(ctx context.Context, args agent.EvaluateArgs) error {
	_, err := e.c.Insert(ctx, args, nil)
	return err
}

// buildVoice constructs the production VoiceService only when Volcano
// Engine credentials are configured; otherwise it returns nil so Deps.Voice
// stays nil and the platform still boots with voice routes 503ing.
func buildVoice(cfg config.Config) api.VoiceService {
	if cfg.VoiceAppID == "" || cfg.VoiceAccessKey == "" {
		return nil
	}
	return api.NewVoiceService(voice.Config{
		AppID:         cfg.VoiceAppID,
		AccessKey:     cfg.VoiceAccessKey,
		TTSVoice:      cfg.VoiceTTSVoice,
		TTSResourceID: cfg.VoiceTTSResource,
		ASRResourceID: cfg.VoiceASRResource,
	})
}

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

	// Embedded river client: the EvaluateWorker runs in-process off the default
	// queue, using the FLAGSHIP eval resolver (never downgraded).
	workers := river.NewWorkers()
	river.AddWorker(workers, &agent.EvaluateWorker{
		Store:    agent.NewSqlcEvalStore(queries),
		Provider: provider,
		Resolver: gateway.NewEvalKeyResolver(cfg), // FLAGSHIP
		SpecByID: specByID,
	})
	riverClient, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Queues:  map[string]river.QueueConfig{river.QueueDefault: {MaxWorkers: 2}},
		Workers: workers,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "river client: %v\n", err)
		pool.Close()
		os.Exit(1)
	}
	if err := riverClient.Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "river start: %v\n", err)
		pool.Close()
		os.Exit(1)
	}

	apiHandler := api.New(api.Deps{
		Queries:      queries,
		Provider:     provider,
		ChatResolver: gateway.NewKeyResolver(cfg),
		EvalResolver: gateway.NewEvalKeyResolver(cfg),
		Catalog:      catalog,
		SpecByID:     specByID,
		Pool:         pool,
		CookieSecure: cfg.CookieSecure,
		Enqueuer:     riverEnqueuer{c: riverClient},
		Voice:        buildVoice(cfg),
		CORSOrigins:  cfg.CORSOrigins,
	}).Handler()

	srv := httpx.NewServer(cfg, pool, apiHandler)

	if err := httpx.RunServer(srv, func(shutdownCtx context.Context) {
		_ = riverClient.Stop(shutdownCtx)
		pool.Close()
	}); err != nil {
		fmt.Fprintf(os.Stderr, "server: %v\n", err)
		os.Exit(1)
	}
}
