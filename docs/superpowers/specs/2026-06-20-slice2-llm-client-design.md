# Slice 2 设计规格 · Provider 无关 LLM 客户端 + 配置 (B2)

> 版本 v0.1 ｜ 2026-06-20 ｜ 状态：待用户评审
> 上游：`docs/架构分解_Roadmap.md` §0.1 + §1 B2（v0.2，前端直连/BYO-key）、`docs/思维印记_Demo_PRD.md`（产品铁律）
> 下游：本规格通过后 → writing-plans 出实现计划 → TDD 实现

---

## 1. 目标与成功判据

**一句话：** 在前端建一个 **provider 无关**的 LLM 客户端：给定 `{format, baseUrl, model, apiKey}`，把一组对话消息发给 OpenAI 格式或 Anthropic 格式的接口，拿回回复。密钥用户自带、存浏览器、绝不入 git。无后端、非流式。

**完成即满足 PRD §2 成功判据 #4 的地基**：真实 LLM 调用可跑通（后续 slice 的陪练/评估都建在此之上）。

**可演示验收（dev 设置面板内）：**
1. 在设置面板填入 format/baseUrl/model/apiKey（或由 `.env.local` 预填），点「保存」→ 配置存入浏览器。
2. 点「测试连接」→ 真实打到所配置的接口 → 显示 ✅ 模型回复 或 ❌ 清晰错误。
3. 切换 format（openai ↔ anthropic）后，同一 `chat()` 调用正确路由到对应适配器。

---

## 2. 范围

**做（In）：**
- `apps/web/src/llm/`：`LlmConfig`/消息/结果类型、`openaiAdapter`、`anthropicAdapter`、统一 `chat()`、`LlmError`、配置存取（localStorage + `.env.local` 预填）。
- `SettingsPanel`：最小 dev 表单（4 字段 + 可选 evalModel）+「保存」+「测试连接」。
- `apps/web/.env.example`（提交）→ 用户复制为 `.env.local`（git-ignored）填 key。
- dev 外壳：`main.tsx` 渲染带两个 tab 的 `DevApp`（卡片 harness ｜ LLM 设置），把 S1 的 Harness 收进一个 tab。

**不做（Out，留给后续 slice）：**
- ❌ tool / function calling（S3 `summon_card` 时实现；本 slice `ChatRequest` 仅**预留** tools 字段位，不实现）
- ❌ 浏览器持久化（S2.5）
- ❌ 流式（设计上不做）
- ❌ 计量/成本（§0.1 决定不做）
- ❌ 陪练系统 prompt / 决策层 / summon_card 循环（S3）
- ❌ 正式设置页样式（B7；本 slice 只做最小 dev 表单）

---

## 3. 架构

```
SettingsPanel ──saveConfig──▶ localStorage("mk.llmConfig")   ◀──loadConfig── import.meta.env.VITE_LLM_*
                                        │
chat(config, request) ──▶ 按 config.format 选适配器
   │                         ├─ "openai"    → openaiAdapter
   │                         └─ "anthropic" → anthropicAdapter
   │                                   │ fetch(baseUrl+path, {headers, body})
   ▼                                   ▼
ChatResult { text, usage? }  ◀── 解析  ◀── provider 响应 / 非2xx→抛 LlmError
```

全前端，无后端。适配器用浏览器 `fetch` 直连。`llm/` 模块**不依赖** `@mind-imprint/contracts`（消息是通用文本）。

---

## 4. 类型契约（`apps/web/src/llm/types.ts`）

```ts
export type LlmFormat = "openai" | "anthropic";

export interface LlmConfig {
  format: LlmFormat;
  baseUrl: string;   // API 根，含 /v1，例：https://api.openai.com/v1 ｜ https://api.anthropic.com/v1
  model: string;
  apiKey: string;
  evalModel?: string; // 可选；S5 评估用，无则回退 model（本 slice 不消费，只存）
}

export type ChatRole = "system" | "user" | "assistant";
export interface ChatMessage { role: ChatRole; content: string }

export interface ChatRequest {
  messages: ChatMessage[];
  maxTokens?: number;     // 默认 1024
  temperature?: number;
  // tools?: ...          // 预留给 S3，本 slice 不实现
}

export interface ChatUsage { inputTokens?: number; outputTokens?: number }
export interface ChatResult { text: string; usage?: ChatUsage; raw?: unknown }
```

`LlmError`（`apps/web/src/llm/LlmError.ts`）：`class LlmError extends Error { status?: number; provider?: LlmFormat; body?: unknown }`，便于 UI 分辨网络错/鉴权错/接口错。

---

## 5. 适配器

两个适配器同签名：`chat(config: LlmConfig, req: ChatRequest): Promise<ChatResult>`。非 2xx 或 fetch 失败 → 抛 `LlmError`（带 status + provider + 解析出的错误文本）。**绝不把 apiKey 写进日志或错误消息。**

### 5.1 `openaiAdapter`
- URL：`${baseUrl}/chat/completions`
- headers：`Authorization: Bearer ${apiKey}`、`Content-Type: application/json`
- body：`{ model, messages, max_tokens: req.maxTokens ?? 1024, temperature?: req.temperature }`（OpenAI 接受 messages 里直接含 `system` 角色）
- 解析：`text = data.choices[0].message.content`；`usage = { inputTokens: data.usage?.prompt_tokens, outputTokens: data.usage?.completion_tokens }`

### 5.2 `anthropicAdapter`
- URL：`${baseUrl}/messages`
- headers：`x-api-key: ${apiKey}`、`anthropic-version: 2023-06-01`、`anthropic-dangerous-direct-browser-access: true`、`Content-Type: application/json`
- body：`{ model, max_tokens: req.maxTokens ?? 1024, temperature?, system?, messages }`，其中：
  - **抽取** `messages` 里所有 `role:"system"` 的 content 合并成顶层 `system` 字符串；
  - 其余按原序作为 `messages`（角色仅 `user`/`assistant`）。
- 解析：`text =` `data.content` 中第一个 `type:"text"` 块的 `.text`；`usage = { inputTokens: data.usage?.input_tokens, outputTokens: data.usage?.output_tokens }`

### 5.3 `client.ts`
```ts
export async function chat(config: LlmConfig, req: ChatRequest): Promise<ChatResult> {
  if (!isConfigured(config)) throw new LlmError("LLM 未配置（缺 format/baseUrl/model/apiKey）");
  return config.format === "anthropic" ? anthropicAdapter(config, req) : openaiAdapter(config, req);
}
```

---

## 6. 配置存取（`apps/web/src/llm/config.ts`）

```ts
const STORAGE_KEY = "mk.llmConfig";

// env 源可注入，便于测试（默认 import.meta.env）
export function loadConfig(env = import.meta.env): Partial<LlmConfig> { ... }
export function saveConfig(cfg: LlmConfig): void { localStorage.setItem(STORAGE_KEY, JSON.stringify(cfg)); }
export function isConfigured(cfg: Partial<LlmConfig>): cfg is LlmConfig { return !!(cfg.format && cfg.baseUrl && cfg.model && cfg.apiKey); }
```

- `loadConfig`：优先 localStorage；缺失时从 `env` 取 `VITE_LLM_FORMAT/BASE_URL/MODEL/API_KEY/EVAL_MODEL` 拼出默认。
- `env` 参数可注入 → 单测不依赖真实 `import.meta.env`。

### 6.1 `apps/web/.env.example`（提交；用户复制为 `.env.local`）
```
# 复制为 apps/web/.env.local（git-ignored）并填写。VITE_* 会打进前端包 —— 仅本地 BYO-key 开发用。
VITE_LLM_FORMAT=openai            # openai | anthropic
VITE_LLM_BASE_URL=https://api.openai.com/v1
VITE_LLM_MODEL=gpt-4o-mini
VITE_LLM_API_KEY=
VITE_LLM_EVAL_MODEL=             # 可选；空则评估回退到 VITE_LLM_MODEL
```
`.gitignore` 已有 `.env.*` + `!.env.example`：`.env.local` 被忽略，`.env.example` 被跟踪。

---

## 7. SettingsPanel + dev 外壳

- `SettingsPanel.tsx`：format 下拉（openai/anthropic）、baseUrl/model/apiKey/evalModel 输入（apiKey 用 `type="password"`）、「保存」（`saveConfig`）、「测试连接」。
  - 测试连接：用当前表单值组 `LlmConfig`，调 `chat(cfg, { messages: [{role:"user", content:"回复 OK"}], maxTokens: 16 })`；成功显示 ✅ + 回复文本，失败显示 ❌ + `LlmError.message`（不含 key）。
- `DevApp.tsx`：两个 tab —「卡片」(S1 的 `<Harness/>`) ｜「LLM 设置」(`<SettingsPanel/>`)。`main.tsx` 改为渲染 `<DevApp/>`。

---

## 8. 错误处理

- 未配置 → `chat` 抛 `LlmError`，面板提示填写。
- 非 2xx → `LlmError{status, body}`，面板显示 provider 返回的错误文本（如 401 鉴权、404 路径、模型名错）。
- 网络/CORS 失败 → `fetch` reject → 包成 `LlmError`，提示检查 baseUrl/CORS。
- apiKey 绝不进 `console`、错误消息、或 `raw` 展示。

---

## 9. 测试策略（TDD：先红后绿）

**适配器（mock `fetch`，`vi.stubGlobal`）：**
1. `openaiAdapter`：断言 fetch 的 URL=`${baseUrl}/chat/completions`、`Authorization: Bearer`、body `{model,messages,max_tokens}`；解析回 `ChatResult.text` + usage。
2. `openaiAdapter` 错误：非 2xx → 抛 `LlmError`，带 status 与错误文本。
3. `anthropicAdapter`：断言 URL=`${baseUrl}/messages`、headers（`x-api-key`/`anthropic-version`/`dangerous-direct-browser-access`）、**system 抽取**（system 进顶层、messages 只剩 user/assistant）、解析 `content[].text` + usage。
4. `anthropicAdapter` 错误：非 2xx → `LlmError`。
5. **不泄密**：构造一个失败响应，断言抛出的 `LlmError.message` 不含 apiKey 串。

**client / config（jsdom）：**
6. `chat` 按 `format` 路由到正确适配器（用 mock fetch 的 URL 区分）。
7. `chat` 未配置 → 抛 `LlmError`。
8. `config`：`saveConfig` 后 `loadConfig` 读回一致（jsdom localStorage）。
9. `config`：localStorage 空时，`loadConfig(injectedEnv)` 从注入 env 拼出默认；`isConfigured` 对完整/缺字段判定正确。

**SettingsPanel（RTL）：**
10. 填字段 → 点保存 → `loadConfig()` 返回所填值。
11. 点测试连接 → mock 的 `chat` resolve → 显示 ✅ + 回复；reject(LlmError) → 显示 ❌ + 错误文本（不含 key）。

**Live 冒烟（仅有 key 才跑）：**
12. `describe.skipIf(!import.meta.env.VITE_LLM_API_KEY)`：用 `.env.local` 真配置打一次真实 `chat`，断言 `text` 非空。CI / 无 key 时自动跳过。

---

## 10. 验收标准（DoD）

- [ ] §9 测试 1–11 绿；12 在有 key 时绿、无 key 时跳过。
- [ ] `pnpm -r typecheck` + `pnpm -r test` 全绿。
- [ ] dev 外壳跑起来：在「LLM 设置」tab 配置并「测试连接」拿到真实回复（openai 与 anthropic 各验一次，若用户有对应 key）。
- [ ] `apps/web/.env.example` 已提交；`.env.local` 不在 git 里（`git check-ignore apps/web/.env.local` 命中）。
- [ ] 全代码无处打印/拼接 apiKey 到日志或错误。

---

## 11. 风险与边界

- **CORS**：openai 兼容端点与 anthropic（带 browser-access 头）一般允许浏览器直连；若用户的 baseUrl 不允许 CORS，测试连接会失败并提示——这是 BYO 配置问题，非本模块缺陷。文档在 `.env.example` 注明。
- **`import.meta.env` 难 mock**：故 `loadConfig` 接受可注入 `env`；适配器/`chat` 不读 env，只吃显式 `config`。
- **tools 预留不实现**：`ChatRequest` 注释保留 tools 位；S3 落地 function-calling 时两适配器各加映射，届时 `ChatResult` 增加 `toolCalls`。**本 slice 不碰**，避免过早抽象。
- **密钥面**：localStorage 明文存 key 是 BYO-demo 的有意取舍（§0.1）；仅本地单用户演示可接受。
