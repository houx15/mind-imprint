# routebench · 分级路由实测

2026-09-20 15:20 · 每格 n=3，取中位数 · 判官 `dashscope/qwen3.7-max`

> **成本按 token 量排序，不是按钱。** 目录里有 23 个模型 `priceUsd` 为空——
> 宁可记成本为空，也不能编一个数字，否则这份目录存在的意义（比成本）当场就废了。
> 把百炼控制台的费率填进 `models.json` 之后，同一份结果不用重跑就能换算成钱。

## 推荐绑定

| 档 | 推荐模型 | 依据 |
|---|---|---|
| `dialogue` | `**无**` | 没有候选通过 |

**`dialogue` 排除了：**
- dashscope/deepseek-v4-flash — dialogue/lite-reading-coach/complete: deterministic expectations rejected 33% of outputs; dialogue/lite-reading-coach/hunt-picked: deterministic expectations rejected 67% of outputs; dialogue/lite-reading-coach/label: deterministic expectations rejected 67% of outputs
- dashscope/deepseek-v4-pro — dialogue/lite-reading-coach: deterministic expectations rejected 100% of outputs; dialogue/lite-reading-coach/hunt-picked: deterministic expectations rejected 67% of outputs; dialogue/lite-reading-coach/label: deterministic expectations rejected 100% of outputs
- dashscope/deepseek-v4.1-flash — dialogue/lite-reading-coach: deterministic expectations rejected 100% of outputs; dialogue/lite-reading-coach/complete: deterministic expectations rejected 100% of outputs; dialogue/lite-reading-coach/hunt-picked: deterministic expectations rejected 67% of outputs; dialogue/lite-reading-coach/label: deterministic expectations rejected 67% of outputs
- dashscope/qwen3.7-plus — dialogue/lite-reading-coach: deterministic expectations rejected 67% of outputs; dialogue/lite-reading-coach/complete: deterministic expectations rejected 67% of outputs; dialogue/lite-reading-coach/hunt-picked: deterministic expectations rejected 67% of outputs; dialogue/lite-reading-coach/label: deterministic expectations rejected 100% of outputs
- dashscope/qwen3.8-flash — dialogue/lite-reading-coach: deterministic expectations rejected 100% of outputs; dialogue/lite-reading-coach/start: deterministic expectations rejected 33% of outputs; dialogue/lite-reading-coach/help: deterministic expectations rejected 33% of outputs; dialogue/lite-reading-coach/hunt-picked: deterministic expectations rejected 100% of outputs; dialogue/lite-reading-coach/label: deterministic expectations rejected 67% of outputs
- deepseek/deepseek-flash — dialogue/lite-reading-coach: deterministic expectations rejected 33% of outputs; dialogue/lite-reading-coach/complete: deterministic expectations rejected 33% of outputs; dialogue/lite-reading-coach/hunt-picked: deterministic expectations rejected 67% of outputs; dialogue/lite-reading-coach/label: deterministic expectations rejected 67% of outputs

## 单轮结果

### `dialogue/lite-writing-coach`

档：`dialogue` · 调用点：`agent.ProposeProjectCoachReply (POST /writings/{id}/turn)`

| 模型 | 首字 | 总时长 | 入 tokens | 出 tokens | 其中推理 | 解析 | 预期 | 任务命中 | 质量 | 备注 |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|
| `dashscope/deepseek-v4-pro` | 900ms | 3.5s | 555 | 73 | 0 | — | 100% | — | — |  |
| `dashscope/deepseek-v4.1-flash` | 900ms | 3.2s | 556 | 103 | 0 | — | 100% | — | — |  |
| `deepseek/deepseek-flash` | 1s | 2.9s | 555 | 59 | 0 | — | 100% | — | — |  |
| `dashscope/deepseek-v4-flash` | 3.4s | 3.4s | 555 | 93 | 0 | — | 100% | — | — |  |
| `dashscope/qwen3.8-flash` | 500ms | 3.2s | 583 | 87 | 0 | — | 100% | — | — |  |
| `dashscope/qwen3.7-plus` | 700ms | 2s | 583 | 71 | 0 | — | 100% | — | — |  |

### `dialogue/lite-reading-coach`

档：`dialogue` · 调用点：`postReadingCoachTurn (POST /readings/{id}/coach)`

| 模型 | 首字 | 总时长 | 入 tokens | 出 tokens | 其中推理 | 解析 | 预期 | 任务命中 | 质量 | 备注 |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|
| `dashscope/deepseek-v4-pro` | 1.6s | 7.7s | 7872 | 175 | 0 | 100% | 0% | — | — | 预期不符：reply gives the target answer before the student requested it   |
| `dashscope/deepseek-v4.1-flash` | 10.9s | 13.8s | 7873 | 145 | 0 | 100% | 0% | — | — | 预期不符：advance = "done", want empty until the student states the distinction   |
| `deepseek/deepseek-flash` | 900ms | 5.1s | 7872 | 168 | 0 | 100% | 67% | — | — | 预期不符：reply gives the target answer before the student requested it   |
| `dashscope/deepseek-v4-flash` | 2.8s | 2.8s | 7872 | 107 | 0 | 100% | 100% | — | — |  |
| `dashscope/qwen3.8-flash` | 1.8s | 4.2s | 8368 | 131 | 0 | 100% | 0% | — | — | 预期不符：reply gives the target answer before the student requested it   |
| `dashscope/qwen3.7-plus` | 900ms | 2.1s | 8368 | 102 | 0 | 100% | 33% | — | — | 预期不符：reply gives the target answer before the student requested it   |

### `dialogue/lite-reading-coach/start`

档：`dialogue` · 调用点：`postReadingCoachTurn (POST /readings/{id}/coach)`

| 模型 | 首字 | 总时长 | 入 tokens | 出 tokens | 其中推理 | 解析 | 预期 | 任务命中 | 质量 | 备注 |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|
| `dashscope/deepseek-v4-pro` | 1.3s | 3.5s | 3430 | 83 | 0 | 100% | 100% | — | — |  |
| `dashscope/deepseek-v4.1-flash` | 1.7s | 3.6s | 3431 | 161 | 0 | 100% | 100% | — | — |  |
| `deepseek/deepseek-flash` | 1s | 3.6s | 3430 | 97 | 0 | 100% | 100% | — | — |  |
| `dashscope/deepseek-v4-flash` | 3.7s | 3.7s | 3430 | 145 | 0 | 100% | 100% | — | — |  |
| `dashscope/qwen3.8-flash` | 1.8s | 4.8s | 3450 | 70 | 0 | 100% | 67% | — | — | 预期不符：this turn must hand the student a valid card   |
| `dashscope/qwen3.7-plus` | 700ms | 2s | 3450 | 94 | 0 | 100% | 100% | — | — |  |

### `dialogue/lite-reading-coach/help`

档：`dialogue` · 调用点：`postReadingCoachTurn (POST /readings/{id}/coach)`

| 模型 | 首字 | 总时长 | 入 tokens | 出 tokens | 其中推理 | 解析 | 预期 | 任务命中 | 质量 | 备注 |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|
| `dashscope/deepseek-v4-pro` | 1.3s | 3.7s | 3444 | 89 | 0 | 100% | 100% | — | — |  |
| `dashscope/deepseek-v4.1-flash` | 1.9s | 3.1s | 3445 | 74 | 0 | 100% | 100% | — | — |  |
| `deepseek/deepseek-flash` | 1.2s | 3.2s | 3444 | 69 | 0 | 100% | 100% | — | — |  |
| `dashscope/deepseek-v4-flash` | 3.1s | 3.1s | 3444 | 86 | 0 | 100% | 100% | — | — |  |
| `dashscope/qwen3.8-flash` | 800ms | 2.1s | 3461 | 53 | 0 | 100% | 67% | — | — | 预期不符：reply gives the target answer before the student requested it   |
| `dashscope/qwen3.7-plus` | 800ms | 1.8s | 3461 | 88 | 0 | 100% | 100% | — | — |  |

### `dialogue/lite-reading-coach/complete`

档：`dialogue` · 调用点：`postReadingCoachTurn (POST /readings/{id}/coach)`

| 模型 | 首字 | 总时长 | 入 tokens | 出 tokens | 其中推理 | 解析 | 预期 | 任务命中 | 质量 | 备注 |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|
| `dashscope/deepseek-v4-pro` | 900ms | 4.6s | 3423 | 155 | 0 | 100% | 100% | — | — |  |
| `dashscope/deepseek-v4.1-flash` | 3.4s | 5.2s | 3424 | 98 | 0 | 100% | 0% | — | — | 预期不符：advance = "", want "done"   |
| `deepseek/deepseek-flash` | 1.2s | 4.2s | 3423 | 103 | 0 | 100% | 67% | — | — | 预期不符：advance = "", want "done"   |
| `dashscope/deepseek-v4-flash` | 3.3s | 3.3s | 3423 | 148 | 0 | 100% | 67% | — | — | 预期不符：advance = "", want "done"   |
| `dashscope/qwen3.8-flash` | 800ms | 2.8s | 3441 | 94 | 0 | 100% | 100% | — | — |  |
| `dashscope/qwen3.7-plus` | 800ms | 2s | 3441 | 102 | 0 | 100% | 33% | — | — | 预期不符：advance = "", want "done"   |

### `dialogue/lite-reading-coach/direct-answer`

档：`dialogue` · 调用点：`postReadingCoachTurn (POST /readings/{id}/coach)`

| 模型 | 首字 | 总时长 | 入 tokens | 出 tokens | 其中推理 | 解析 | 预期 | 任务命中 | 质量 | 备注 |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|
| `dashscope/deepseek-v4-pro` | 1s | 3.7s | 3395 | 126 | 0 | 100% | 100% | — | — |  |
| `dashscope/deepseek-v4.1-flash` | 5.1s | 7.1s | 3396 | 148 | 0 | 100% | 100% | — | — |  |
| `deepseek/deepseek-flash` | 1s | 4.5s | 3395 | 144 | 0 | 100% | 100% | — | — |  |
| `dashscope/deepseek-v4-flash` | 3.4s | 3.4s | 3395 | 140 | 0 | 100% | 100% | — | — |  |
| `dashscope/qwen3.8-flash` | 800ms | 2.2s | 3413 | 61 | 0 | 100% | 100% | — | — |  |
| `dashscope/qwen3.7-plus` | 800ms | 1.5s | 3413 | 65 | 0 | 100% | 100% | — | — |  |

### `dialogue/lite-reading-coach/skip`

档：`dialogue` · 调用点：`postReadingCoachTurn (POST /readings/{id}/coach)`

| 模型 | 首字 | 总时长 | 入 tokens | 出 tokens | 其中推理 | 解析 | 预期 | 任务命中 | 质量 | 备注 |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|
| `dashscope/deepseek-v4-pro` | 1.2s | 2.5s | 3394 | 50 | 0 | 100% | 100% | — | — |  |
| `dashscope/deepseek-v4.1-flash` | 3s | 4.9s | 3395 | 90 | 0 | 100% | 100% | — | — |  |
| `deepseek/deepseek-flash` | 1.4s | 3.7s | 3394 | 89 | 0 | 100% | 100% | — | — |  |
| `dashscope/deepseek-v4-flash` | 2.8s | 2.8s | 3394 | 78 | 0 | 100% | 100% | — | — |  |
| `dashscope/qwen3.8-flash` | 900ms | 1.6s | 3413 | 27 | 0 | 100% | 100% | — | — |  |
| `dashscope/qwen3.7-plus` | 700ms | 1.6s | 3413 | 74 | 0 | 100% | 100% | — | — |  |

### `dialogue/lite-reading-coach/hunt-no-pick`

档：`dialogue` · 调用点：`postReadingCoachTurn (POST /readings/{id}/coach)`

| 模型 | 首字 | 总时长 | 入 tokens | 出 tokens | 其中推理 | 解析 | 预期 | 任务命中 | 质量 | 备注 |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|
| `dashscope/deepseek-v4-pro` | 1.3s | 3.1s | 3366 | 71 | 0 | 100% | 100% | — | — |  |
| `dashscope/deepseek-v4.1-flash` | 6.6s | 7.7s | 3367 | 90 | 0 | 100% | 100% | — | — |  |
| `deepseek/deepseek-flash` | 1.7s | 4.6s | 3366 | 92 | 0 | 100% | 100% | — | — |  |
| `dashscope/deepseek-v4-flash` | 6.6s | 6.6s | 3366 | 115 | 0 | 100% | 100% | — | — |  |
| `dashscope/qwen3.8-flash` | 1s | 3.4s | 3384 | 81 | 0 | 100% | 100% | — | — |  |
| `dashscope/qwen3.7-plus` | 1.2s | 2.2s | 3384 | 71 | 0 | 100% | 100% | — | — |  |

### `dialogue/lite-reading-coach/hunt-picked`

档：`dialogue` · 调用点：`postReadingCoachTurn (POST /readings/{id}/coach)`

| 模型 | 首字 | 总时长 | 入 tokens | 出 tokens | 其中推理 | 解析 | 预期 | 任务命中 | 质量 | 备注 |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|
| `dashscope/deepseek-v4-pro` | 1.4s | 5s | 3388 | 139 | 0 | 100% | 33% | — | — | 预期不符：turn must not attach a card or lens outside the next planned step   |
| `dashscope/deepseek-v4.1-flash` | 7.2s | 9.7s | 3389 | 132 | 0 | 100% | 33% | — | — | 预期不符：turn must not attach a card or lens outside the next planned step   |
| `deepseek/deepseek-flash` | 4.4s | 7.3s | 3388 | 135 | 0 | 100% | 33% | — | — | 预期不符：turn must not attach a card or lens outside the next planned step   |
| `dashscope/deepseek-v4-flash` | 4s | 4s | 3388 | 136 | 0 | 100% | 33% | — | — | 预期不符：turn must not attach a card or lens outside the next planned step   |
| `dashscope/qwen3.8-flash` | 5.4s | 9.6s | 3408 | 127 | 0 | 100% | 0% | — | — | 预期不符：turn must not attach a card or lens outside the next planned step   |
| `dashscope/qwen3.7-plus` | 900ms | 2.5s | 3408 | 132 | 0 | 100% | 33% | — | — | 预期不符：turn must not attach a card or lens outside the next planned step   |

### `dialogue/lite-reading-coach/label`

档：`dialogue` · 调用点：`postReadingCoachTurn (POST /readings/{id}/coach)`

| 模型 | 首字 | 总时长 | 入 tokens | 出 tokens | 其中推理 | 解析 | 预期 | 任务命中 | 质量 | 备注 |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|
| `dashscope/deepseek-v4-pro` | 1.2s | 6.8s | 3427 | 185 | 0 | 100% | 0% | — | — | 预期不符：turn must not attach a card or lens outside the next planned step   |
| `dashscope/deepseek-v4.1-flash` | 3.5s | 4.8s | 3428 | 155 | 0 | 100% | 33% | — | — | 预期不符：turn must not attach a card or lens outside the next planned step   |
| `deepseek/deepseek-flash` | 1.1s | 6.3s | 3427 | 162 | 0 | 100% | 33% | — | — | 预期不符：turn must not attach a card or lens outside the next planned step   |
| `dashscope/deepseek-v4-flash` | 4.5s | 4.5s | 3427 | 238 | 0 | 100% | 33% | — | — | 预期不符：turn must not attach a card or lens outside the next planned step   |
| `dashscope/qwen3.8-flash` | 900ms | 5.6s | 3448 | 108 | 0 | 100% | 33% | — | — | 预期不符：final student-visible reply ends mid-sentence   |
| `dashscope/qwen3.7-plus` | 800ms | 3.8s | 3448 | 214 | 0 | 100% | 0% | — | — | 预期不符：turn must not attach a card or lens outside the next planned step   |

