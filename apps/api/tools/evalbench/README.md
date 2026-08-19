# Evalbench

Evalbench 是一个独立的离线 CLI，用来在同一份冻结的学生过程输入上比较评估方案和旗舰模型：

- candidate 与人工 gold 在各评价项上的对齐与差异；
- 多次成功运行之间的稳定性；
- 成功率、端到端耗时、TTFT 与 provider 原始 token 用量；
- 每一次真实模型调用的原始输出和观测数据。

它不会连接数据库、写入正式 `evaluation`、改变项目状态，或影响线上 `review → evaluation` 流程。

实现约束、输入映射和结果格式见 [SPEC.md](SPEC.md)。

## 前置条件

从 `apps/api` 目录运行命令：

```bash
cd apps/api
```

在服务端环境或 `apps/api/.env.local` 中提供所用 provider 的 key：

```bash
DEEPSEEK_API_KEY=...
# 或
ANTHROPIC_API_KEY=...
```

配置中的 provider 目前只能是 `deepseek` 或 `anthropic`。评估试验台始终按 `flagship` 档位调用模型。

## 配置实验

默认配置是 [config.json](config.json)。其中包含三个展示 case（深度研究、确认偏误、代写依赖）、对应 Gold Markdown、模型 profile，以及两个同模型 variant：在 Evalbench 内本地复刻的四调用基线 `production-evalreport-v1`，和可行的单次综合调用 `single-prompt-evalreport-v1`。后者使用 24K 输出、reasoning、DeepSeek JSON mode、完整 Zod 派生 JSON Schema 和完整结构示例。默认每个 case/variant 要求两次完整成功，最多三次 attempt；可直接运行，或按需要另建 JSON 并用 `--config` 覆盖。成本估算复用服务端共享 USD 价格表；配置不维护第二份价格或价格覆盖项。

```json
{
  "schemaVersion": 2,
  "name": "baseline-2026-08",
  "successfulRuns": 2,
  "maxAttempts": 3,
  "cases": [
    {
      "id": "persona2-deepdiver",
      "projectData": "cases/persona2-deepdiver.json",
      "goldReport": "cases/persona2-deepdiver-gold.md"
    },
    {
      "id": "persona4-confirmbias",
      "projectData": "cases/persona4-confirmbias.json",
      "goldReport": "cases/persona4-confirmbias-gold.md"
    },
    {
      "id": "persona5-ghostwrite",
      "projectData": "cases/persona5-ghostwrite.json",
      "goldReport": "cases/persona5-ghostwrite-gold.md"
    }
  ],
  "models": {
    "deepseek-flagship": {
      "provider": "deepseek",
      "model": "deepseek-v4-pro"
    }
  },
  "variants": [
    {
      "id": "production-current",
      "evaluator": "production-evalreport-v1",
      "model": "deepseek-flagship",
      "params": {}
    },
    {
      "id": "single-prompt-v1",
      "evaluator": "single-prompt-evalreport-v1",
      "model": "deepseek-flagship",
      "params": {}
    }
  ],
  "comparator": {
    "model": "deepseek-flagship",
    "promptVersion": "comparator-v1"
  }
}
```

实验配置中的相对路径以配置文件所在目录为准。`successfulRuns` 是每个 case/variant 需要的完整有效 run 数：candidate Report 与它对应的 gold comparison 都必须通过校验。`maxAttempts` 是包含失败尝试的上限，且必须不小于前者。

可选 `timeouts` 为每次模型调用设置上限（单位：秒）。未配置时，candidate 默认为 300 秒，comparator 默认为 120 秒；超时会作为该次尝试的可记录失败，而不会无限等待：

```json
"timeouts": {
  "candidateSeconds": 300,
  "comparatorSeconds": 120
}
```

一个模型 profile 可被多个 variant 使用，因此可在不改生产代码的前提下，配置同一 evaluator 分别使用多个旗舰模型。默认两组都使用同一旗舰模型并保持 reasoning 开启。单 prompt v1 的正常完整实验发起 production 的 4 次 candidate calls 加单 prompt 的 1 次 candidate call，以及每个成功 variant 各 1 次 comparator call。

## 添加实验 evaluator

Evalbench 的 evaluator 是受版本控制的 Go 实现加可审计 prompt，不支持从配置执行任意 shell、Go 脚本或任意外部 prompt 文件。

- 适合放在 evalbench：只读取冻结 Evalbench `Input`、只输出标准 `EvaluationReport`、不连数据库、不改变生产状态、且仅用于离线比较的简单实验方案。例如 `single-prompt-evalreport-v1` 使用一个嵌入式版本化 prompt。
- 应抽到共享 `internal/...` 包：方案需要数据库或共享业务状态、异步任务、生产复用，或要改变真实线上评估管线。evalbench 在这种情况下只负责适配输入、调用和观测。

每个 evaluator 必须注册固定 ID、拒绝未声明参数、通过严格报告校验，并在 manifest 记录实现版本；有嵌入 prompt 时还要记录 prompt SHA-256。这样修改 prompt 不会被误认为同一实验。

## 准备人工 Gold

运行实验前，请自行提供并确认标准 Markdown gold。evalbench 不读取、转换或生成 `.docx`、`.txt` 等源材料，也不会调用模型补全 gold。

每份 gold 至少包含 D1–D6、A1–A6、综述、提问透镜、风险、下一步和未归类内容。运行前会校验这些章节是否存在；FACT 区域不会交给 LLM 裁判。具体格式可参考 [persona2-deepdiver-gold.md](cases/persona2-deepdiver-gold.md)。

## 运行实验

```bash
go run ./tools/evalbench run
```

指定另一份配置：

```bash
go run ./tools/evalbench run --config /absolute/path/experiment.json
```

默认结果写入 `tools/evalbench/results/<experiment-id>/`。指定其他本地目录：

```bash
go run ./tools/evalbench run \
  --config /absolute/path/experiment.json \
  --results-dir /absolute/path/eval-results
```

运行会先完成配置、文件、gold 格式、评估器参数和模型 key 的 preflight；之后才创建 experiment ID 并发起模型请求。同一 case 内，多个 variant 会轮流执行，而不会先跑完其中一个。

退出码：

- `0`：所有 case/variant 达到 `successfulRuns`；
- `1`：配置或 preflight 失败；
- `2`：至少一个组合为 partial/failed，或运行被取消。

中断不会删除已完成结果；当前 MVP 不支持 resume，重新执行会产生新的 experiment ID。

## 查看结果

每次运行会创建独立目录，例如：

```text
tools/evalbench/results/20260814T120000Z-<uuid>/
  report.md                 # 面向内部研发/产品的精简客观结果摘要
  report-details.md         # 完整审计附件：candidate、attempt、comparison 与稳定性
  summary.json              # 机器可读的汇总
  manifest.json             # 输入、gold、配置、rubric hash、evaluator/prompt fingerprint 与执行顺序
  cases/<case-id>/
    input.json              # 所有 variant 共享的冻结 Evalbench Input
    input.sha256
    gold-report.md
    <variant-id>/
      attempt-001/
        status.json
        report.json
        comparison.json
        evaluator-calls/
        comparator-calls/
      stability.json
```

`report.md` 是默认阅读入口，按“实验范围 → 核心比较 → Gold 对齐与错误画像 → 性能与成本 → 可靠性与解释边界”组织。它只展示可由实验产物直接支持的数值和统计口径：不会选择或推荐 variant，不输出行动指引，也不将 Gold 对齐表述为绝对正确率。多 case 实验会附上紧凑的 case/variant 覆盖表；单次完整成功 run 会明确标示稳定性不可评估。

`report-details.md` 是审计附件，保留每个 case/variant 的 attempt 表、代表 run、完整 Candidate `EvaluationReport`、46 项 comparator 矩阵、调用成本明细、D/A 与 comparator 稳定性、hash 和执行顺序。代表 run 是 attempt 编号最小的完整成功 run；没有完整成功 run 时，最早已落盘 candidate 可作为不计入成功率的诊断预览。附件中的 Candidate 原始建议和行动内容不代表 EvalBench 对评估方案的建议。

报告中的 D1–D6、A1–A6 名称直接来自 canonical rubric。自主轴是 0–5 行为计数带，不是质量分数；不会合成学生总分。缺失的 token、TTFT、comparison 或 evidence 显示为“未提供”。学生原话、模型输出和 HTML/Markdown 特殊字符都会按展示场景转义。

`summary.json`、`manifest.json`、各 attempt 的 `report.json` / `comparison.json` 和 `stability.json` 仍是机器可读真相源；两份 Markdown 都只是对这些现有产物的确定性汇总，不发起额外模型调用，也不改变成功判定。

成本采用“全成本分拆”口径：每个 variant 分别显示 Candidate、Comparator 和两者合计的实验支出；所有失败、流异常和重试 attempt 都会计入。报告还会给出 Candidate/完整成功 run（评估器运行效率）和总成本/完整成功 run（含 comparator 的实验预算）。金额固定为 USD 估算，使用运行开始时写入 `manifest.json` 的 gateway 费率快照；输出 token 包含 reasoning token。它不是 provider 最终账单，不包含折扣、税费、cache hit 或汇率。

模型未定价或 provider 没返回完整 usage 时，报告只显示已知成本小计和覆盖情况，绝不把未知成本写成 `$0`；没有发起调用时才明确显示 `$0.000000`。`summary.json` 的 `costUsd` 只在全部调用可计价时出现，`knownCostUsd` 则始终表示已覆盖调用的小计。

`summary.json` 将 candidate 与 comparator 的调用统计分开，包含调用次数、输入/输出 token、reasoning/content token、成本覆盖和异常流数。`contentTokens` 仅在 provider 返回 `reasoningTokens` 时按 `outputTokens - reasoningTokens` 记录；不保存 reasoning/CoT 原文。若 provider 没有返回 usage，相关 token 会保持为 `null`，不会自行估算。流式调用在 HTTP 200 后仍可能以 `provider stream read failed`、`provider stream ended before completion` 或超时失败；这些安全分类会写入调用 metadata，不记录 key 或上游响应体。

`single-prompt-evalreport-v1` 的正常 candidate attempt 只有 `single_prompt_evalreport_v1` 一次调用，不做 JSON 重试；输出少维度、重复维度、非法字段、不合法 JSON 或模型截断都会使 attempt 失败。这是有意设计，用来测量单次综合 prompt 的真实完整率，而不是用回填掩盖它。24K JSON mode 只由 DeepSeek 支持；若配置到不支持的 provider，调用会明确失败而不会静默降级。

结果目录默认被 Git 忽略。调用记录中的 request 和 raw output 可能包含实验输入与 prompt，应按学生过程数据妥善保管；其中不会记录 API key。
