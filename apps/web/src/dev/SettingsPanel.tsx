import { useState } from "react";
import type { LlmConfig, LlmFormat } from "../llm/types";
import { loadConfig, saveConfig } from "../llm/config";
import { chat } from "../llm/client";
import { LlmError } from "../llm/LlmError";

type Status = { kind: "idle" | "testing" | "ok" | "err"; text?: string };

const inputCls = "mt-1 w-full rounded-[10px] border border-mk-input bg-mk-input-bg px-3 py-2.5 text-sm text-mk-ink outline-none";
const labelCls = "block text-[13px] font-semibold text-[#3A4256] mt-3";

export function SettingsPanel() {
  const initial = loadConfig();
  const [format, setFormat] = useState<LlmFormat>(initial.format ?? "openai");
  const [baseUrl, setBaseUrl] = useState(initial.baseUrl ?? "");
  const [model, setModel] = useState(initial.model ?? "");
  const [apiKey, setApiKey] = useState(initial.apiKey ?? "");
  const [evalModel, setEvalModel] = useState(initial.evalModel ?? "");
  const [status, setStatus] = useState<Status>({ kind: "idle" });

  const current = (): LlmConfig => ({ format, baseUrl, model, apiKey, evalModel: evalModel || undefined });

  const onSave = () => { saveConfig(current()); setStatus({ kind: "idle", text: "已保存" }); };
  const onTest = async () => {
    setStatus({ kind: "testing", text: "测试中…" });
    try {
      const r = await chat(current(), { messages: [{ role: "user", content: "回复 OK" }], maxTokens: 16 });
      setStatus({ kind: "ok", text: r.text || "(空回复)" });
    } catch (e) {
      setStatus({ kind: "err", text: e instanceof LlmError ? e.message : "未知错误" });
    }
  };

  return (
    <div className="mx-auto max-w-[560px] p-8 font-sans text-mk-ink">
      <h2 className="text-lg font-bold">LLM 设置（BYO-key）</h2>
      <p className="mt-1 text-[13px] text-mk-muted-2">密钥只存在你的浏览器，不上传、不入库。也可用 apps/web/.env.local 预填。</p>

      <label className={labelCls}>格式
        <select aria-label="格式" value={format} onChange={(e) => setFormat(e.target.value as LlmFormat)} className={inputCls}>
          <option value="openai">openai</option>
          <option value="anthropic">anthropic</option>
        </select>
      </label>
      <label className={labelCls}>Base URL
        <input aria-label="Base URL" value={baseUrl} onChange={(e) => setBaseUrl(e.target.value)} placeholder="https://api.openai.com/v1" className={inputCls} />
      </label>
      <label className={labelCls}>Model
        <input aria-label="Model" value={model} onChange={(e) => setModel(e.target.value)} className={inputCls} />
      </label>
      <label className={labelCls}>API Key
        <input aria-label="API Key" type="password" value={apiKey} onChange={(e) => setApiKey(e.target.value)} className={inputCls} />
      </label>
      <label className={labelCls}>Eval Model（可选）
        <input aria-label="Eval Model" value={evalModel} onChange={(e) => setEvalModel(e.target.value)} className={inputCls} />
      </label>

      <div className="mt-5 flex items-center gap-3">
        <button type="button" onClick={onSave} className="rounded-[11px] bg-mk-primary px-5 py-2.5 text-sm font-bold text-white">保存</button>
        <button type="button" onClick={onTest} className="rounded-[11px] bg-mk-accent px-5 py-2.5 text-sm font-bold text-white">测试连接</button>
        {status.text && (
          <span className={`text-[13px] font-semibold ${status.kind === "err" ? "text-[#C0552F]" : status.kind === "ok" ? "text-mk-green" : "text-mk-muted-2"}`}>
            {status.kind === "ok" ? "✅ " : status.kind === "err" ? "❌ " : ""}{status.text}
          </span>
        )}
      </div>
    </div>
  );
}
