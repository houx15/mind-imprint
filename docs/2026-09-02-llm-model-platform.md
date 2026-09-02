# LLM 模型平台：换模型 = 改一个环境变量

2026-09-02

## 为什么改

在此之前，「哪条通道跑哪个模型」被写死在三处 Go 代码里，而且三处会各自漂移：

| 位置 | 写死了什么 |
|---|---|
| `internal/gateway/keyresolver.go` | 三个 resolver，各自一段 `switch`，全部返回 `deepseek-v4-pro` |
| `internal/evalbench/runtime.go` | 第二份 provider → baseURL / key 的映射 |
| `internal/gateway/pricing.go` | 第三份表：`provider/model` → 价格 |

结果是：换一个模型要改 Go、重新构建、重新部署；而且**离线评测跑的模型和线上服务的模型没有任何机制保证一致**——两张表分开维护，漂移只是时间问题。要横向比「能力 / 速度 / 成本」，这个结构本身就是障碍。

## 现在的结构

`apps/api/internal/gateway/models.json`（`go:embed` 进二进制）是单一真相源，三层：

- **providers** —— 一条通道：`kind`（线上协议）、`baseUrl`、**key 的环境变量名**（密钥本身永远不进这个文件）、关思考用哪个字段、默认 body 片段。
- **models** —— 一个可绑定的模型：属于哪个 provider、上线用的模型名、`flagship`（能否服务评估）、`capabilities`、`priceUsd`。
- **能力档（catalog 里仍叫 `lanes`）** —— 一次调用**需要多少智力**：`reflex`（一个标签）、
  `dialogue`（学生当场看得见的一轮）、`compose`（从已陈述的输入派生 schema 产物）、
  `review`（判学生的成果）、`assess`（过程评估，绝不降级）、`digest`（长输入短输出），
  外加预留的 `search` / `multimodal`。每一档带着自己的推理要求与延迟预算。
  旧的 `chat` / `fastChat` / `eval` 保留为别名（→ dialogue / reflex / assess）。
  为什么从三条变成六档，以及全部 67 个调用点各归哪一档：见
  `docs/superpowers/specs/2026-09-02-llm-routing-taxonomy-design.md`。

派生关系是单向的：价格表由目录生成，evalbench 由同一份目录解析，mux 按 `kind`（协议）分发而不是按厂商名——**所以新增一个 OpenAI 兼容厂商是加一段 JSON，不是写一个 Go adapter**。

## 怎么换模型

```bash
# 只动陪练这一档，评估不动
MODEL_DIALOGUE=dashscope/qwen3.8-max ./api

# 看八个档 + 各自的绑定、推理要求、延迟预算（不需要数据库）
./api --print-models
```

每档一个变量，是刻意的：一次只动一条，测出来的速度和成本才能归因到那个模型。变量为空时用 `models.json` 里的绑定。

**重新绑定不靠感觉，靠实测：**

```bash
DASHSCOPE_API_KEY=… go run ./cmd/routebench -out report.md
```

routebench 是一件**与运行系统分开**的工具——不连数据库、不被 `cmd/api` 引用（有测试守着），
用真实生产 prompt 跑候选模型，测结构合法性（喂真实解析函数）/ 延迟 / token / 判官质量，
输出一份推荐绑定。**改 `models.json` 的是人。** 见 `apps/api/cmd/routebench/README.md`。

**写错会在启动时失败，不会跑一周才发现**：
- id 不在目录里 → 报错并列出所有合法 id
- `MODEL_ASSESS` 指向非 `flagship` 模型 → 拒绝启动（过程评估绝不降级）
- 要求关思考的档（reflex / dialogue / digest）绑了 `thinkingOffUnsupported` 的模型 → 拒绝启动
- 档绑定了图像 / embedding 模型 → 拒绝启动并说明原因

## 默认走阿里云 DashScope（百炼）

DashScope 是聚合端点：**一把 key 直达 250 个模型**——Qwen（含 omni 多模态）、DeepSeek、GLM、Kimi（含 code）、图像、视频、embedding、rerank。这正是横向比模型需要的形状：一个账号、一份账单、同一套 OpenAI 兼容协议。

DashScope 上就有 `deepseek-v4-pro`，所以默认绑定它——**切换到聚合端点不改变实际服务的模型**。

迁移是渐进的，不是一刀切：`DASHSCOPE_API_KEY` 为空时，lane 会回退到直连厂商端点（优先找同一个模型的另一条路，找不到再用该 provider 的默认模型）。所以**加上这把 key 才是真正把流量切到 DashScope 的那一步**，出问题去掉即可回退。

## 一个不加测试就一定会踩的坑

同一个模型换一条路，关思考的字段不一样。2026-09-02 实测：

| 请求字段 | 直连 DeepSeek | 经 DashScope |
|---|---|---|
| `thinking: {"type":"disabled"}` | 生效 | **静默忽略**（照样推理） |
| `enable_thinking: false` | — | **生效**（deepseek / qwen / kimi / glm-5.1、5.2 都生效） |

`enable_thinking:false` 在 qwen3.8-max 上的实测：开着 = 53 completion tokens（其中 47 推理），
关掉 = 3 tokens、没有 `reasoning_content`、答案一样对。

危险的地方在于**它不报错**：请求 200，响应完全正常，只是慢了一个数量级、贵了一个数量级。如果只是把 base URL 指向 DashScope 而不动 body，陪练那条 lane 会悄悄恢复满额推理——正是 `keyresolver.go` 里记录过的那次回归（每轮 4,000–7,000 completion tokens、40–66s，最坏的一次 JSON 信封被截断）。

所以 `thinkingOff` 是写在 provider 上的**数据**，不是写在 adapter 里的代码。换通道后必须实测验证：

```bash
LIVE_LLM=1 DASHSCOPE_API_KEY=... go test ./internal/gateway -run TestLive -v
```

这两个 live 测试断言的是单元测试看不见的东西：陪练 lane 回来的 **reasoning token 必须是 0**，以及流式 tool call 的参数分片必须能重新拼成 JSON。已验证通过：陪练 lane 2.0s、零推理 token；tool call 正常。

## 每个模型能不能关推理 / 能不能调档

「关思考」这件事**不只按通道分，还按模型分**。用
`internal/gateway/probe_reasoning.py` 实测（每档取 3 次的中位数——单次采样会上下
浮动三分之一，足以凭空造出一个并不存在的「档位控制」）：

| 模型 | 默认推理 tokens | `enable_thinking:false` | 档位 low→high |
|---|---|---|---|
| `deepseek-v4-pro` | 106 | 归零 | 无差别 |
| `deepseek-v4-flash` | 89 | 归零 | 无差别 |
| `qwen3.8-max` | 61 | 归零 | 63→86 有效 |
| `qwen3.7-max` | 347 | 归零 | 65→408 有效 |
| `kimi-k3` | 50 | 归零 | 44→51 有效 |
| `kimi-k2.6` | 0 | 本来就不推理 | — |
| `kimi-k2.7-code` | 59 | 🚨 **静默忽略**（61） | 无差别 |
| `glm-5.2` | 253 | 归零 | 无差别 |
| `glm-5.1` | 195 | 归零 | 无差别 |
| `ZHIPU/GLM-5.3` | 49 | 🚨 **直接报错** | 只能靠档位 |

两个坑，都已经写进 `models.json`：

**GLM 5.3（含 Flash）拒绝关思考**，返回
「该模型始终思考，不支持关闭思考；请使用 low、high 或 max」。所以它带
`thinkingOffUnsupported` + `defaultReasoningEffort: low`——陪练那条 lane 于是改发
档位字段，而不是发一个会被 400 掉的字段。不这么标，GLM 5.3 上的每一轮陪练都是硬报错。

**`kimi-k2.7-code` 接受 `enable_thinking:false` 然后照样推理**。这是「慢且贵且不报错」
那一类，所以同样标成 `thinkingOffUnsupported`：宁可什么都不发、明确知道它一直在推理，
也不要发一个只买来一句安慰的字段。它因此**不能绑到陪练 lane**——live 测试会当场红。

名字上还有两个意外：GLM 5.3 认的是 `ZHIPU/GLM-5.3`（裸 `glm-5.3` 是 access denied，
`glm-5.3-flash` 是 404，Flash 的真名是 `ZHIPU/GLM-5.3-Flash`）；而且**要先在百炼控制台
开通**，开通前每一次调用都是一个读起来像参数错误的 400。

## 待办：DashScope 的价格

DashScope 按人民币计价，费率我没有可靠来源，**所以目录里 DashScope 那些模型的 `priceUsd` 是空的**——宁可记成本为空，也不能编一个数字，否则这个目录存在的意义（比成本）当场就废了。`--print-models` 会把它们标成 `UNPRICED`。

从百炼控制台拿到费率后，填进 `models.json` 对应模型的 `priceUsd` 即可，`llm_call` 的成本记录会自动跟上。

## 后面的多模态 / 编码

目录里已经登记了 `dashscope/kimi-k2.7-code`（编码）、`dashscope/qwen3.5-omni-plus|flash`（多模态输入）、`dashscope/qwen-image-3.0*`、`dashscope/qwen-image-edit-max`、embedding 与 rerank，并用 `capabilities` 标注。

登记之前每个名字都对着这个 workspace 自己的 `GET /models` 核过一遍。PAI 有、DashScope
没有的那几个（`wan3.0-video`、`qwen3-vl-embedding`、`text-embedding-v4`、`qwen3-rerank`）
**没有顺手搬过来**——目录里写着一个通道根本供不出来的模型，正是这份文件存在的意义要防的那种谎。
（embedding / rerank 在 DashScope 的真名是 `qwen3.7-text-embedding` 和 `qwen3.7-text-rerank`。）

其中 chat 类的（coding / omni）**现在就能绑定**到 lane 上。图像 / 视频 / embedding / rerank 走的是别的端点（`/images/generations`、`/embeddings`、`/rerank`），需要各自的 provider `kind` —— 到时候是加一个 adapter，而不是重新调研一遍厂商。lane 只接受 chat 模型，绑错了启动就失败。
