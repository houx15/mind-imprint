import type { LlmConfig, ChatRequest, ChatResult, ChatMessage, StopReason, ToolCall } from "./types";
import { LlmError } from "./LlmError";

function serializeMessage(m: ChatMessage): Record<string, unknown> {
  if (m.role === "tool") {
    return { role: "tool", tool_call_id: m.toolCallId, content: m.content };
  }
  if (m.role === "assistant" && m.toolCalls?.length) {
    return {
      role: "assistant",
      content: m.content || null,
      tool_calls: m.toolCalls.map(c => ({
        id: c.id,
        type: "function",
        function: { name: c.name, arguments: JSON.stringify(c.args) },
      })),
    };
  }
  return { role: m.role, content: m.content };
}

function mapFinishReason(reason: string | undefined): StopReason | undefined {
  if (reason === "tool_calls") return "tool_call";
  if (reason === "stop") return "stop";
  if (reason === "length") return "length";
  if (reason === undefined) return undefined;
  return "other";
}

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
        messages: req.messages.map(serializeMessage),
        max_tokens: req.maxTokens ?? 1024,
        ...(req.temperature !== undefined ? { temperature: req.temperature } : {}),
        ...(req.tools?.length
          ? {
              tools: req.tools.map(t => ({ type: "function", function: { name: t.name, description: t.description, parameters: t.parameters } })),
              tool_choice: "auto",
            }
          : {}),
      }),
    });
  } catch {
    throw new LlmError("网络请求失败（检查 baseUrl / CORS）", { provider: "openai" });
  }
  const data = (await res.json().catch(() => ({}))) as {
    choices?: {
      message?: {
        content?: string | null;
        tool_calls?: { id: string; type: string; function: { name: string; arguments: string } }[];
      };
      finish_reason?: string;
    }[];
    usage?: { prompt_tokens?: number; completion_tokens?: number };
    error?: { message?: string };
  };
  if (!res.ok) {
    throw new LlmError(data.error?.message ?? `OpenAI 接口返回 ${res.status}`, { status: res.status, provider: "openai", body: data });
  }
  const choice = data.choices?.[0];
  const rawToolCalls = choice?.message?.tool_calls;
  const toolCalls: ToolCall[] | undefined = rawToolCalls?.map(c => {
    let args: Record<string, unknown>;
    try {
      args = JSON.parse(c.function.arguments) as Record<string, unknown>;
    } catch {
      throw new LlmError("模型返回的工具参数无法解析", { provider: "openai" });
    }
    return { id: c.id, name: c.function.name, args };
  });
  return {
    text: choice?.message?.content ?? "",
    ...(toolCalls?.length ? { toolCalls } : {}),
    ...(choice?.finish_reason !== undefined ? { stopReason: mapFinishReason(choice.finish_reason) } : {}),
    usage: { inputTokens: data.usage?.prompt_tokens, outputTokens: data.usage?.completion_tokens },
    raw: data,
  };
}
