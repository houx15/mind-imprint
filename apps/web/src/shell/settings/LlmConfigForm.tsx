import { useState } from "react";
import { loadConfig, saveConfig, markVerified, isConfigured } from "../../llm";
import type { LlmConfig, ChatRequest, ChatResult } from "../../llm/types";

type ChatFn = (config: Partial<LlmConfig>, req: ChatRequest) => Promise<ChatResult>;

const inputStyle: React.CSSProperties = {
  border: "1px solid #E1E4ED",
  borderRadius: 11,
  padding: "11px 13px",
  fontSize: 14,
  background: "#FCFCFD",
  width: "100%",
  boxSizing: "border-box",
  outline: "none",
  fontFamily: "inherit",
};

const labelStyle: React.CSSProperties = {
  display: "flex",
  flexDirection: "column",
  gap: 6,
  fontSize: 13,
  color: "#4B5168",
  fontWeight: 500,
};

type TestState = "idle" | "testing" | "ok" | "fail";

export function LlmConfigForm({
  chat: chatFn,
  onVerified,
}: {
  chat?: ChatFn;
  onVerified: () => void;
}) {
  const initial = loadConfig();

  const [format, setFormat] = useState<string>(initial.format ?? "openai");
  const [baseUrl, setBaseUrl] = useState(initial.baseUrl ?? "");
  const [model, setModel] = useState(initial.model ?? "");
  const [evalModel, setEvalModel] = useState(initial.evalModel ?? "");
  const [apiKey, setApiKey] = useState(initial.apiKey ?? "");
  const [testState, setTestState] = useState<TestState>("idle");
  const [testError, setTestError] = useState("");
  const [incompleteMsg, setIncompleteMsg] = useState("");

  // Resolve the chat function — default to the real `chat` lazily to avoid importing
  // it at module level (which can cause issues in tests). The prop always takes priority.
  async function resolveChat(): Promise<ChatFn> {
    if (chatFn) return chatFn;
    const { chat } = await import("../../llm");
    return chat;
  }

  function resetTestState() {
    setTestState("idle");
    setTestError("");
    setIncompleteMsg("");
  }

  async function handleTest() {
    const cfg: Partial<LlmConfig> = {
      format: format as LlmConfig["format"],
      baseUrl,
      model,
      evalModel: evalModel || undefined,
      apiKey,
    };

    if (!isConfigured(cfg)) {
      setIncompleteMsg("请先填写完整配置");
      return;
    }

    setIncompleteMsg("");
    saveConfig(cfg);
    setTestState("testing");

    try {
      const fn = await resolveChat();
      // A real generation check, not just reachability. maxTokens must be large
      // enough for a reasoning model to finish its hidden reasoning AND emit a
      // visible token — a tiny cap (e.g. 4) returns empty content on those
      // models, which would pass a "didn't throw" check while proving nothing.
      const r = await fn(cfg, { messages: [{ role: "user", content: "回复一个字：好" }], maxTokens: 256 });
      if (!r.text.trim() && !(r.toolCalls && r.toolCalls.length > 0)) {
        throw new Error("已连通，但模型没有返回任何内容（请确认模型名是否正确）");
      }
      markVerified(cfg);
      setTestState("ok");
      onVerified();
    } catch (e) {
      setTestState("fail");
      // Never include the API key in the rendered error text
      const rawMsg = e instanceof Error ? e.message : String(e);
      // Sanitize: strip the api key if somehow present
      const safeMsg = apiKey ? rawMsg.split(apiKey).join("[key]") : rawMsg;
      setTestError(safeMsg);
    }
  }

  return (
    <div
      style={{
        background: "#fff",
        border: "1px solid #E1E4ED",
        borderRadius: 14,
        padding: "24px 28px",
        display: "flex",
        flexDirection: "column",
        gap: 18,
        maxWidth: 520,
      }}
    >
      {/* Provider format */}
      <label style={labelStyle}>
        <span>格式</span>
        <select
          value={format}
          onChange={(e) => { setFormat(e.target.value); resetTestState(); }}
          style={{ ...inputStyle, cursor: "pointer" }}
        >
          <option value="openai">OpenAI-compatible</option>
          <option value="anthropic">Anthropic</option>
        </select>
      </label>

      {/* Base URL */}
      <label style={labelStyle}>
        <span>Base URL</span>
        <input
          id="base-url"
          type="text"
          value={baseUrl}
          onChange={(e) => { setBaseUrl(e.target.value); resetTestState(); }}
          placeholder="https://api.openai.com/v1"
          style={inputStyle}
        />
      </label>

      {/* Model */}
      <label style={labelStyle}>
        <span>模型</span>
        <input
          id="model"
          type="text"
          value={model}
          onChange={(e) => { setModel(e.target.value); resetTestState(); }}
          placeholder="gpt-4o"
          style={inputStyle}
        />
      </label>

      {/* Eval model (optional) */}
      <label style={labelStyle}>
        <span>评估模型（可选）</span>
        <input
          id="eval-model"
          type="text"
          value={evalModel}
          onChange={(e) => { setEvalModel(e.target.value); resetTestState(); }}
          placeholder="gpt-4o（留空与模型相同）"
          style={inputStyle}
        />
      </label>

      {/* API Key */}
      <label style={labelStyle}>
        <span>API Key</span>
        <input
          id="api-key"
          type="password"
          value={apiKey}
          onChange={(e) => { setApiKey(e.target.value); resetTestState(); }}
          placeholder="sk-..."
          style={inputStyle}
          autoComplete="new-password"
        />
      </label>

      {/* Incomplete config message */}
      {incompleteMsg && (
        <p style={{ margin: 0, fontSize: 13, color: "#E5484D" }}>{incompleteMsg}</p>
      )}

      {/* Test connection button + status */}
      <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
        <button
          type="button"
          onClick={handleTest}
          disabled={testState === "testing"}
          style={{
            background: testState === "ok" ? "#22C55E" : "#3B5BDB",
            color: "#fff",
            border: "none",
            borderRadius: 9,
            padding: "10px 20px",
            fontSize: 14,
            fontWeight: 600,
            cursor: testState === "testing" ? "not-allowed" : "pointer",
            opacity: testState === "testing" ? 0.7 : 1,
            fontFamily: "inherit",
          }}
        >
          {testState === "testing" ? "连接中…" : "测试连接"}
        </button>

        {testState === "ok" && (
          <span style={{ color: "#22C55E", fontSize: 13, fontWeight: 500 }}>✓ 连接成功</span>
        )}
      </div>

      {/* Failure message */}
      {testState === "fail" && (
        <div
          style={{
            background: "#FFF0F0",
            border: "1px solid #F9C0C0",
            borderRadius: 9,
            padding: "10px 14px",
            fontSize: 13,
            color: "#E5484D",
          }}
        >
          <span style={{ fontWeight: 600 }}>连接失败</span>
          {testError && <span>：{testError}</span>}
        </div>
      )}
    </div>
  );
}
