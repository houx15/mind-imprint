# 学生过程数据 · 数据字典 (Data Dictionary)

**面向对象：** 想用脚本读取/分析这批走查数据、为「过程评估」建模的同事。
**这份文档说明每份学生 JSON 的结构、每个字段的含义与取值，并给出重建「多版本提案/论文」「证据链」等的配方。**

数据是 9 个学生画像在生产环境走完整项目后的**全量导出**（对话 + 行为事件 + 输出物）。同题不同脑：*"Did China make the Earth become more sustainable?"*（800 词议论文）。

---

## 0. 拿到数据 / 重新生成

- **文件：** `.deploy-local/student-data/persona{2..10}-*.json`（9 份，每份 ~170–330 KB，一份 = 一个学生一个项目）。
- **导出脚本：** `.deploy-local/dump-student-data.sh`（读生产库；库无公网端口，经部署密钥 `docker exec` 进 `mindimprint-db-1` 跑 psql）。
  - `dump-student-data.sh <project_id> [out.json]` — 导一个
  - `dump-student-data.sh --email <email>` — 按邮箱找该用户对话最多的项目导出
  - `dump-student-data.sh --list-email <email>` — 列出该用户所有项目（挑 pid 用）
  - `dump-student-data.sh --all-deepwalk` — 一次导出全部 9 个走查项目
- 每份 JSON 是**一个对象**，UTF-8，时间戳均为 **UTC ISO-8601**。

### 语料一览（数字来自生产库直读）

| 文件 | 画像 | 对话(学生/印记) | 行为事件 | 修订检查点 | 论文快照 | 卡 | 文献 | 评估 |
|---|---|---|---|---|---|---|---|---|
| persona2-deepdiver | 自主深潜 | 84 (42/42) | 107 | 11 | 2 | 9 | 12 | ✓ |
| persona3-anxious | 焦虑反复 | 175 (87/88)† | 108 | 1† | 4 | 6 | 10 | ✓ |
| persona4-confirmbias | 确认偏误 | 73 (36/37) | 104 | 17 | 3 | 6 | 13 | ✓ |
| persona5-ghostwrite | 代写倾向 | 95 (47/48) | 87 | 10 | 2 | 2 | 11 | ✓ |
| persona6-ordinary | 普通务实 | 69 (34/35) | 88 | 10 | 2 | 3 | 13 | ✓ |
| persona7-divergent | 发散跳跃 | 61 (30/31) | 109 | 13 | 3 | 5 | 15 | ✓ |
| persona8-lifeexp | 生活经验 | 81 (40/41) | 95 | 9 | 1 | 3 | 16 | ✓ |
| persona9-hoarder | 囤积收藏 | 77 (38/39) | 137 | 14 | 3 | 5 | 17 | ✓ |
| persona10-search | 检索薄弱 | 60 (30/30) | 92 | 11 | 2 | 5 | 16 | ✓ |

† **persona3 有「双跑」污染**：Opus 走查中途流中断→恢复，恢复时项目已归档完成，恢复的对话仍写进了共享 studio 线程（故 175 条对话），但阅读/证据/快照类结构化写入在已归档项目上空转，故修订检查点仅 1。分析时把 persona3 当**对话/评估可用、结构化输出物不完整**处理。
**另需排除**两个脏项目（不在这 9 份里，但同账号下存在）：`c35b7637…`（persona3 诊断废弃）、`44e4ba39…`（persona7 旧残缺项目）。

---

## 1. 顶层结构

```jsonc
{
  "project":  { … },   // 项目元信息（含 studio_state：阶段/工具态/assignment brief）
  "owner":    { "email": "...", "display_name": "..." },
  "chat":     [ … ],   // 对话历史（时间序）
  "actions":  [ … ],   // 行为事件流（event 表）—— 过程评估的主料
  "revision_checkpoints": [ … ],  // 修订轨迹快照（可按 hash 差分）
  "outputs":  { … }    // 24 个输出物子段（提案/论文各版本/图谱/计划/活动日志/评估…）
}
```

三条主轴，正对「xlsx=脑、平台=手」：`chat` = 学生怎么想/被引导，`actions` = 学生做了什么，`outputs` = 学生产出了什么（及其演化）。

---

## 2. `project` / `owner`

| 字段 | 类型 | 说明 |
|---|---|---|
| `project.id` | uuid | project_id，全表外键 |
| `project.title` | string | 项目名 |
| `project.qualification` | string | 学段/资格（如 IB EE） |
| `project.status` | string | `finished`（本批全部）/ `active` / `evaluating` |
| `project.studio_state` | object | 状态机快照：`stage`(当前阶段) / `started` / `openTool` / `essayTrack` / `proposalTrack` / `resourceNeeds` / `updatedAtTurn` 等。**assignment brief 与阶段真相以此为准。** |
| `project.created_at` / `last_active_at` | ts | |
| `owner.email` / `display_name` | string | 学生账号 |

---

## 3. `chat[]` — 对话历史

每元素 = 一条消息，按 `at` 升序。

| 字段 | 类型 | 说明 |
|---|---|---|
| `at` | ts | UTC 时间戳 |
| `role` | enum | `user` = **学生**；`assistant` = **印记(AI)** |
| `surface` | enum | `studio` = 主线（一条连续的印记线程）；`find_sources` / `reflection` = 隔离子代理教练 |
| `stage` | enum | 发生时的 StudioStage（见 §7 枚举） |
| `folded` | bool | 是否已被压缩折叠（长对话的 recap 机制） |
| `content` | string | 正文（学生用中文对话、英文写论文；印记中文） |

> 说明：一次「陪练回合」在产品里横跨两条通道——**思考行（reasoning）→ 对话回合**，**动作行（action）→ 功能调用**。所以 `chat` 与 `actions[type=coach_turn]` 大体一一对应，但真正的「做了什么」在 `actions`。

---

## 4. `actions[]` — 行为事件流（重点）

来自 `event` 表，是过程评估的核心离散动作轨迹。每元素：

```jsonc
{ "at": ts, "surface": "studio", "type": "<事件类型>", "payload": { … } }
```

`payload` 形状随 `type` 而定。全部类型与其 payload：

| type | payload 关键字段 | 含义 |
|---|---|---|
| `coach_turn` | `student`, `ai`, `scope`, `cardId?` | 一次陪练回合（学生话+印记话；`scope`=房间标签） |
| `lead_added` | `leadId`, `text`, `origin`, `stage` | 在探索图谱新增一个问题节点 |
| `lead_removed` | `leadId`, `text`, `stage` | 删除问题节点（**剪枝**信号，见 persona7） |
| `lead_adopted` | `leadId`, `parentLeadId`, `source`, `stage` | 把一条 dig 出的候选采纳为某父问题下的节点 |
| `edge_added` | `from`, `to`, `origin`, `stage` | 连接两个问题节点 |
| `edge_removed` | `from`, `to`, `stage` | 删除边 |
| `dig_performed` | `keyword`, `stage` | 用关键词跑一次文献检索（弱检索者会先失败再精修） |
| `source_added` | `referenceId`, `title`, `classification`, `provenance`, `stage` | 加一条文献；`provenance`∈`manual`/`dig`/`chat_link` |
| `source_attached` | `referenceId`, `parentLeadId`, `stage` | 把来源挂到某问题节点下 |
| `source_reclassified` | `referenceId`, `field`, `before`, `after`, `stage` | 改来源某字段（`field`∈`triage`/`decision`/`classification`/证据字段…，含前后值——**分拣/归位轨迹**） |
| `card_surfaced` | `card_id`, `material_id`, `card_instance_id` | 工具卡被自动召唤 |
| `card_logged` | `cardId`, `cardInstanceId` | 工具卡被落库/填写 |
| `card_completed` | `cardId`, `objective` | 工具卡完成 |
| `card_skipped` | — | 被召唤的卡被学生跳过（**摩擦即信号**） |
| `gate_checked` | `status`, `missing[]`, `contract` | 阶段门校验（如暂定论点提交门） |
| `intervention_posted` | `intervention_id`, `anchor`, `criterion` | 印记发起一次批注/裁决（见 outputs.interventions） |
| `review_ordered` | `snapshot_id`, `items` | 对某快照跑「整稿体检」；`items`=体检批注条数 |
| `version_saved` | `snapshot_id`, `seq`, `word_count` | 提交一个论文快照（**论文版本**） |
| `revision_checkpoint` | `artifactType`, `trigger` | 记录一次修订检查点（内容见 §5） |
| `ai_use_written` | — | 保存 AI 使用诚实声明 |
| `reflection_written` | `text` | 保存反思 |
| `project_finished` | `generatedAt` | 项目完成→评估入队 |

---

## 5. `revision_checkpoints[]` — 修订轨迹（可差分）

「修订记录」功能的产物：在关键触发点对写作产物打快照，用于**看学生改了什么、AI 反馈后又改了什么**。

| 字段 | 类型 | 说明 |
|---|---|---|
| `at` | ts | |
| `artifact` | enum | `proposal` / `outline` / `snippets` / `claim` / `draft` |
| `trigger` | enum | `ask_feedback`（如整稿体检/提案体检时）/ `finish` / `advance` |
| `hash` | string | 规范化内容的 sha256（十六进制）——**两个相邻同 artifact 检查点 hash 相同 = 反馈后未改动**） |
| `content` | 见下 | 该产物在该时刻的内容 |

`content` 按 `artifact` 不同：

| artifact | content 形状 | 说明 |
|---|---|---|
| `proposal` | object `{objective, reason, activities, resources, counterpoints}` | 提案五要素当时的全文 |
| `claim` | array `[{id, text}]` | 暂定论点 / 子问题当时的列表 |
| `snippets` | array `[{id, text}]` 或 `null` | 逐论点写作片段 |
| `outline` | array 或 `null` | 大纲节点 |
| `draft` | object `{snapshotId}` | **引用**，不重复正文——用 `snapshotId` 去 `outputs.snapshots[].id` 取正文 |

**重建某产物的版本史：** 按 `artifact` 过滤、按 `at` 排序即可；正文版本另见 `outputs.snapshots`（§6）。

---

## 6. `outputs` — 输出物（24 子段）

> 现状 vs 版本史：`proposal`/`snippets`/`outline` 等**当前态**表只存最新一版；它们的**演化**在 `revision_checkpoints`。**论文多版本**在 `snapshots`（每个 seq 一版，带 `id`+`content`+字数）。

**写作产物**
| 子段 | 形状 | 说明 |
|---|---|---|
| `proposal` | object `{objective, reason, activities, resources, counterpoints, updated_at}` | 研究提案五要素（最终态） |
| `outline` | `[{text, depth, position}]` | 大纲节点（最终态） |
| `snippets` | `[{section, position, text}]` | 逐论点写作片段；`section` 形如 `claim:<id>` / `sub:intro` / `prop:<key>` |
| `snapshots` | `[{id, seq, doc_kind, at, content}]` | **论文/提案各版本正文**；`doc_kind`∈`essay`/`proposal`；按 `seq` 递增即演化顺序 |
| `buffers` | `[{doc_kind, content, updated_at}]` | 实时编辑缓冲（最新草稿态） |
| `writing_finish` | `[{doc_kind, finished_at}]` | 完成门（essay/proposal 各一） |

**文献与阅读**
| 子段 | 形状 | 说明 |
|---|---|---|
| `library` | `[{…}]` | 文献库，每条含来源元信息 + 阅读简报 + **证据标注** + 分拣，见下 |
| `materials` | `[{id, kind, source, title, source_url, blocks:[{id,text}], scratch, at}]` | **学生实际粘入/读到的来源正文**（`blocks` = 段落数组） |
| `source_log` | `[{url, title, time_spent_s, takeaway, tier, lateral_read, opened_at}]` | 阅读日志（停留时长、是否横向核查 lateral_read） |
| `collections` | `[{id, name, parent_id, position}]` | 文献分组桶（仅 persona9 有 5 个；其余 0，忠实） |

`library[]` 每条关键字段：`id, title, classification, author, year, url, journal, abstract, tags[], reading_status, reading_note, takeaway(object), triage(red/yellow/""), decision(use/maybe/drop/null), evidence_nature(support/challenge/""), evidence_argument, evidence_finding, evidence_placement, material_id, collection_id, archived`。
`takeaway` 对象：`{findings, keyQuotes, newLeads, credibility, proposalImpact}`。

**探索图谱**
| 子段 | 形状 | 说明 |
|---|---|---|
| `exploration_leads` | `[{id, text, status, origin, parent, position}]` | 问题节点（兔子洞地图的点）；`parent` 为父节点 id |
| `question_edges` | `[{from, to, label, status}]` | 问题节点之间的边 |
| `process_graph_nodes` | `[{id, type, body, author, span_ref, at}]` | 过程图节点（如挂在句子上的卡/批注；`body` 是对象） |
| `process_graph_edges` | `[{type, from_kind, from_id, to_kind, to_id}]` | 过程图边（本批多为空） |

**计划与工具卡**
| 子段 | 形状 | 说明 |
|---|---|---|
| `plan` | `[{title, tag, col, stage, ref_material_id, start_day, days, position}]` | 研究计划板；`tag`∈`read`/`write`/`review`，`col`∈`todo`/`doing`/`done` |
| `cards` | `[{card_id, status, field_values, rubric_tags, at}]` | 已填工具卡；`field_values` 形状随 `card_id`（见 §7） |
| `activity_log` | `[{date, text, source}]` | 活动日志（18–36 条/项目） |
| `interventions` | `[{type, anchor, criterion, body, level, output_check_verdict, at}]` | 印记的批注/裁决；`anchor` 定位对象；`output_check_verdict` 为产出校验结论 |

**复盘与评估**
| 子段 | 形状 | 说明 |
|---|---|---|
| `ai_use` | `{used_for, not_used_for}` | AI 使用诚实声明 |
| `reflection` | `{answers:[…], done}` | 结构化反思 |
| `assessment` | 见下 | **过程评估结果**（你要建模的目标形态参考） |
| `mirror` | object 或 null | 「你的思维印记」镜像叙述（分段+carry-forwards） |
| `summary` | object 或 null | 项目摘要叙述 |
| `conversation_digests` | `[{prose, turns_folded, at}]` | 长对话压缩摘要 |

`assessment` 对象：`{status, trigger, created_at, narrative, scores, signals, leaps, rubric}`；其中 `scores` 含 `depthAxis[]`（D1–D6，每项 `{code,name,level,evidence}`，`level`∈L1–L4）、`autonomyAxis[]`（A1–A6，0–5）、`promptLens`、`guidance.nextSteps[]`、`axiom`（"两轴永不合成总分"）。**双轴永不合成总分；单次会话为事件级证据。**

---

## 7. 枚举与词表

- **stage (StudioStage)：** `topic_discussion` 立题讨论 · `proposal_forming` 提案成形 · `plan_generation` 生成计划 · `proposal_writing` 写提案 · `proposal_review` 提案体检 · `body_writing` 写正文 · `retrospective` 复盘。
- **surface：** `studio`（主线一条印记线程）· `find_sources` · `reflection`（隔离子代理）。
- **role：** `user`=学生 · `assistant`=印记。
- **triage：** `red` / `yellow` / `""`。 **decision：** `use` / `maybe` / `drop` / `null`。
- **evidence_nature：** `support` / `challenge` / `""`。
- **provenance (source_added)：** `manual` / `dig` / `chat_link`。
- **doc_kind：** `essay` / `proposal`。
- **plan：** `tag`∈`read`/`write`/`review`；`col`∈`todo`/`doing`/`done`。
- **常见 card_id → field_values 键：** `search-plan`→`{goal,avoid,channels,keywords}` 或 `{plan,sources,keywords,question}`；`metacognition`→`{noticed,adjusted,nextTime}`；`fact-opinion-value`→`{fact,opinion,value}`；`concession`→`{opposing,concede}`；`perspective-matrix` / `knower-perspective` / `rabbit-hole` / `argument-map` / `pee` / `question-card` 各有自己的键（以实际 `field_values` 为准，勿硬编码）。

---

## 8. 重建配方（Recipes）

- **论文的多版本演化：** `outputs.snapshots` 按 `seq` 升序；相邻 `content` 做 diff / 字数曲线。想对齐「AI 反馈前后」用 `revision_checkpoints[artifact=draft].snapshotId` join `snapshots[].id`，再看 `trigger=ask_feedback` 前后哪个快照变了。
- **提案的多版本演化：** `revision_checkpoints[artifact=proposal]` 按 `at` 排序，逐版比 `objective/reason/…`；最终态在 `outputs.proposal`。
- **"反馈后没改" 检测：** 同一 `artifact` 相邻检查点 `hash` 相同 = 学生忽略了反馈（负信号）。
- **证据链/来源功能：** `library[]` 的 `evidence_nature/argument/finding/placement` + `triage` + `decision`；配 `source_reclassified` 事件看归位轨迹；配 `materials` 看实际读了什么、`source_log.lateral_read` 看是否横向核查。
- **探索广度→收敛：** `actions` 里 `lead_added` vs `lead_removed`/`edge_removed`（发散→剪枝，见 persona7）；`dig_performed.keyword` 序列看检索精修（见 persona10）。
- **铁律① 自查（AI 是否代写正文）：** 扫 `chat[role=assistant]`（或 `actions[coach_turn].ai`）中出现整段可直接抄用的正文；本批 persona2 有一例——见 `docs/2026-08-12-persona-walks-report.md` §3。

---

## 9. 最小可用示例（Python，仅标准库）

```python
import json, glob, os
from collections import Counter

for f in sorted(glob.glob(".deploy-local/student-data/persona*.json")):
    d = json.load(open(f))
    chat, acts, o = d["chat"], d["actions"], d["outputs"]

    # 1) 对话规模（学生/印记）
    n_student = sum(1 for m in chat if m["role"] == "user")

    # 2) 行为直方图
    hist = Counter(a["type"] for a in acts)

    # 3) 论文版本字数曲线
    essays = [s for s in o["snapshots"] if s["doc_kind"] == "essay"]
    essays.sort(key=lambda s: s["seq"])
    words = [len((s["content"] or "").split()) for s in essays]

    # 4) 提案版本史（含最终态）
    prop_versions = [c["content"] for c in d["revision_checkpoints"] if c["artifact"] == "proposal"]

    # 5) 证据标注 & 深度轴
    tagged = [r for r in o["library"] if (r.get("evidence_nature") or "").strip()]
    depth = {x["code"]: x["level"] for x in (o["assessment"]["scores"]["depthAxis"])}

    print(os.path.basename(f), "| student turns", n_student,
          "| essay words", words, "| proposal versions", len(prop_versions),
          "| evidence", len(tagged), "| depth", depth)
```

---

## 10. 注意事项

- 时间戳均 UTC。跨段对齐用 `at`（`chat`/`actions`/`checkpoints` 同源时钟）。
- persona3 结构化输出物不完整（见 §0 †）；证据标注为**事后按各画像报告忠实回填**（原走查脚本字段名有误导致空写，产品前端本身正确，学生真实路径不受影响）。
- `library.credibility` 等个别字段可能为 `null`（未填即空），做统计前判空。
- 只读导出：脚本只跑 SELECT，不改生产数据。
- 相关文档：走查总报告 `docs/2026-08-12-persona-walks-report.md`；存储指南 `docs/2026-08-11-evaluation-data-storage-guide.md`；代码地图 `docs/2026-08-11-evaluation-and-teacher-code-map.md`。
```
