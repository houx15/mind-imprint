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
- **lanes** —— 三条通道对应产品里的三种角色：`chat`（陪练）、`fastChat`（状态路由）、`eval`（旗舰审阅 / 评估）。

派生关系是单向的：价格表由目录生成，evalbench 由同一份目录解析，mux 按 `kind`（协议）分发而不是按厂商名——**所以新增一个 OpenAI 兼容厂商是加一段 JSON，不是写一个 Go adapter**。

## 怎么换模型

```bash
# 只动陪练这条 lane，评估不动
MODEL_CHAT=pai/qwen3.8-max ./api

# 看目录 + 当前绑定（不需要数据库）
./api --print-models
```

三条 lane 相互独立，是刻意的：一次只动一条，测出来的速度和成本才能归因到那个模型。三个变量为空时用 `models.json` 里的绑定。

**写错会在启动时失败，不会跑一周才发现**：
- id 不在目录里 → 报错并列出所有合法 id
- `MODEL_EVAL` 指向非 `flagship` 模型 → 拒绝启动（评估走旗舰模型绝不降级）
- lane 绑定了图像 / embedding 模型 → 拒绝启动并说明原因

## 默认走阿里云 PAI

PAI 是聚合端点：**一把 key 直达 33 个模型**——Qwen（含 omni 多模态）、DeepSeek、GLM、Kimi（含 code）、图像、视频、embedding、rerank。这正是横向比模型需要的形状：一个账号、一份账单、同一套 OpenAI 兼容协议。

PAI 上就有 `deepseek-v4-pro`，所以默认绑定它——**切换到聚合端点不改变实际服务的模型**。

迁移是渐进的，不是一刀切：`PAI_API_KEY` 为空时，lane 会回退到直连厂商端点（优先找同一个模型的另一条路，找不到再用该 provider 的默认模型）。所以**加上这把 key 才是真正把流量切到 PAI 的那一步**，出问题去掉即可回退。

## 一个不加测试就一定会踩的坑

同一个模型换一条路，关思考的字段不一样。2026-09-02 实测：

| 请求字段 | 直连 DeepSeek | 经 PAI |
|---|---|---|
| `thinking: {"type":"disabled"}` | 生效 | **静默忽略**（照样推理） |
| `reasoning_effort: "low"` | 生效 | deepseek / glm 忽略，qwen 生效 |
| `enable_thinking: false` | — | **生效**（三家都生效） |

危险的地方在于**它不报错**：请求 200，响应完全正常，只是慢了一个数量级、贵了一个数量级。如果只是把 base URL 指向 PAI 而不动 body，陪练那条 lane 会悄悄恢复满额推理——正是 `keyresolver.go` 里记录过的那次回归（每轮 4,000–7,000 completion tokens、40–66s，最坏的一次 JSON 信封被截断）。

所以 `thinkingOff` 是写在 provider 上的**数据**，不是写在 adapter 里的代码。换通道后必须实测验证：

```bash
LIVE_LLM=1 PAI_API_KEY=... go test ./internal/gateway -run TestLive -v
```

这两个 live 测试断言的是单元测试看不见的东西：陪练 lane 回来的 **reasoning token 必须是 0**，以及流式 tool call 的参数分片必须能重新拼成 JSON。已验证通过：陪练 lane 2.0s、零推理 token；tool call 正常。

## 待办：PAI 的价格

PAI 按人民币计价，费率我没有可靠来源，**所以目录里 PAI 那些模型的 `priceUsd` 是空的**——宁可记成本为空，也不能编一个数字，否则这个目录存在的意义（比成本）当场就废了。`--print-models` 会把它们标成 `UNPRICED`。

从 PAI 控制台拿到费率后，填进 `models.json` 对应模型的 `priceUsd` 即可，`llm_call` 的成本记录会自动跟上。

## 后面的多模态 / 编码

目录里已经登记了 `pai/kimi-k2.7-code`（编码）、`pai/qwen3.5-omni-plus|flash`（多模态输入）、`pai/qwen-image-3.0*`、`pai/wan3.0-video`、embedding 与 rerank，并用 `capabilities` 标注。

其中 chat 类的（coding / omni）**现在就能绑定**到 lane 上。图像 / 视频 / embedding / rerank 走的是别的端点（`/images/generations`、`/embeddings`、`/rerank`），需要各自的 provider `kind` —— 到时候是加一个 adapter，而不是重新调研一遍厂商。lane 只接受 chat 模型，绑错了启动就失败。
