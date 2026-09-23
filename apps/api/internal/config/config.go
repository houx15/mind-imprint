// Package config loads typed service configuration from the environment.
//
// 12-factor: configuration lives in env vars. A local .env.local is loaded as a
// developer convenience only and never committed. Missing required values are a
// fatal, fail-fast error at boot.
package config

import (
	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

// Config is the fully-resolved service configuration.
type Config struct {
	// Port is the HTTP listen port; defaults to 8080.
	Port string `env:"PORT" envDefault:"8080"`
	// DatabaseURL is the Postgres DSN. Required.
	DatabaseURL string `env:"DATABASE_URL,required"`
	// CORSOrigins is the comma-separated allowlist of SPA origins.
	CORSOrigins []string `env:"CORS_ORIGINS" envSeparator:","`
	// AnthropicKey is the Anthropic provider key (server-side only).
	AnthropicKey string `env:"ANTHROPIC_API_KEY"`
	// DeepSeekKey is the DeepSeek provider key (server-side only).
	DeepSeekKey string `env:"DEEPSEEK_API_KEY"`
	// ZAIKey is the Zhipu AI / GLM provider key (server-side only). It is
	// intentionally not used by the default resolvers.
	ZAIKey string `env:"ZAI_API_KEY"`
	// DashScopeKey is the Aliyun DashScope / Bailian key (server-side only).
	// DashScope is an aggregator: one key reaches Qwen, DeepSeek, GLM and Kimi —
	// which is what makes swapping a model for an ability/speed/cost comparison a
	// one-line change.
	DashScopeKey string `env:"DASHSCOPE_API_KEY"`

	// ModelChat / ModelFastChat / ModelEval override a lane's catalog binding by
	// naming a model id from gateway/models.json (e.g. "dashscope/qwen3.8-max"). Empty
	// keeps the catalog default. Each lane is independent, so one model can be
	// swapped and measured while the others hold still. An unknown id — or a
	// non-flagship model on ModelEval — fails at boot, not mid-session.
	//
	// Run `api --print-models` to see the catalog and what each lane resolves to.
	ModelChat     string `env:"MODEL_CHAT"`
	ModelFastChat string `env:"MODEL_FAST_CHAT"`
	ModelEval     string `env:"MODEL_EVAL"`

	// ModelClass overrides a CAPABILITY CLASS binding: MODEL_REFLEX,
	// MODEL_DIALOGUE, MODEL_COMPOSE, MODEL_REVIEW, MODEL_ASSESS, MODEL_DIGEST.
	// Keyed by class name. Left nil outside the real binary; NewResolvers reads
	// the environment itself when a key is absent, so a test can set a class
	// override without touching this map.
	//
	// A class override beats the legacy lane variable it replaced, so a
	// half-migrated environment resolves to the more specific instruction rather
	// than to whichever happened to be read last.
	ModelClass map[string]string `env:"-"`
	// CookieSecure sets the Secure flag on the session cookie. Default true;
	// set COOKIE_SECURE=false for local http dev so the browser sends it.
	CookieSecure bool `env:"COOKIE_SECURE" envDefault:"true"`

	// Voice (Volcano Engine) credentials and resource ids — server-side
	// only. Left empty in dev/test to keep the voice feature optional; the
	// platform must still boot when unset (Deps.Voice stays nil).
	VoiceAppID       string `env:"VOICE_APP_ID"`
	VoiceAccessKey   string `env:"VOICE_ACCESS_KEY"`
	VoiceTTSVoice    string `env:"VOICE_TTS_VOICE"`
	VoiceTTSResource string `env:"VOICE_TTS_RESOURCE_ID" envDefault:"seed-tts-2.0"`
	VoiceASRResource string `env:"VOICE_ASR_RESOURCE_ID" envDefault:"volc.bigasr.sauc.duration"`

	// OSS (Aliyun object storage) — server-side only. Left empty in dev/test to
	// keep file storage optional; the platform must still boot when unset
	// (Deps.OSS stays nil and the /oss/* routes return 503).
	OSSEndpoint     string `env:"OSS_ENDPOINT"`          // mind-imprint.oss-cn-beijing.aliyuncs.com
	OSSBucket       string `env:"OSS_BUCKET"`            // mind-imprint
	OSSCDNDomain    string `env:"OSS_CDN_DOMAIN"`        // mind-oss.uni-robot.cn
	OSSAccessKeyID  string `env:"OSS_ACCESS_KEY_ID"`     // server-side only
	OSSAccessSecret string `env:"OSS_ACCESS_KEY_SECRET"` // server-side only
	// OSSAdminKey is a static bearer secret authorizing the admin upload routes
	// (course/asset management from backend scripts). Empty disables those
	// routes (503) so no request can authenticate against a blank key.
	OSSAdminKey string `env:"OSS_ADMIN_KEY"`
	// OSSCDNAuthKey is the Aliyun CDN URL鉴权 Type A 主KEY. When set, download
	// URLs are signed as CDN URL鉴权 links (cacheable, auth_key excluded from the
	// cache key) instead of OSS presigned URLs. Empty ⇒ presigned fallback.
	OSSCDNAuthKey string `env:"OSS_CDN_AUTH_KEY"`
	// OSSCDNAuthWindow is the URL鉴权 validity window in seconds; it must mirror
	// the console 验证时长. Used to report expiresAt / schedule client refresh.
	OSSCDNAuthWindow           int    `env:"OSS_CDN_AUTH_WINDOW" envDefault:"7200"`
	PublicAssetOSSEndpoint     string `env:"PUBLIC_ASSET_OSS_ENDPOINT"`
	PublicAssetOSSBucket       string `env:"PUBLIC_ASSET_OSS_BUCKET"`
	PublicAssetCDNDomain       string `env:"PUBLIC_ASSET_CDN_DOMAIN"`
	PublicAssetOSSAccessKeyID  string `env:"PUBLIC_ASSET_OSS_ACCESS_KEY_ID"`
	PublicAssetOSSAccessSecret string `env:"PUBLIC_ASSET_OSS_ACCESS_KEY_SECRET"`
}

// Load reads .env.local if present (ignored if absent), then parses the
// environment into a Config. Returns an error if a required value is missing.
func Load() (Config, error) {
	// Best-effort local file; absence is not an error.
	_ = godotenv.Load(".env.local")
	return env.ParseAs[Config]()
}
