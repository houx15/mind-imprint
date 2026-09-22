package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"

	"github.com/joho/godotenv"

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

// modelEnvOnlyConfig builds the subset of config that model routing reads,
// straight from the environment. It exists so `--print-models` answers "which
// model would serve each lane?" on a laptop with no DATABASE_URL set.
func modelEnvOnlyConfig() config.Config {
	_ = godotenv.Load(".env.local")
	return config.Config{
		DashScopeKey:  os.Getenv("DASHSCOPE_API_KEY"),
		DeepSeekKey:   os.Getenv("DEEPSEEK_API_KEY"),
		AnthropicKey:  os.Getenv("ANTHROPIC_API_KEY"),
		ZAIKey:        os.Getenv("ZAI_API_KEY"),
		ModelChat:     os.Getenv("MODEL_CHAT"),
		ModelFastChat: os.Getenv("MODEL_FAST_CHAT"),
		ModelEval:     os.Getenv("MODEL_EVAL"),
	}
}

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	migrateUp := flag.Bool("migrate-up", false, "run database migrations then exit")
	printModels := flag.Bool("print-models", false, "print the model catalog and the active lane bindings, then exit")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		// --print-models is a diagnostic about model routing, so it must work
		// without a database. Fall back to the env-only fields it actually reads.
		if *printModels {
			fmt.Print(gateway.DescribeBindings(modelEnvOnlyConfig()))
			return
		}
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}

	if *printModels {
		fmt.Print(gateway.DescribeBindings(cfg))
		return
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
	publicAssets, err := oss.NewPublicAssets(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "public assets: %v\n", err)
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
	// Dispatch is by wire protocol (Resolved.Kind), so every OpenAI-compatible
	// vendor in the catalog shares one adapter. The vendor-named entries remain
	// for pre-catalog Resolved values that carry no Kind.
	provider := gateway.NewMuxProvider(map[string]gateway.Provider{
		gateway.KindOpenAICompatible: gateway.NewCatalogProvider(httpClient),
		gateway.KindAnthropic:        gateway.NewAnthropicProvider(httpClient),
		"deepseek":                   gateway.NewDeepSeekProvider(httpClient),
		"glm":                        gateway.NewGLMProvider(httpClient),
	})

	// A bad MODEL_* override or a malformed catalog fails here, at boot, rather
	// than at a student's first turn.
	resolvers, err := gateway.NewResolvers(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "model catalog: %v\n", err)
		pool.Close()
		os.Exit(1)
	}
	// The boot log is the only place that says which model each capability class
	// actually landed on. 2026-09-02: a key that never reached the server made
	// production run the fallback provider for hours while every other signal —
	// including a green "DEPLOY OK" — said otherwise. This line was what caught it.
	for _, class := range gateway.Classes {
		b, ok := resolvers.Bindings[class]
		if !ok {
			log.Printf("class %-11s → UNRESOLVED (no provider key configured)", class)
			continue
		}
		log.Printf("class %-11s → %s (provider=%s tier=%s think=%s)", class, b.ModelID, b.Provider, b.Tier, b.Reasoning)
	}

	theAPI := api.New(api.Deps{
		Queries:          queries,
		Provider:         provider,
		Route:            resolvers.For,
		ChatResolver:     resolvers.Chat,     // legacy alias → dialogue
		FastChatResolver: resolvers.FastChat, // legacy alias → reflex
		EvalResolver:     resolvers.Eval,     // legacy alias → assess
		Catalog:          catalog,
		SpecByID:         specByID,
		Pool:             pool,
		CookieSecure:     cfg.CookieSecure,
		Voice:            voiceSvc,
		CORSOrigins:      cfg.CORSOrigins,
		Fetcher:          materialize.NewFetcher(),
		OSS:              ossSvc,
		PublicAssets:     publicAssets,
		OSSAdminKey:      cfg.OSSAdminKey,
	})
	apiHandler := theAPI.Handler()

	// 后台任务队列：兴趣采集与 lite AI 批改共用这一个 river client。**起不来就
	// 只 log**：队列是可降级的子系统，不是正确性不变量；为它拒绝启动会让整个接口
	// 下线。EnqueueHarvest 对 nil client 是一次安静的空操作，扫尾也就不跑；批改的
	// 入队接口返回 503 grading_queue_unavailable，其余照常。
	riverClient, rerr := api.StartHarvestQueue(ctx, pool, theAPI)
	if rerr != nil {
		slog.Error("job queue failed to start; interest harvesting and AI grading are off", "err", rerr)
	} else {
		theAPI.AttachRiver(riverClient)
		slog.Info("job queue started (interest harvesting, AI grading)")
	}

	srv := httpx.NewServer(cfg, pool, apiHandler)

	if err := httpx.RunServer(srv, func(shutdownCtx context.Context) {
		if riverClient != nil {
			// 先停队列再关连接池：一个还在跑的 worker 会用到 pool。
			if err := riverClient.Stop(shutdownCtx); err != nil {
				slog.Warn("interest harvest queue did not stop cleanly", "err", err)
			}
		}
		pool.Close()
	}); err != nil {
		fmt.Fprintf(os.Stderr, "server: %v\n", err)
		os.Exit(1)
	}
}
