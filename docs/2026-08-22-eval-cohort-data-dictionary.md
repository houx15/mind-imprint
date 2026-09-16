# 学生过程数据 · 数据字典 (Data Dictionary) — 2026-08-22 评估 cohort

**面向对象：** 想用脚本读取/分析这批走查数据、为「过程评估」建模的同事。
**这份文档只描述「评估输入」一侧**（学生过程数据）。**评估输出（`EvaluationReport`）不在 datapack 里**，作为独立文件另放 `reports/`，见 §6 末尾。

数据是 10 个学生画像在生产环境走完整项目后的**全量导出**（对话 + 行为事件 + 输出物），一份 = 一个学生一个项目。同题不同脑：*"To what extent is China making the world more environmentally sustainable?"*（约 800 词议论文）。

> 这批与 2026-08-12 那批（`docs/2026-08-12-persona-walk-data-dictionary.md`）**输入侧结构 1:1 相同**。唯一差别：那批旧的评估子段 `assessment`/`mirror`/`summary`（已退役表）在本批**不存在**——评估以独立的 `EvaluationReport` v1 文件交付，不再塞进 datapack。若你已读过 08-12 那份，本文档只需看 §0、§6 末尾、§8。

---

## 0. 拿到数据

三样东西，沿「输入 / 输出 / 账号」切开，都在 `.deploy-local/eval-cohort-2026-08-21/`（git-ignored）：

| 目录/文件 | 是什么 | 本文档是否描述 |
|---|---|---|
| `datapacks/personaN-<slug>.datapack.json` ×10 | **评估输入**：一个学生一个项目的全量过程数据 | ✅ 本文档 |
| `reports/personaN-<slug>.report.json` ×10 | 评估输出：`EvaluationReport` v1（envelope `{status, report}`） | ❌ 见契约 `packages/contracts/src/evaluationReport.ts` |
| `accounts.json` + `accounts/personaN-<slug>.account.json` ×10 | 每个学生的账号信息 + 指向上面两个文件的指针 | ✅ §8 |

- 每份 datapack 是**一个对象**，UTF-8，时间戳均为 **UTC ISO-8601**。
- **导出脚本：** `.deploy-local/dump-student-data.sh <project_id> [out.json]`（读生产库；库无公网端口，经部署密钥 `docker exec` 进 `mindimprint-db-1` 跑 psql，只读 SELECT）。**注意：** 该脚本导出的 `outputs` 会含 `evaluation_report` 子段；本 cohort 的 datapack 已刻意剥除（保持纯输入）。
- **生成方法（本 cohort 特有）：** 先用 Playwright 真实走查 persona1 录下全部 `/api/v1/*` 调用作为权威动作图，再用 `replay-driver.sh` 忠实复现同一端点序列/形状（含「写提案→写正文」两阶段），每个 persona 的对话内容取自 `docs/reference/学生画像模拟+报告/全部对话/` 真实脚本；教练回合走真实 LLM。

### 语料一览（数字来自导出文件直读）

| # | 文件 | 画像 | 对话(学生/印记) | 行为事件 | 修订检查点 | 论文快照 | 卡 | 文献 | project id |
|---|---|---|---|---|---|---|---|---|---|
| 1 | evidence-solid | 证据论证扎实 | 63 (31/32) | 79 | 34 | 2 | 2 | 2 | 8a7dc7e9 |
| 2 | deepdiver | 自主深潜 | 65 (32/33) | 82 | 34 | 2 | 3 | 2 | a91866d6 |
| 3 | anxious | 谨慎证据焦虑 | 69 (34/35) | 84 | 37 | 2 | 1 | 2 | a356e485 |
| 4 | confirmbias | 结论先行确认偏误 | 67 (33/34) | 87 | 37 | 2 | 2 | 3 | fde49854 |
| 5 | ghostwrite | 产出导向代写依赖 | 69 (34/35) | 81 | 34 | 2 | 1 | 2 | 37dbfbfe |
| 6 | ordinary | 普通任务完成 | 69 (34/35) | 88 | 37 | 2 | 2 | 3 | 1d8d6064 |
| 7 | divergent | 兴趣发散跑题 | 75 (37/38) | 90 | 37 | 2 | 1 | 3 | a9e7a1c4 |
| 8 | lifeexp | 生活经验外推 | 65 (32/33) | 84 | 34 | 2 | 3 | 3 | 07f3792a |
| 9 | hoarder | 材料搬运 | 65 (32/33) | 87 | 34 | 2 | 1 | 5 | 1a800ff6 |
| 10 | search | 检索表达薄弱 | 65 (32/33) | 84 | 34 | 2 | 2 | 2 | 401c8b09 |

> 全部 10 份结构完整（无 08-12 persona3 那类「双跑」污染）。深度与 08-12 手走批相当：对话 63–75、行为 79–90、检查点 34–37。

---

## 1. 顶层结构（输入侧，6 键）

```jsonc
{
  "project":  { … },   // 项目元信息（含 studio_state：阶段/工具态/assignment brief）
  "owner":    { "email": "...", "display_name": "..." },
  "chat":     [ … ],   // 对话历史（时间序）
  "actions":  [ … ],   // 行为事件流（event 表）—— 过程评估的主料
  "revision_checkpoints": [ … ],  // 修订轨迹快照（可按 hash 差分）
  "outputs":  { … }    // 21 个输入侧输出物子段（提案/论文各版本/图谱/计划/活动日志/复盘…）
}
```

三条主轴，正对「脑 / 手 / 产出」：`chat` = 学生怎么想/被引导，`actions` = 学生做了什么，`outputs` = 学生产出了什么（及其演化）。**没有 `evaluation_report` 顶层键——评估输出在 `reports/`。**

---

## 2. `project` / `owner`

| 字段 | 类型 | 说明 |
|---|---|---|
| `project.id` | uuid | project_id，全表外键 |
| `project.title` | string | 项目名 |
| `project.qualification` | string | 学段/资格 |
| `project.status` | string | `finished`（本批全部） |
| `project.studio_state` | object | 状态机快照：`stage` / `started` / `openTool` / `essayTrack` / `proposalTrack` / `resourceNeeds` / `updatedAtTurn` 等。**assignment brief 与阶段真相以此为准。** |
| `project.created_at` / `last_active_at` | ts | |
| `owner.email` / `display_name` | string | 学生账号（完整账号信息见 §8 `accounts.json`） |

---

## 3. `chat[]` — 对话历史

每元素 = 一条消息，按 `at` 升序。

| 字段 | 类型 | 说明 |
|---|---|---|
| `at` | ts | UTC 时间戳 |
| `role` | enum | `user` = **学生**；`assistant` = **印记(AI)** |
| `surface` | enum | `studio` = 主线（一条连续印记线程）；`find_sources` / `reflection` = 隔离子代理教练 |
| `stage` | enum | 发生时的 StudioStage（见 §7） |
| `folded` | bool | 是否已被压缩折叠（长对话 recap 机制） |
| `content` | string | 正文（学生用中文对话、英文写论文；印记中文） |

> 一次「陪练回合」横跨两条通道：**思考行→对话回合**、**动作行→功能调用**。`chat` 与 `actions[type=coach_turn]` 大体一一对应，但真正「做了什么」在 `actions`。

---

## 4. `actions[]` — 行为事件流（重点）

来自 `event` 表，是过程评估的核心离散动作轨迹。每元素：

```jsonc
{ "at": ts, "surface": "studio", "type": "<事件类型>", "payload": { … } }
```

`payload` 形状随 `type` 而定。全部类型：

| type | payload 关键字段 | 含义 |
|---|---|---|
| `coach_turn` | `student`, `ai`, `scope`, `cardId?` | 一次陪练回合 |
| `lead_added` / `lead_removed` / `lead_adopted` | `leadId`, `text`, `origin`/`parentLeadId`, `stage` | 探索图谱增/删/采纳问题节点（删=剪枝信号） |
| `edge_added` / `edge_removed` | `from`, `to`, `origin`, `stage` | 连/删问题节点间的边 |
| `dig_performed` | `keyword`, `stage` | 用关键词跑一次文献检索 |
| `source_added` | `referenceId`, `title`, `classification`, `provenance`, `stage` | 加一条文献；`provenance`∈`manual`/`dig`/`chat_link` |
| `source_attached` | `referenceId`, `parentLeadId`, `stage` | 把来源挂到某问题节点下 |
| `source_reclassified` | `referenceId`, `field`, `before`, `after`, `stage` | 改来源某字段（含前后值——分拣/归位轨迹） |
| `card_surfaced` / `card_logged` / `card_completed` / `card_skipped` | `card_id`/`cardId`, `cardInstanceId`, `objective` | 工具卡被召唤/落库/完成/跳过（跳过=摩擦即信号） |
| `gate_checked` | `status`, `missing[]`, `contract` | 阶段门校验 |
| `intervention_posted` | `intervention_id`, `anchor`, `criterion` | 印记发起批注/裁决（见 `outputs.interventions`） |
| `review_ordered` | `snapshot_id`, `items` | 对某快照跑整稿体检 |
| `version_saved` | `snapshot_id`, `seq`, `word_count` | 提交一个论文快照（论文版本） |
| `revision_checkpoint` | `artifactType`, `trigger` | 记录一次修订检查点（内容见 §5） |
| `ai_use_written` / `reflection_written` | `text?` | 保存 AI 使用声明 / 反思 |
| `project_finished` | `generatedAt` | 项目完成→评估入队 |

---

## 5. `revision_checkpoints[]` — 修订轨迹（可差分）

在关键触发点对写作产物打快照，用于**看学生改了什么、AI 反馈后又改了什么**。

| 字段 | 类型 | 说明 |
|---|---|---|
| `at` | ts | |
| `artifact` | enum | `proposal` / `outline` / `snippets` / `claim` / `draft` |
| `trigger` | enum | `ask_feedback` / `finish` / `advance` |
| `hash` | string | 规范化内容的 sha256——**相邻同 artifact 检查点 hash 相同 = 反馈后未改动** |
| `content` | 见下 | 该产物在该时刻的内容 |

`content` 按 `artifact` 不同：`proposal`→object `{objective, reason, activities, resources, counterpoints}`；`claim`→array `[{id,text}]`；`snippets`→array 或 `null`；`outline`→array 或 `null`；`draft`→object `{snapshotId}`（**引用**，用 `snapshotId` 去 `outputs.snapshots[].id` 取正文）。

**重建版本史：** 按 `artifact` 过滤、按 `at` 排序；论文正文版本另见 `outputs.snapshots`（§6）。

---

## 6. `outputs` — 输入侧输出物（21 子段）

> 现状 vs 版本史：`proposal`/`snippets`/`outline` 等**当前态**表只存最新一版；它们的**演化**在 `revision_checkpoints`。**论文多版本**在 `snapshots`（每个 seq 一版）。

**写作产物**
| 子段 | 形状 | 说明 |
|---|---|---|
| `proposal` | object `{objective, reason, activities, resources, counterpoints, updated_at}` | 研究提案五要素（最终态） |
| `outline` | `[{text, depth, position}]` | 大纲节点（最终态） |
| `snippets` | `[{section, position, text}]` | 逐论点写作片段；`section` 形如 `claim:<id>` / `sub:intro` / `prop:<key>` |
| `snapshots` | `[{id, seq, doc_kind, at, content}]` | **论文/提案各版本正文**；`doc_kind`∈`essay`/`proposal`；`seq` 递增即演化顺序 |
| `buffers` | `[{doc_kind, content, updated_at}]` | 实时编辑缓冲（最新草稿态） |
| `writing_finish` | `[{doc_kind, finished_at}]` | 完成门（essay/proposal 各一） |

**文献与阅读**
| 子段 | 形状 | 说明 |
|---|---|---|
| `library` | `[{…}]` | 文献库，每条含来源元信息 + 阅读简报 + **证据标注** + 分拣，见下 |
| `materials` | `[{id, kind, source, title, source_url, blocks:[{id,text}], scratch, at}]` | **学生实际粘入/读到的来源正文**（`blocks`=段落数组） |
| `source_log` | `[{url, title, time_spent_s, takeaway, tier, lateral_read, opened_at}]` | 阅读日志（停留时长、是否横向核查） |
| `collections` | `[{id, name, parent_id, position}]` | 文献分组桶 |

`library[]` 每条关键字段：`id, title, classification, author, year, url, journal, abstract, tags[], reading_status, reading_note, takeaway(object), triage(red/yellow/""), decision(use/maybe/drop/null), evidence_nature(support/challenge/""), evidence_argument, evidence_finding, evidence_placement, material_id, collection_id, archived`。`takeaway`：`{findings, keyQuotes, newLeads, credibility, proposalImpact}`。

**探索图谱**
| 子段 | 形状 | 说明 |
|---|---|---|
| `exploration_leads` | `[{id, text, status, origin, parent, position}]` | 问题节点（兔子洞地图的点） |
| `question_edges` | `[{from, to, label, status}]` | 问题节点之间的边 |
| `process_graph_nodes` | `[{id, type, body, author, span_ref, at}]` | 过程图节点（挂在句子上的卡/批注） |
| `process_graph_edges` | `[{type, from_kind, from_id, to_kind, to_id}]` | 过程图边（本批多为空） |

**计划与工具卡**
| 子段 | 形状 | 说明 |
|---|---|---|
| `plan` | `[{title, tag, col, stage, ref_material_id, start_day, days, position}]` | 研究计划板；`tag`∈`read`/`write`/`review`，`col`∈`todo`/`doing`/`done` |
| `cards` | `[{card_id, status, field_values, rubric_tags, at}]` | 已填工具卡；`field_values` 形状随 `card_id`（见 §7） |
| `activity_log` | `[{date, text, source}]` | 活动日志 |
| `interventions` | `[{type, anchor, criterion, body, level, output_check_verdict, at}]` | 印记的批注/裁决 |

**复盘（诚实声明与反思）**
| 子段 | 形状 | 说明 |
|---|---|---|
| `ai_use` | `{used_for, not_used_for}` | AI 使用诚实声明 |
| `reflection` | `{answers:[…], done}` | 结构化反思 |
| `conversation_digests` | `[{prose, turns_folded, at}]` | 长对话压缩摘要 |

> **评估输出不在这里。** 08-12 那批的 `assessment` / `mirror` / `summary` 三个子段（已退役）在本批**不存在**。本 cohort 的评估结果作为独立文件放 `reports/personaN-<slug>.report.json`，形状是 `EvaluationReport` v1（envelope `{status, report}`），契约见 **`packages/contracts/src/evaluationReport.ts`**：`version, reportId, projectId, student, basics, abstract, events[], materials[], depth[D1–D6·level 1–4], autonomy[A1–A6·band 0–5], promptLens, toolUsage[], risks[], generatedAt`。**双轴永不合成总分。**

---

## 7. 枚举与词表

- **stage (StudioStage)：** `topic_discussion` 立题讨论 · `proposal_forming` 提案成形 · `plan_generation` 生成计划 · `proposal_writing` 写提案 · `proposal_review` 提案体检 · `body_writing` 写正文 · `retrospective` 复盘。
- **surface：** `studio` · `find_sources` · `reflection`。 **role：** `user`=学生 · `assistant`=印记。
- **triage：** `red`/`yellow`/`""`。 **decision：** `use`/`maybe`/`drop`/`null`。 **evidence_nature：** `support`/`challenge`/`""`。
- **provenance：** `manual`/`dig`/`chat_link`。 **doc_kind：** `essay`/`proposal`。 **plan：** `tag`∈`read`/`write`/`review`，`col`∈`todo`/`doing`/`done`。
- **常见 card_id → field_values 键：** `search-plan`→`{goal,avoid,channels,keywords}` 或 `{plan,sources,keywords,question}`；`metacognition`→`{noticed,adjusted,nextTime}`；`fact-opinion-value`→`{fact,opinion,value}`；`concession`→`{opposing,concede}`；其余卡各有自己的键（以实际 `field_values` 为准，勿硬编码）。

---

## 8. `accounts.json` / `accounts/` — 账号信息

一个组合索引 `accounts.json` + 每人一份 `accounts/personaN-<slug>.account.json`。共享事实（班级/密码/域名/题目）在 `accounts.json` 顶部只写一次；`students[]` 里每人一行。

- 全部 10 个账号在 `https://mind-web.uni-robot.cn` 登录，密码 `Test123456`，同属 **Demo Class**（join code `DEMO-0001`），邮箱域 `eval0821.mindimprint.cn`。
- 每份 account 文件字段：`persona, slug, display_name, account{email, password, class, class_join_code, login_url}, project{id, title, status, created_at}, evaluation{depth_levels, autonomy_bands, report_endpoint}, files{evaluation_input_datapack, evaluation_report}`。
- `files.*` 是相对本目录的指针，把一个账号一步连到它的**输入 datapack** 与**输出 report**。

---

## 9. 重建配方 & 最小示例

- **论文多版本演化：** `outputs.snapshots` 按 `seq` 升序做 diff / 字数曲线；对齐「AI 反馈前后」用 `revision_checkpoints[artifact=draft].snapshotId` join `snapshots[].id`，再看 `trigger=ask_feedback` 前后哪个快照变了。
- **提案多版本演化：** `revision_checkpoints[artifact=proposal]` 按 `at` 排序逐版比五要素；最终态在 `outputs.proposal`。
- **「反馈后没改」检测：** 同一 `artifact` 相邻检查点 `hash` 相同 = 学生忽略了反馈（负信号）。
- **证据链/来源：** `library[]` 的 `evidence_nature/argument/finding/placement` + `triage` + `decision`；配 `source_reclassified` 看归位轨迹，配 `materials` 看实际读了什么、`source_log.lateral_read` 看是否横向核查。
- **探索广度→收敛：** `actions` 里 `lead_added` vs `lead_removed`/`edge_removed`；`dig_performed.keyword` 序列看检索精修。
- **铁律① 自查（AI 是否代写正文）：** 扫 `chat[role=assistant]`（或 `actions[coach_turn].ai`）中是否出现整段可直接抄用的正文。

```python
import json, glob, os
from collections import Counter

for f in sorted(glob.glob(".deploy-local/eval-cohort-2026-08-21/datapacks/persona*.json")):
    d = json.load(open(f)); chat, acts, o = d["chat"], d["actions"], d["outputs"]
    n_student = sum(1 for m in chat if m["role"] == "user")
    hist = Counter(a["type"] for a in acts)
    essays = sorted((s for s in o["snapshots"] if s["doc_kind"] == "essay"), key=lambda s: s["seq"])
    words = [len((s["content"] or "").split()) for s in essays]
    prop_versions = [c["content"] for c in d["revision_checkpoints"] if c["artifact"] == "proposal"]
    tagged = [r for r in o["library"] if (r.get("evidence_nature") or "").strip()]
    print(os.path.basename(f), "| student turns", n_student,
          "| essay words", words, "| proposal versions", len(prop_versions),
          "| evidence", len(tagged))
    # 评估分数不在 datapack；如需 → 读同名 reports/ 文件的 report.depth / report.autonomy
```

---

## 10. 注意事项

- 时间戳均 UTC。跨段对齐用 `at`（`chat`/`actions`/`checkpoints` 同源时钟）。
- **datapack 是纯输入**，不含任何评估结果；要评分去 `reports/`（或 account 文件的 `evaluation.depth_levels/autonomy_bands`）。
- `library.credibility` 等个别字段可能为 `null`，统计前判空。
- 只读导出：脚本只跑 SELECT，不改生产数据。
- 相关文档：08-12 数据字典 `docs/2026-08-12-persona-walk-data-dictionary.md`（输入侧同构参考）；评估报告契约 `packages/contracts/src/evaluationReport.ts`；本目录 `README.md` + `SUMMARY.md`（D/A 汇总表）。
