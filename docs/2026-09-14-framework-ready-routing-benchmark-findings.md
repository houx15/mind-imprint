# Framework ready prompt 与路由校准

2026-09-14 · `routebench` 最终结果 · 判官 `dashscope/qwen3.7-max`

## 目标

本轮校准 `frameworkReviewSystem`：`ready` 表示现有学生输入足以派生初步计划，AI 无需
发明研究范围、主要证据方向或核心分析方法。它不是正式提案验收，也不阻止学生生成计划。
完整行为定义见 `docs/2026-08-09-all-statuses.md`。

生产 prompt 将判断写成五个独立门槛：

`ready = objective_ok AND activities_ok AND resources_ok AND coherence_ok AND reason_ok`

目标只能依据目标字段判断；反例不进入公式，缺失或跳过不能单独导致 `false`。输出前还要
检查 verdict、理由和建议是否一致。

## 验证方式

专用配置：`apps/api/cmd/routebench/routebench-framework-ready.json`；每个模型、每个用例
运行 3 个样本。四个用例覆盖两项负例与两项正例：

- `review/framework-review`：活动只是日程、资源只是渠道，预期 `false`；
- `review/framework-vague-objective`：目标只是复述题目，预期 `false`；
- `review/framework-minimum-ready`：已有比较方法和可靠来源类型、反例为空，预期 `true`；
- `review/framework-strong-ready`：目标、方法、来源方向和反例检验完整，预期 `true`。

routebench 分开记录三层结果：生产解析器能否接收、逐样本 gold verdict 是否命中、判官对
理由和建议的质量评分。结构或 gold 任一样本失败都会淘汰候选。

## 最终结果

| 候选 | 结构 | gold | 判官最低分 | 四格 p50 |
|---|---:|---:|---:|---|
| deepseek-v4-pro | 100% | 100% | 4 | 18.2 / 11.2 / 11.6 / 14.3s |
| qwen3.8-max | 100% | 100% | 5 | 38.9 / 21.9 / 29.3 / 23.5s |
| glm-5.3 | 100% | 100% | 5 | 4.5 / 3.5 / 3.7 / 3.4s |
| glm-5.2 | 100% | 100% | 5 | 25.2 / 15.4 / 17.0 / 19.2s |

四个候选均通过结构与 gold 硬门槛。GLM-5.3、qwen3.8-max 和 GLM-5.2 的最低质量与平均
质量并列，GLM-5.3 的延迟最低，因此按“最差质量 → 平均质量 → p50 延迟 → token”的统一
排序规则胜出。只有所有排序指标完全相同才保留现有绑定。

## 结论

`review` 绑定为 `dashscope/glm-5.3`。`review` 档写 `reasoning: default`，解析后采用该模型
的默认 **low** effort。本轮生产 prompt 为 979 个字符。

当前四个用例足以支持本次阶段性优化，但覆盖面仍然有限。扩大场景、增加盲测模型和人工
校准新用例留作后续工作；在扩充 benchmark 前，不把本轮结果解释为对所有 framework 输入
的全面证明。
