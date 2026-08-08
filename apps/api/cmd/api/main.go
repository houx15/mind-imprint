package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/api"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/config"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/materialize"
	"mindimprint/api/internal/oss"
	"mindimprint/api/internal/store"
	"mindimprint/api/internal/store/sqlc"
	"mindimprint/api/internal/voice"
)

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

	// Hoisted above the --migrate-up branch (course voice narration, Task 4):
	// SeedCourses now (re)generates each seeded course's narration manifest,
	// so migrate-up needs the same Voice/OSS dependencies the server path
	// builds below. Both are nil-safe when unconfigured (oss.New returns
	// (nil, nil); buildVoice returns nil) — narration generation degrades to
	// an empty manifest rather than failing migrate-up or server boot.
	ossSvc, err := oss.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "oss: %v\n", err)
		pool.Close()
		os.Exit(1)
	}
	voiceSvc := buildVoice(cfg)

	if *migrateUp {
		if err := store.RunMigrations(ctx, pool); err != nil {
			fmt.Fprintf(os.Stderr, "migrations: %v\n", err)
			pool.Close()
			os.Exit(1)
		}
		log.Println("migrations applied successfully")

		// Guarded the same way course_admin.go's postAdminUploadCourse guards
		// a.d.Voice/a.d.OSS: voiceSvc is already a nil-safe interface, but
		// ossSvc is a concrete *oss.Service — assigning a nil *oss.Service
		// straight into the agent.CourseAudioStore interface parameter would
		// NOT compare equal to nil inside GenerateCourseAudio (typed-nil-in-
		// interface), so it's explicitly guarded here too.
		var audioSynth agent.CourseAudioSynth
		if voiceSvc != nil {
			audioSynth = voiceSvc
		}
		var audioStore agent.CourseAudioStore
		if ossSvc != nil {
			audioStore = ossSvc
		}

		seeded, err := store.SeedCourses(ctx, pool, audioSynth, audioStore)
		if err != nil {
			fmt.Fprintf(os.Stderr, "seed courses: %v\n", err)
			pool.Close()
			os.Exit(1)
		}
		log.Printf("seeded %d course(s)", seeded)

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

	// ossSvc/voiceSvc already constructed above (hoisted for the --migrate-up
	// path's SeedCourses call); reused here unchanged.

	// No global timeout on the HTTP client: streaming is governed by request ctx.
	httpClient := &http.Client{}
	provider := gateway.NewMuxProvider(map[string]gateway.Provider{
		"deepseek":  gateway.NewDeepSeekProvider(httpClient),
		"anthropic": gateway.NewAnthropicProvider(httpClient),
	})

	apiHandler := api.New(api.Deps{
		Queries:          queries,
		Provider:         provider,
		ChatResolver:     gateway.NewKeyResolver(cfg),
		FastChatResolver: gateway.NewFastChaperoneResolver(cfg), // fast per-status studio router
		EvalResolver:     gateway.NewEvalKeyResolver(cfg),       // flagship (course step render)
		Catalog:          catalog,
		SpecByID:         specByID,
		Pool:             pool,
		CookieSecure:     cfg.CookieSecure,
		Voice:            voiceSvc,
		CORSOrigins:      cfg.CORSOrigins,
		Fetcher:          materialize.NewFetcher(),
		OSS:              ossSvc,
		OSSAdminKey:      cfg.OSSAdminKey,
	}).Handler()

	srv := httpx.NewServer(cfg, pool, apiHandler)

	if err := httpx.RunServer(srv, func(shutdownCtx context.Context) {
		pool.Close()
	}); err != nil {
		fmt.Fprintf(os.Stderr, "server: %v\n", err)
		os.Exit(1)
	}
}
