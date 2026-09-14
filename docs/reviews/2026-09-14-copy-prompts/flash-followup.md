# 阅读 coach 修复与 V4.1 Flash 复测

2026-09-14 · `codex/coach-copy-clarity` · 已将独立分支更新到 `origin/main` 的 `66e4f673`，没有合并回 main，也没有发布。

**阅读新版通过 54 次真实模型功能检查；其他 dialogue 陪练通过 33 次。全部新实验都使用 DeepSeek V4.1 Flash，并核对服务商实际返回的模型。** 下述通过指本轮输入下的解析、推进和工具协议检查，不代表所有文章、所有语言或所有教学效果均已验证。

## 为什么上次没有通过

上次候选在“帮助学生理解”和“判断当前步骤完成”之间没有划清边界：出现过回答不完整却推进、回答完整仍追问、把跳过写为完成。那次用的是 V4 Pro，不能拿来证明 V4.1 Flash 的表现。

此外，旧阅读用例曾把“找数字并说明含义”配置为 `label`。但当前产品里的 `label` 是需要真实标注板提交的任务，口头解释并不等于完成标注板。该夹具不能用来判断普通分析任务是否应推进。本轮以真实任务类型重建用例；保留旧失败记录，不将其与新对照混算。

在当前 main 和 Flash 上，修正后的 30 次阅读对照仍有 5 次失败：完整解释数字的 3 次都没有推进；通读完成 1 次未推进；已经真实选句 1 次未推进。例如学生已解释“超过一半”衡量新增装机占比，原版认可后又要求分析另一个数字“三成”，并把 `advance` 留空。这会让用户感到教练不断追加要求。

## 改了什么

1. **每轮明确当前任务的完成条件。** 在现有上下文末尾，根据当前任务类型说明何时停留、完成或跳过；普通概念提问可以直接解释，解释本身不算完成分析任务；完成原任务后不再追加第二个例子或更多证据。保留 main 新增的卡片、透镜、重试、段落定位等行为。
2. **修复一个已有推进兜底无法执行的条件。** 外层原本要求模型的 `advance` 非空，内层却在处理它为空的情况，导致“真实标注板已提交”和“连续六轮卡住”的既有规则永远进不去。已移除外层矛盾条件，实际写入仍要求有效推进值。六轮兜底是 main 已有设计，并非本次新设的完成标准；hunt 仍要求真实选句。真实标注板提交后，不再无条件补发一块新板。
3. **补测时发现并修复 PBL 计划 JSON 不闭合。** Flash 初测连续 3 次漏掉顶层结束括号，实际解析器拒绝。Prompt 增加完整的嵌套输出示例，随后全部 18 次 PBL 重测通过。没有修改解析器容错或降低判定标准。

## 实测范围

| 集合 | 场景 | 结果 |
|---|---|---:|
| main 阅读对照 | 10 个场景，每个 3 次 | 25/30；5 次推进失败 |
| 新版阅读同组 | 概念、求助、部分回答、完整回答、跳过、通读、联系经历、未选句、真实选句、选句任务跳过 | 30/30 |
| 新版阅读连续对话 | 另外两篇文章，分别涉及图书馆人数与支持比例、城市与郊区温差；每篇 3 条对话，每条 3 轮 | 18/18 |
| 新版阅读透镜 | 尚未完成、真实完成回传，各 3 次 | 6/6 |
| 新版 PBL | 示例、计划、分工、观察清单、课程、概念，各 3 次 | 18/18 |
| 自由／项目／课程陪练 | 概念答疑、项目求助、拒绝代写、课程答疑，各 3 次 | 12/12 |
| 写作规划 | 从学生已陈述的主张整理提纲 | 3/3 |

对照组与新版阅读的 30 对请求逐一比对，唯一差异是末尾新增的当前步骤判据；文章、历史、任务类型、学生输入和校验函数一致。连续对话把真实上一轮模型回复放回历史，未用手写的理想回复替代。

真实模型测试调用生产 prompt builder 与 parser，直接检查模型推进结果，未用 handler 的兜底把失败改成通过。它们没有覆盖浏览器点击、完整登录流程和线上数据库。

另以真实 HTTP handler 和临时数据库验证 5 类状态：真实标注板完成、口头说已完成但未提交、未完成的普通分析、普通任务连续卡住、未真实选句的 hunt 连续卡住。修复前两个预期失败，修复后全部通过；也检查已提交标注板不会收到无条件补发的新板。这些是固定输出的逻辑回归，不是实际模型实验，日志中的 mock 模型名称不能用作模型证据。

后端 `api / agent / pbl / routebench / vocab / cards / claritytest` 回归通过；PBL 最后一次 prompt 修改后又重跑相关回归。此轮没有重跑前端浏览器验收，旧报告的前端测试仅适用于当时版本。

## 模型和证据

显式绑定 `MODEL_DIALOGUE=deepseek/deepseek-flash`；目录对应 V4.1 Flash，实际请求模型与服务商返回的 `model` 都是 `deepseek-flash`。测试在绑定不符或回包模型不符时直接失败。只读取服务端环境文件中的必要配置，不记录密钥。未改生产模型目录和各档绑定。

所有 136 次新调用（包括 8 次失败和全部中间版）均保存于 [完整证据](flash-evidence/all-runs.jsonl.gz)。其中最终选择 87 次：阅读 54 次、其他 dialogue 33 次；没有仅挑同一版的成功样本。`dialogue` 中初测的 18 次 PBL 全部归历史，最终使用 `pbl-v2` 全组。1 次模型连通检查也独立归历史。

- [运行索引与选择标记](flash-evidence/runs.csv)、[统计](flash-evidence/summary.json)、[校验和](flash-evidence/SHA256SUMS)
- [阅读和计划的失败／修复输出示例](flash-evidence/examples.jsonl)
- [真实接口最终回归](flash-evidence/clarity-progress-final.log)、[后端回归](flash-evidence/clarity-flash-regression.log)

合计输入 761,174 tokens、输出 26,905 tokens。按目录峰时非缓存价估算约 0.261 美元；未扣缓存和闲时优惠，实际收费以服务商账单为准。未固定随机种子。输入为合成材料，不使用学生生产数据。

## 复跑

在 `apps/api` 中运行，环境文件只在服务端保存，输出必须使用新的目录：

```sh
MODEL_DIALOGUE=deepseek/deepseek-flash \
CLARITY_EXPECT_MODEL=deepseek/deepseek-flash \
CLARITY_EXPECT_WIRE_MODEL=deepseek-flash \
CLARITY_LIVE=1 CLARITY_SAMPLES=3 \
CLARITY_ENV_FILE=/absolute/path/to/server.env \
CLARITY_OUT=/absolute/path/to/new-evidence-directory \
go test ./internal/api ./internal/pbl ./internal/agent \
  -run '^(TestClarityReading|TestClarityReadingSequence|TestClarityReadingLens|TestClarityPBL|TestClarityWriting|TestClarityAgent)$/^(plan|chat-concept|course-concept|project-help|project-no-ghostwriting|concept|help|partial|done|skip|read-done|connect|hunt-no-pick|hunt-picked|hunt-skip|not-completed|completed|examples|split|observe|course|library-[123]|urban-heat-[123])$' -count=1 -v
```

连续对话测试内部固定每轮 1 次、每篇重复 3 条，避免把同一轮的独立采样伪装成连续对话。`compose / review / assess` 不在这次 Flash 对话档复测范围内；旧报告对应结果仍属于旧模型，不能宣称已在 Flash 验证。尤其过程评估必须保持旗舰模型约束。

## 仍需观察

解析和推进通过不能证明知识准确性、建议质量和语言自然程度。部分模型输出仍会带多余追问或生成被 parser 丢弃的空卡片；此次未将这些归为“文风全部通过”。本轮覆盖中文合成案例，未覆盖任意文章、英语、多用户并发和全部浏览器交互。阅读修改已保留在独立分支；旧 `reading-candidate-not-approved.patch` 继续作为历史附件，合并时不要应用。
