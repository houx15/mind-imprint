# 分级路由实测：这一组绑定是怎么定下来的

2026-09-03 · 数据来自 `go run ./cmd/routebench` · 判官 `dashscope/qwen3.7-max`

> **状态：历史基线。** 当前 `framework ready` 校准与 `review` 绑定见
> `docs/2026-09-14-framework-ready-routing-benchmark-findings.md`。

---

## 结论：2026-09-03 当时这一组

| 档 | 绑定 | 相比之前 | 依据（第二轮，判官可用） |
|---|---|---|---|
| `reflex` | qwen3.7-flash | ← deepseek-v4-pro | 200ms，输出 3 tokens；glm-5.3 同样结构全对但 1.5s |
| `dialogue` | deepseek-v4-pro（关思考） | 模型不变，推理从满额改成关 | 阅读陪练最差一项 3 分，四个候选里唯一没掉到 1–2 的 |
| `compose` | **glm-5.3** | ← kimi-k3 | 三个候选质量并列 4 分，4.1s vs 10.3s |
| `review` | deepseek-v4-pro | 不变 | 四个候选仍然全是 2 分——并列不构成换绑的依据 |
| `assess` | deepseek-v4-pro | 不变 | 与 qwen3.8-max 并列 5 分；并列时不动，何况这一档绝不降级 |
| `digest` | deepseek-v4-flash | ← deepseek-v4-pro | 1.4s vs 2.3s，两者都 5 分 |

**最大的一笔收益不在这张表里。** lite 的阅读陪练、PBL 对话、写作间那批调用原来全都落在
旗舰档、满额推理上——不是配错了，是只有三条 lane 时「要一点判断力」只能靠买下
绝不降级的审阅档来换。归档之后它们落在 `dialogue`（关思考），实测每轮 1.2–3.8 秒。

---

## 一、第一轮的三个结论是错的，而且错在我这边

产品负责人问了一句「glm-5.3 和 qwen3.8-max 看起来更强，为什么一个都没进最终名单」。
查下来，两个模型都**从来没有被公平地测过**，三处原因全是工具的问题，不是模型的：

**① `thinkingOffUnsupported` 被我当成了「这个模型停不下来思考」。**
它的真实含义只有「这条通道不收关思考那个字段」。百炼的 400 把该怎么办写得很清楚：

> `code 1210: 该模型始终思考，不支持关闭思考；请使用 low、high 或 max。`

实测：`ZHIPU/GLM-5.3` 用 `reasoning_effort:"low"` 时 **推理 token 为 0**，回复正常。
那就是关思考，只是换了个旋钮。而我那条规则把 glm-5.3 挡在了
`reflex` / `dialogue` / `digest` 三档门外，整整一轮。

**② glm-5.3 在 `assess` 上的 400 根本与推理无关。**

> `code 1214: messages 参数非法。`

`GenerateLookback` 发的是**一条 system 消息、没有 user 消息**。目录里其他每个模型都收，
只有 `ZHIPU/GLM-5.3` 不收。这是我们自己的请求构造问题——也意味着
「换模型 = 改一个环境变量」这句承诺当时是假的：把 `MODEL_ASSESS` 指向任何 GLM 模型
都会在线上 400。

**③ 我们把百炼的错误正文扔了。** `openai_compatible.go` 在非 200 时
`io.Copy(io.Discard, resp.Body)`，错误只剩一句 `dashscope http 400`。
上面两条原话一直都在，只是从没到过我眼前。现在带上了（401/403 除外——那正是
provider 可能把 key 回显出来的场合，AGENTS.md 不允许密钥进抛出的错误）。

## 二、第一次重跑整轮作废：判官全程 400

换上 qwen3.7-max 当判官之后，**每一次判分调用都失败**，报告却照样印出了一张推荐表——
按 token 数排的名次，质量列全空。原因是 `reasoning_effort` **不是一个统一的枚举**：

| | `low` | `high` | `max` | `xhigh` |
|---|---|---|---|---|
| qwen3.7-max / -plus / -flash、kimi-k2.6 | ✓ | ✓ | **400** | ✓ |
| ZHIPU/GLM-5.3 | ✓ | ✓ | ✓ | **400** |
| qwen3.8-*、deepseek-v4-*、kimi-k3、glm-5.2 | ✓ | ✓ | ✓ | ✓ |

`assess` 档发的是 `max`，判官恰好是 qwen3.7-max。同一个家族的 3.7 和 3.8 词表都不一样，
所以这张表现在写进 `models.json`（`reasoningEffortAliases`），别再靠猜。

修法有两条：模型侧加别名翻译；**工具侧改成「有判官用例却一个分都没拿到，就不出推荐」**。
缺一次测量必须读成「缺」，不能读成「并列」。

## 三、判分规则第三次被磨：成本只能打破平局，不能压过差距

拿到真实质量数据后，`dialogue` 这一档是这样的：

| 模型 | pro 陪练 | lite 写作 | PBL | **lite 阅读陪练** | 最差 |
|---|---|---|---|---|---|
| deepseek-v4-pro | 4 | 5 | 5 | **3** | **3** |
| qwen3.8-max | 3 | 5 | 5 | **2** | 2 |
| glm-5.3 | 5 | 5 | 5 | **1** | 1 |
| glm-5.2 | 4 | 5 | 5 | **1** | 1 |

工具当时推荐了 **qwen3.8-max**——最差一项比 deepseek 低一分，但还在「一分带」内，
于是按 token 更少胜出。第一轮学到的「看最差一项」只做成了**过滤器**，没做成**排序依据**。
现在的规则是：最差一项 → 均分 → **延迟** → token。

> 三者的优先级由产品负责人定（2026-09-03）：**性能 > 速度 > 成本**。
> 速度排在成本前面，因为等待是学生亲身经历的，token 账单不是。

还加了一条：`review` / `assess` 这种「对不对压倒便宜」的档，**并列不构成换绑的依据**。
四个模型都判 2 分说明的是评分标准没有区分度，不是最便宜的那个赢了。

## 四、glm-5.3 到底强在哪、弱在哪

被公平测过之后，它不是不能用，而是**很快、判断力不稳**：

- `compose`（阅读 router + 检索建议）：**4 分、结构 100%、4.1s、零推理 token**。
  kimi-k3 同样 4 分要 10.3s。这一档换成它。
- `dialogue` 的 pro 陪练拿了全场唯一的 **5 分**；但同一个模型在 **lite 阅读陪练拿 1 分**——
  「直接把答案告诉了学生……以『收工』结束，等于在她根本没回答的情况下放她过去」。
  一个学生面坏掉就是坏掉，所以这一档不给它。
- `assess`：**16384 tokens 里 16382 是推理，输出只有一个左括号，4 分 43 秒**。
  这一档要的是长而结构化的产物，它会把预算全烧在思考上。出局。
- `reflex`：结构全对，但 1.5s 对 qwen3.7-flash 的 200ms，且仍花了 45 个推理 token。

qwen3.8-max 在 `assess` 上与 deepseek-v4-pro **并列 5 分**，而且更省（3080 vs 4489 tokens）。
并列不换绑，但它现在是这一档有据可查的备选。

## 五、`review` 的那个 prompt bug 还在

用例是一份**故意的半成品**框架。第二轮换了判官、加了候选，**四个模型仍然全部判 ready**，
全部 2 分。deepseek-v4-pro 的扣分理由：「将半成品框架标记为 ready=true，且三条建议均为
直接告知学生该做什么……违反铁律①」。

**这是 prompt 的问题，换模型解决不了。** 待办。

---

## 成本这一列还是空的

DashScope 那批模型 `priceUsd` 全是空的，所以上面所有「便宜」都是**按 token 量**说的。
宁可记成本为空也不编数字。把百炼控制台的费率填进 `models.json`，同一份结果不用重跑
就能换算成钱。

## 下次怎么重跑

```bash
cd apps/api
DASHSCOPE_API_KEY=… go run ./cmd/routebench -config cmd/routebench/routebench-round2.json \
  -cases reflex,dialogue,compose,digest,review -out round2b-fast.md
# 慢档单独跑：单次调用 80 秒以上
DASHSCOPE_API_KEY=… go run ./cmd/routebench -config cmd/routebench/routebench-round2.json \
  -cases assess -samples 1 -out round2-assess.md
```

慢档分开跑是第一轮的教训：整轮被中断在 `assess` 里，前面每一档都测完了，
但报告只在最后写一次，全部结果连同已经付过钱的一百多次调用一起没了。
现在每跑完一个用例就落一次报告。

## 当时待办（2026-09-03）

1. **`frameworkReviewSystem` 要改**：四个模型都把半成品判成 ready，这是 prompt 的问题。
2. **compose 再加一两个判官用例**：现在只有一个，结论比别的档薄。
3. **填价格**：目录里 22 个模型 `UNPRICED`。
4. **`dialogue` 要不要改成 `low`**：现在关思考，所以「查看思考过程」那个折叠区在
   陪练对话上是空的。要让它有内容，就得给这一档一点推理预算——用延迟换透明度。
5. **联网搜索接到学生流程里**：`enable_search` 已经在网关上（按次开关，不按档），
   但还没有任何调用点用它。注意实测：这条端点**不返回结构化的 search_results**，
   只在正文里引用来源——溯源体检要可点开的 URL，仍然得走 `internal/websearch`。
