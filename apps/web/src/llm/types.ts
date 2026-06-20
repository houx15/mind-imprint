export type LlmFormat = "openai" | "anthropic";

export interface LlmConfig {
  format: LlmFormat;
  baseUrl: string; // API root incl. /v1, e.g. https://api.openai.com/v1 | https://api.anthropic.com/v1
  model: string;
  apiKey: string;
  evalModel?: string; // optional; consumed by B6/S5, only stored here
}

export type ChatRole = "system" | "user" | "assistant";
export interface ChatMessage { role: ChatRole; content: string }

export interface ChatRequest {
  messages: ChatMessage[];
  maxTokens?: number;
  temperature?: number;
  // tools?: reserved for Slice 3 (summon_card) — not implemented in Slice 2.
}

export interface ChatUsage { inputTokens?: number; outputTokens?: number }
export interface ChatResult { text: string; usage?: ChatUsage; raw?: unknown }
