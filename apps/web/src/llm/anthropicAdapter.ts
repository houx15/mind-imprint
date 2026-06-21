import type { LlmConfig, ChatRequest, ChatResult, ChatMessage, StopReason, ToolCall } from "./types";
import { LlmError } from "./LlmError";

function serializeMessage(m: ChatMessage): Record<string, unknown> {
  if (m.role === "tool") {
    return {
      role: "user",
      content: [{ type: "tool_result", tool_use_id: m.toolCallId, content: m.content }],
    };
  }
  if (m.role === "assistant" && m.toolCalls?.length) {
    return {
      role: "assistant",
      content: [
        ...(m.content ? [{ type: "text", text: m.content }] : []),
        ...m.toolCalls.map(c => ({ type: "tool_use", id: c.id, name: c.name, input: c.args })),
      ],
    };
  }
  return { role: m.role, content: m.content };
}

function mapStopReason(reason: string | undefined): StopReason | undefined {
  if (reason === "tool_use") return "tool_call";
  if (reason === "end_turn") return "stop";
  if (reason === "max_tokens") return "length";
  if (reason === undefined) return undefined;
  return "other";
}

export async function anthropicAdapter(config: LlmConfig, req: ChatRequest): Promise<ChatResult> {
  const system = req.messages.filter((m) => m.role === "system").map((m) => m.content).join("\n\n");
  const messages = req.messages.filter((m) => m.role !== "system").map(serializeMessage);
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
        ...(req.tools?.length
          ? { tools: req.tools.map(t => ({ name: t.name, description: t.description, input_schema: t.parameters })) }
          : {}),
        messages,
      }),
    });
  } catch {
    throw new LlmError("网络请求失败（检查 baseUrl / CORS）", { provider: "anthropic" });
  }
  const data = (await res.json().catch(() => ({}))) as {
    content?: { type?: string; text?: string; id?: string; name?: string; input?: Record<string, unknown> }[];
    stop_reason?: string;
    usage?: { input_tokens?: number; output_tokens?: number };
    error?: { message?: string };
  };
  if (!res.ok) {
    throw new LlmError(data.error?.message ?? `Anthropic 接口返回 ${res.status}`, { status: res.status, provider: "anthropic", body: data });
  }
  const textParts: string[] = [];
  const toolCalls: ToolCall[] = [];
  for (const block of data.content ?? []) {
    if (block.type === "text" && block.text !== undefined) {
      textParts.push(block.text);
    } else if (block.type === "tool_use" && block.id && block.name) {
      toolCalls.push({ id: block.id, name: block.name, args: block.input ?? {} });
    }
  }
  const stopReason = mapStopReason(data.stop_reason);
  return {
    text: textParts.join("\n"),
    ...(toolCalls.length ? { toolCalls } : {}),
    ...(stopReason !== undefined ? { stopReason } : {}),
    usage: { inputTokens: data.usage?.input_tokens, outputTokens: data.usage?.output_tokens },
    raw: data,
  };
}
