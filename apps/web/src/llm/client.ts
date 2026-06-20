import type { LlmConfig, ChatRequest, ChatResult } from "./types";
import { isConfigured } from "./config";
import { LlmError } from "./LlmError";
import { openaiAdapter } from "./openaiAdapter";
import { anthropicAdapter } from "./anthropicAdapter";

export async function chat(config: Partial<LlmConfig>, req: ChatRequest): Promise<ChatResult> {
  if (!isConfigured(config)) {
    throw new LlmError("LLM 未配置（缺 format / baseUrl / model / apiKey）");
  }
  return config.format === "anthropic" ? anthropicAdapter(config, req) : openaiAdapter(config, req);
}
