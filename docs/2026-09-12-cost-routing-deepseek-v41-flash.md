# 2026-09-12 · 成本与路由：DeepSeek V4.1 Flash 实测

起点是一句话：「qwen-3.8-max 成本极高，能不能测一下刚发布的 deepseek-v4.1-flash 来比较。」

下面每一条都是当天实测出来的，不是从文档推的。

---

## 1 · qwen3.8-max 没有服务任何一次学生请求

`--print-models` 的绑定表里，qwen3.8-max 只出现在一个地方：`search` 档。
而 `search` 是预留档，全仓 grep `ClassSearch` 只有三处命中，全是声明（`catalog.go`
定义它、`Classes` 列出它、`keyresolver.go` 把它和 `multimodal` 一起排除），
**没有任何调用点路由到它**。

它真正被调用的地方是两件走查工具：

- `apps/lite-web/e2e/camp/brain.ts` — `CAMP_BRAIN_MODEL ?? "qwen3.8-max"`
- `apps/lite-web/e2e/readwalk/brain.ts` — `READ_BRAIN_MODEL ?? "qwen3.8-max"`

这两件工具拿开发机上的 `DASHSCOPE_API_KEY` 直接打百炼，**和产品用的是同一把 key、
同一张账单**，但它们不经过网关，所以 `llm_call` 表里一行都没有。
这解释了为什么控制台上这个模型很贵，而我们自己的成本表里看不到它：
**那是我们自己跑走查的钱，不是学生的钱。**

它在这两处是被**故意**选贵的：印记走 deepseek，学生必须换一家的模型演，
否则就是同一个脑子自问自答，界面上说不清的地方会被自动补上。
所以**不建议**把这两个默认值调低——
`docs/2026-09-12` 之前已经栽过两次「走查记下的产品缺陷，一半是那只眼睛自己的毛病」。
要省这笔钱，省的是**跑的次数和轮数**，不是换一只更差的眼睛。

---

## 2 · deepseek-v4.1-flash 在哪条通道上存在

| 通道 | 结论 |
|---|---|
| DashScope（百炼聚合口，生产走的这条） | **没有**。`GET /models` 的 249 个名字里没有它；`deepseek-v4.1-flash` / `deepseek-v4.1` / `deepseek-v4.1-pro` 三个名字直接调都是 `model_not_found`。那边的 `deepseek-v4-flash` 仍是老的 V4 Flash |
| DeepSeek 直连 | **有**。线名不带版本号，就叫 `deepseek-flash` |

直连口的 `GET /models` 只列两行：`deepseek-flash`、`deepseek-v4-pro`。
官方把旧线名 `deepseek-v4-flash` 指向了 V4.1 Flash——实测请求它，回包的 `model`
字段是 `deepseek-flash`。

所以「同一个线名在两条通道上不是同一个模型」在这里是字面意义上成立的。

思考开关实测（直连口）：

```
thinking 默认      completion 300，其中 reasoning 300，content 空
thinking disabled  completion  20，其中 reasoning   0，答案正常
```

`{"thinking":{"type":"disabled"}}` 有效，和目录里 `deepseek` provider 已有的
`thinkingOff` 一致，不用新写。

---

## 3 · dialogue 档实测（n=3，判官 qwen3.7-max）

`dialogue` 是调用量最大的一档，也是唯一一档学生每说一句就打一次。

| 模型 | 结构 | 质量（四格） | 均分 | 最差 | 出 tokens | p50 总时长 |
|---|---|---|---|---|---|---|
| `dashscope/deepseek-v4-pro`（在位） | 100% | 5 / 5 / 5 / **1** | 4.0 | **1** | 252 | 1.6–4.2s |
| `deepseek/deepseek-flash`（V4.1） | 100% | 5 / 5 / 3 / **2** | 3.8 | **2** | 273 | **0.8–1.6s** |
| `dashscope/deepseek-v4-flash` | 100% | 2 / 5 / 5 / 1 | 3.25 | 1 | 251 | 1.1–2.5s |
| `dashscope/qwen3.8-flash` | 100% | 4 / 4 / 3 / 1 | 3.0 | 1 | 242 | 1.6–3s |
| `dashscope/qwen3.7-plus` | 100% | 3 / 5 / 5 / 1 | 3.5 | 1 | 217 | 1.2–2.6s |

四格依次是 `pro-coach-turn` / `lite-writing-coach` / `pbl-turn` / `lite-reading-coach`。

工具推荐 `deepseek/deepseek-flash`。三件事值得单独说：

**① 均分低 0.2，最差项高 1 分。** 按「给最差的一项打分」这条规矩，V4.1 Flash 赢。
在位模型在 `lite-reading-coach` 上拿 1：它把「装机容量 ≠ 实际发电量」直接讲给学生，
然后放她过去。V4.1 Flash 在同一格拿 2——同样讲了答案，但至少判官认为它看出了
学生没回答那个问题。

**② `lite-reading-coach` 五个候选全军覆没（1,2,1,1,1）。**
这一格在 2026-09-03 那轮是 deepseek 唯一拿 5 的地方，也正是当时不换它的理由。
今天它拿 1。**这说明该格的问题已经从「模型不行」变成了「prompt 不行」，换模型解决不了**——
和当年 `review` 档「所有候选都只有 2 分」是同一种结论。这一格该单独修 prompt。

**③ 延迟差距比质量差距大得多。** V4.1 Flash 在最重的一格（7,067 入 tokens）
1.6s 答完，在位模型 4.2s。学生当场看得见的一档，这是能感觉到的差别。

---

## 4 · compose 档实测

| 模型 | 结构 | 质量 | 出 tokens | 其中推理 | p50 |
|---|---|---|---|---|---|
| `dashscope/glm-5.3`（在位） | 100% | 4 | 380 | 20 | 6.8s |
| `dashscope/glm-5.3-flash` | 100% | 3 | 394 | 35 | 5.9s |
| `deepseek/deepseek-flash` | 100% | 4 | 1774 | **1436** | 11.2s |
| `dashscope/qwen3.8-flash` | **0% / 67%** | 4 | 711 | 386 | 22s |
| `dashscope/qwen3.7-plus` | 100% | 4 | 2843 | **2436** | 32.8s |

结论：**compose 不动。**

- `qwen3.8-flash` 出局：生产解析器拒收 100% / 33%，线上就是给学生弹一个退回。
- `deepseek-flash` 和 `qwen3.7-plus` 在这一档刹不住思考（1,436 / 2,436 推理 tokens）。
  这一档要求 `reasoning: low`，而这两个模型的 effort 旋钮实测「无效」——
  它们便宜，但放在会思考的路线上就不再便宜。
- `glm-5.3-flash` 是唯一真正的候选：token 和延迟与在位持平，价格差一个数量级。
  但它在 `reading-router` 上被判违反铁律①（把推理跳跃直接讲给学生），比在位低 1 分。
  **这一分要不要换那个数量级，是产品决定，不是工具决定。**

---

## 5 · 价格：这仍然是最大的一个洞

目录里 22 个模型 `priceUsd` 为空，**全部 DashScope 行在内**。后果有两层：

1. `llm_call` 给每一次走 DashScope 的调用记成本 **0**。线上成本这一列基本是空的。
2. routebench **按 token 量排序，不是按钱**。这会得出错的结论——
   compose 那一档就是现成的例子：`glm-5.3-flash` 比 `glm-5.3` 多用 4% 的 token，
   却便宜一个数量级，而工具按 token 判在位赢。

今天查到的公开报价（**两个来源，未经控制台核对**）：

| 模型 | help.aliyun.com（CNY/1M 入/出） | 第三方聚合站 | 一致？ |
|---|---|---|---|
| qwen3.8-max | 12 / 36 | 11.94 / 35.83 | ✅ |
| qwen3.8-flash | 0.8 / 2.7 | 0.80 / 2.69 | ✅ |
| qwen3.7-flash | 0.2 / 0.8 | 0.20 / 0.80 | ✅ |
| kimi-k3 | 20 / 100 | 20 / 100 | ✅ |
| GLM-5.3 | 8 / 28 | 9.41 / 29.56 | 接近 |
| GLM-5.3-Flash | 0.8 / 2.8 | — | — |
| **deepseek-v4-pro** | **12 / 24** | **9 / 27** | ❌ |
| **deepseek-v4-flash** | **1 / 2** | **3 / 9** | ❌ |
| qwen3.7-plus | 2 / 8 | 3.36 / 20.16 | ❌ |

**偏偏是 deepseek 那两行对不上，而那正是生产用得最多的两行。**
所以这些数字**没有**写进 `models.json`——宁可记空，也不能编一个数字。
请从百炼控制台把实际费率贴出来，我把它们填进目录；填完之后
**已经跑出来的这两份报告不用重跑就能换算成钱**。

顺带记下一个可以直接读出的数量级：kimi-k3 出 100 CNY/1M，是目录里最贵的一行，
是 deepseek-v4-pro 的 4 倍。`_bindingComment` 里写着 compose 曾推荐过它，
现在实际绑的是 glm-5.3 —— 这一条算是运气好。

**已改的两行（直连口，官方文档有明确报价）：**
`deepseek/deepseek-v4-pro` 从 0.435/0.87 改成 **1.32/3.96**，
`deepseek/deepseek-v4-flash` 从 0.14/0.28 改成 **0.30/1.20**（USD/1M，峰时、cache miss）。
原来那两个数是 V4 时代的旧价，**按它算出来的钱只有真实的三分之一**。
记峰时是保守：估高了只是保守，估低了会让一次该做的降级看起来没必要。

---

## 6 · 线上其实换不了模型（已修）

`deploy/docker-compose.prod.yml` 只往容器里传了三个旧变量
（`MODEL_CHAT` / `MODEL_FAST_CHAT` / `MODEL_EVAL` → dialogue / reflex / assess）。
`MODEL_COMPOSE` / `MODEL_REVIEW` / `MODEL_DIGEST` / `MODEL_DRAW` / `MODEL_REFLEX`
**根本进不去容器**。

也就是说 AGENTS.md 里「换模型 = 改一个环境变量，不改代码」这句话，
在线上对七个活跃档里的五个是假的：变量设了，服务照旧用目录里的绑定，
而改的人会以为自己已经换过了。**已补齐七个。**

---

## 7 · 建议（按收益排序）

1. **把百炼控制台的费率贴出来。** 在这之前所有成本结论都只有数量级，没有数字。
2. **`dialogue` 换 `deepseek/deepseek-flash`，一条变量：**
   `MODEL_DIALOGUE=deepseek/deepseek-flash`（或旧别名 `MODEL_CHAT`）。
   依据见 §3：结构 100%、最差项反而高 1 分、快 2–4 倍。
   **代价要说清楚：这会离开「一把 key、一张账单」**——AGENTS.md 里选 DashScope
   聚合口就是为了这个。换成两条通道两张账单，值不值得由产品定。
   直连口另有一个这份报告没测的便宜法：cache hit 是 $0.003/1M，比 cache miss 便宜 50 倍，
   而 `dialogue` 是**入 tokens 主导**的一档（最重那格 7,067 入 / 128 出），
   系统 prompt 前缀又长又固定，命中率应该很高。
3. **单独修 `lite-reading-coach` 的 prompt。** 五个模型全判 1–2 分，换谁都一样。
4. **`compose` 暂不动**，除非愿意用 1 分换一个数量级（§4）。
5. **`assess` 不动**（铁律：绝不降级），`review` 留着等 prompt 修完再测。
6. **走查那两只眼睛不要调低**（§1）。要省就少跑几轮。

---

## 复现

```bash
cd apps/api
DASHSCOPE_API_KEY=… DEEPSEEK_API_KEY=… go run ./cmd/routebench -cases dialogue -samples 3 -out dialogue.md
DASHSCOPE_API_KEY=… DEEPSEEK_API_KEY=… go run ./cmd/routebench -cases compose  -samples 3 -out compose.md
```

候选名单在 `apps/api/cmd/routebench/routebench.json`，这次把 `dialogue` 和 `compose`
两档的候选换成了「在位的 + 有可能在成本上赢它的」。
