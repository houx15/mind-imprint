import { z } from "zod";
import type { LlmConfig, LlmFormat } from "./types";

const STORAGE_KEY = "mk.llmConfig";
type EnvSource = Record<string, string | undefined>;

const StoredConfig = z.object({
  format: z.enum(["openai", "anthropic"]).optional(),
  baseUrl: z.string().optional(),
  model: z.string().optional(),
  apiKey: z.string().optional(),
  evalModel: z.string().optional(),
  verified: z.boolean().optional(),
});

export function loadConfig(env: EnvSource = import.meta.env as EnvSource): Partial<LlmConfig> {
  const stored = typeof localStorage !== "undefined" ? localStorage.getItem(STORAGE_KEY) : null;
  if (stored) {
    try {
      const parsed = StoredConfig.safeParse(JSON.parse(stored));
      if (parsed.success) return parsed.data;
    } catch {
      /* corrupt storage — fall through to env defaults */
    }
  }
  const fmt = env.VITE_LLM_FORMAT;
  return {
    format: fmt === "anthropic" || fmt === "openai" ? (fmt as LlmFormat) : undefined,
    baseUrl: env.VITE_LLM_BASE_URL,
    model: env.VITE_LLM_MODEL,
    apiKey: env.VITE_LLM_API_KEY,
    evalModel: env.VITE_LLM_EVAL_MODEL || undefined,
  };
}

export function saveConfig(cfg: LlmConfig): void {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(cfg));
}

export function isConfigured(cfg: Partial<LlmConfig>): cfg is LlmConfig {
  return Boolean(cfg.format && cfg.baseUrl && cfg.model && cfg.apiKey);
}

export function isVerified(cfg: Partial<LlmConfig>): boolean {
  return isConfigured(cfg) && cfg.verified === true;
}

export function markVerified(cfg: LlmConfig): void {
  saveConfig({ ...cfg, verified: true });
}
