import type { LlmConfig, ChatRequest, ChatResult } from "./types";
import { LlmError } from "./LlmError";

export async function anthropicAdapter(config: LlmConfig, req: ChatRequest): Promise<ChatResult> {
  const system = req.messages.filter((m) => m.role === "system").map((m) => m.content).join("\n\n");
  const messages = req.messages.filter((m) => m.role !== "system");
  let res: Response;
  try {
    res = await fetch(`${config.baseUrl}/messages`, {
      method: "POST",
      headers: {
        "x-api-key": config.apiKey,
        "anthropic-version": "2023-06-01",
        "anthropic-dangerous-direct-browser-access": "true",
        "Content-Type": "application/json",
      },
      body: JSON.stringify({
        model: config.model,
        max_tokens: req.maxTokens ?? 1024,
        ...(system ? { system } : {}),
        ...(req.temperature !== undefined ? { temperature: req.temperature } : {}),
        messages,
      }),
    });
  } catch {
    throw new LlmError("网络请求失败（检查 baseUrl / CORS）", { provider: "anthropic" });
  }
  const data = (await res.json().catch(() => ({}))) as {
    content?: { type?: string; text?: string }[];
    usage?: { input_tokens?: number; output_tokens?: number };
    error?: { message?: string };
  };
  if (!res.ok) {
    throw new LlmError(data.error?.message ?? `Anthropic 接口返回 ${res.status}`, { status: res.status, provider: "anthropic", body: data });
  }
  const textBlock = data.content?.find((b) => b.type === "text");
  return {
    text: textBlock?.text ?? "",
    usage: { inputTokens: data.usage?.input_tokens, outputTokens: data.usage?.output_tokens },
    raw: data,
  };
}
