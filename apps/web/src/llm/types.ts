export type LlmFormat = "openai" | "anthropic";

export interface LlmConfig {
  format: LlmFormat;
  baseUrl: string; // API root incl. /v1, e.g. https://api.openai.com/v1 | https://api.anthropic.com/v1
  model: string;
  apiKey: string;
  evalModel?: string; // optional; consumed by B6/S5, only stored here
  verified?: boolean; // set true only after a passing 测试连接; gates the key-gate
}

export type ChatRole = "system" | "user" | "assistant" | "tool";
export interface ChatTool { name: string; description: string; parameters: Record<string, unknown> }
export interface ToolCall { id: string; name: string; args: Record<string, unknown> }
export interface ChatMessage { role: ChatRole; content: string; toolCalls?: ToolCall[]; toolCallId?: string }

export interface ChatRequest {
  messages: ChatMessage[];
  tools?: ChatTool[];
  maxTokens?: number;
  temperature?: number;
}

export type StopReason = "stop" | "tool_call" | "length" | "other";
export interface ChatUsage { inputTokens?: number; outputTokens?: number }
export interface ChatResult { text: string; toolCalls?: ToolCall[]; stopReason?: StopReason; usage?: ChatUsage; raw?: unknown }
