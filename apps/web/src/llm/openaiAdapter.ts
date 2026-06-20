import type { LlmConfig, ChatRequest, ChatResult } from "./types";
import { LlmError } from "./LlmError";

export async function openaiAdapter(config: LlmConfig, req: ChatRequest): Promise<ChatResult> {
  let res: Response;
  try {
    res = await fetch(`${config.baseUrl}/chat/completions`, {
      method: "POST",
      headers: {
        "Authorization": `Bearer ${config.apiKey}`,
        "Content-Type": "application/json",
      },
      body: JSON.stringify({
        model: config.model,
        messages: req.messages,
        max_tokens: req.maxTokens ?? 1024,
        ...(req.temperature !== undefined ? { temperature: req.temperature } : {}),
      }),
    });
  } catch {
    throw new LlmError("网络请求失败（检查 baseUrl / CORS）", { provider: "openai" });
  }
  const data = (await res.json().catch(() => ({}))) as {
    choices?: { message?: { content?: string } }[];
    usage?: { prompt_tokens?: number; completion_tokens?: number };
    error?: { message?: string };
  };
  if (!res.ok) {
    throw new LlmError(data.error?.message ?? `OpenAI 接口返回 ${res.status}`, { status: res.status, provider: "openai", body: data });
  }
  return {
    text: data.choices?.[0]?.message?.content ?? "",
    usage: { inputTokens: data.usage?.prompt_tokens, outputTokens: data.usage?.completion_tokens },
    raw: data,
  };
}
