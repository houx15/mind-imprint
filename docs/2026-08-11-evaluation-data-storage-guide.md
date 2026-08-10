# 评估数据落库指南 · 一个学生的全流程数据存在哪里

> 给要做「过程评估」的人看：学生与 AI 的**每一次互动**、每一个**选择**、工具卡使用、探索、写作、回顾，分别落在**哪张表、哪些列**，以及怎么按 `project_id` 把一个学生一次任务的全部过程拉出来。列名来自 prod 实测（`information_schema`），不是旧架构文档。

**一句话心智模型：** 一切以 **`project`（一次任务）** 为中心。identity/org 在 `users/classes` 侧；这次任务的 **AI 对话**在 `chat_thread/chat_message`；**结构化选择/产物**散在若干专表（proposal、plan、exploration_lead、reference、snippet、evaluations…）；**过程信号与账单**在 `event / activity_log_entry / llm_call`。绝大多数表都带 `project_id`，这是拉全量数据的主键。

---

## 0. 身份与组织（谁在做）

| 表 | 关键列 | 存的是 |
|---|---|---|
| `users` | `id, email, role, school_id, display_name, card_theme` | 账号。`role` = student/teacher/admin。每个账号必属于某学校。 |
| `schools` | `id, name` | 学校。 |
| `classes` | `id, school_id, name, join_code` | 班级 + 注册用的 join code。 |
| `enrollments` | `user_id, class_id, role_in_class` | 用户↔班级（多对一以上）。 |

评估要按人/班/校聚合时从这里 join。**一次具体任务的过程数据不在这里**——在下面各表，用 `project.user_id` 回连到人。

---

## 1. 任务本体：`project`

| 列 | 含义 |
|---|---|
| `id` | 项目主键 —— **所有过程表的外键**。 |
| `user_id` | 归属学生。 |
| `qualification` | 学科/项目类型（EE / TOK / AP…），决定 rubric 与 AI 角色。 |
| `title` | 研究题目（新架构里 = 驱动问题的兜底来源）。 |
| `status` | 生命周期状态（active / evaluating / …）。 |
| `studio_state` | **jsonb** —— 印记编排器的运行态（当前 stage、proposal track、resource needs、started 等）。**这是「学生走到哪、印记在带他做什么」的实时快照。** |
| `deadline, created_at, last_active_at, cover` | 元数据。 |

> `studio_state` 是理解一次会话「编排轨迹」的核心：它记录 flow status / studio stage（topic_discussion→proposal_forming→plan_generation→proposal_writing/review→body_writing→retrospective）。评估要重建「学生在每个阶段停留、如何推进」时读它。

---

## 2. 学生 ↔ AI 的对话（**AI 互动历史的主表**）

一次任务里那条**连续的「印记」对话**（跨立项/阅读/写作/回顾各房间）就是评估最想要的「详细互动」。

| 表 | 关键列 | 说明 |
|---|---|---|
| `chat_thread` | `id, user_id, seeded_project_id, title` | 一条印记线程。`seeded_project_id` 关到项目。 |
| `chat_message` | `id, thread_id, role, content, surface, stage, modality, attachments, quoted_fragment, folded_at, created_at` | **每一轮消息一行。** |

`chat_message` 的列正是「互动细节」：
- `role` —— user / assistant（谁说的）。
- `content` —— 消息正文（学生问什么、印记答什么）。
- `surface` —— 这轮发生在**哪个房间/界面**（立项 / 阅读 / 写作 / 回顾…），so 你能区分「在写作面问 AI」vs「在阅读室问 AI」。
- `stage` —— 发生时的 studio stage（migration 0060 加的），把每轮对话钉到流程阶段。
- `quoted_fragment` —— 学生**引用了正文的哪一句**来问（写作面「就这句问印记」）。
- `folded_at` —— 该轮是否被压缩（长对话 compaction）。压缩后的摘要在 `conversation_digest.prose`。

> **「学生在哪里请求了 AI 帮助」** = 在 `chat_message` 里按 `role='user'` + `surface`/`stage` 分组即可看到：他在阅读室问了几次、在写作面就哪几句求助、在回顾里问了什么。

**遗留：** `messages`（关 `task_id`，旧任务面）已冻结、不再新写；老数据仍在，评估的 `llm_usage` 视图会 UNION 它。新数据一律走 `chat_message`。

---

## 3. 立项：提案 + 计划 + 大纲

| 表 | 关键列 | 存的是 |
|---|---|---|
| `project_proposal` | `project_id, objective, reason, activities, resources, counterpoints` | 研究提案的结构化四要素 + 反例/张力（`counterpoints`，0056）。**一个项目一行**，随写随更新。 |
| `plan_item` | `project_id, title, tag, col, stage, ref_material_id, start_day, days, position` | 研究**计划**的每个条目（看板式）。系统从框架/研究问题自动派生（确定性步骤，不设学生确认门槛）。 |
| `outline_node` | `project_id, text, depth, position` | 论文**大纲**树（由主问题 + 2–4 子问题初始化，学生据材料修改）。`depth` 投影层级。 |
| `snippet`（section `prop:*`） | 见 §6 | 提案各部分的正文片段（引导式写作时）。 |

---

## 4. 阅读与探索（兔子洞地图 + 来源）

这是「学生怎么找、读、判断来源」的全部落点。

| 表 | 关键列 | 存的是 |
|---|---|---|
| `exploration_lead` | `project_id, text, status, origin, parent_lead_id, connected_reference_id, source_reference_id, position` | **兔子洞图谱的节点。** `connected_reference_id=NULL` → 问题节点（`parent_lead_id=NULL` 是主问题，非空是子问题）；`connected_reference_id` 有值 → 文献节点，挂在某问题下。`origin` = takeaway/manual/guide/note（这个问题/线索**怎么来的**）。`status` = open/connected/pruned。 |
| `question_edge` | `project_id, from_lead_id, to_lead_id, label, status` | 问题之间的**有向带标签关系**（子问题/支持/反驳-张力/细化/依赖）。`status` = proposed（印记提议未确认）/ confirmed（学生确认或自己连）。**「印记提议→学生确认」的选择痕迹就在这里。** |
| `reference` | `project_id, title, author, year, journal, url, abstract, classification, credibility, decision, triage, evidence_*, reading_status, material_id, archived, tags` | **每一条来源的完整档案。** `decision` = use/maybe/drop（学生对来源的取舍判断）；`triage`/`credibility`/`evidence_nature/argument/finding/placement`（0062）= 学生对**证据功能**的标注；`reading_status` = to_read/reading/done；`material_id` 非空 = 真读过（engaged）。 |
| `material` | `project_id, kind, source, title, source_url, blocks, scratch, thread_id` | 一篇来源**被读进来的正文/材料**（分块 `blocks`），阅读室逐句共读的底料。 |
| `collection` | `project_id, name, parent_id, position` | 来源的合集/文件夹（与「未归类」的图谱归属是两个维度）。 |
| `reading_note` | `project_id, ...` (0043) | 学生在阅读室写的**自己的笔记**。 |
| `source_log_entry` | `project_id, url, title, time_spent_s, takeaway, tier, lateral_read, opened_at, material_id` | **来源日志**：打开了哪个链接、停留多久、横向核查了没（`lateral_read`）、一句话收获、可信度分层（`tier`）。评估「检索/核查行为」的金矿。 |

> **来源归位（2026-08-11 新）**：学生自加的来源若没挂到任何问题下 → 落进「未归类」节点（= `reference` 存在但无非-pruned 的 `connected_reference_id` lead）。挂载动作 = 新建一条 `origin='manual'` 的连接 lead。印记的归属建议是一次 `llm_call(purpose='suggest_placement')`。

---

## 5. 工具卡使用

| 表 | 关键列 | 存的是 |
|---|---|---|
| `card_instances` | `project_id, card_id, status, field_values, event_trace, rubric_tags, anchors, framework_fill, thread_id, created_at, completed_at` | **每一次工具卡的召唤与作答。** `card_id` = 哪张卡；`status` = 生命周期（提议/进行/完成/跳过）；`field_values` = 学生填的内容（jsonb）；`event_trace` = 卡内交互事件序列（**这是「学生怎么用这张卡」的逐步记录**）；`rubric_tags` = 评估打点。**学生跳过卡、让 AI 直接答，也会留一行**（摩擦即信号）。 |
| `card_competence` | `user_id, card_id, scaffold_state, unprompted_count, prompted_count` | **跨项目的卡熟练度**：这张卡被主动用了几次(`unprompted`)、被提示才用了几次(`prompted`)、脚手架状态。图鉴星级的底数。 |
| `intervention` | `project_id, card_instance_id, type, anchor, criterion, body, level, output_check_verdict` | 一次卡触发的 **AI 介入/追问**（挂在某 `card_instance` 上），含克制阶梯的 `level` 与产出校验结论。 |
| `disposition` | `intervention_id, action, reason` | 学生对那次介入的**处置**（采纳/忽略/…）+ 原因。**「AI 提议、学生怎么回应」的最细粒度选择痕迹。** |

---

## 6. 写作

| 表 | 关键列 | 存的是 |
|---|---|---|
| `snippet` | `project_id, text, section, position` | **按段/按声明的正文片段。** `section` 键：`prop:<step>`（提案各部分）、`claim:<id>`/`subq:<id>`（论文各论点/子问题）。引导式写作把每部分写成一条 section 化 snippet。 |
| `edit_buffer` | `project_id, content, doc_kind` | 当前正在编辑的**整篇缓冲**（`doc_kind` = proposal / essay，0061 多文档）。自动存草稿就写这里。 |
| `draft_snapshot` | `project_id, seq, content, span_index, doc_kind` | 正文的**版本快照序列**（`seq` 递增）——写作演进轨迹。 |
| `writing_finish` | `project_id, doc_kind, finished_at` | 某文档「完成」的时刻（提案完成 / 论文完成）。触发过程评估的里程碑之一。 |

> 铁律：正文活在学生自己那边、AI 绝不代写；这些表存的都是**学生自己写的**文本 + 版本，不是 AI 生成的成品。

---

## 7. 回顾与评估（rubric 那一刀）

| 表 | 关键列 | 存的是 |
|---|---|---|
| `project_reflection` | `project_id, answers, done` | 学生的**回顾问答**（结构化 `answers`）。 |
| `evaluations` | `project_id, scores, narrative, rubric, signals, leaps, model, tier, trigger, trigger_milestone, status, prompt_tokens, completion_tokens, cost_estimate, session_id, thread_id` | **过程评估的正式产物。** `scores` = rubric 评级；`narrative` = 过程叙述；`signals`/`leaps` = 抽取的过程信号与「思维跃迁」；`trigger`/`trigger_milestone` = 由什么里程碑触发；评估走旗舰模型（`tier`）**绝不降级**，用量也记在本行。 |
| `project_mirror_prose` | `project_id, sections, carry_forwards` | 「你的思维印记」镜像叙述（分段 + 可带到下一次的 carry-forwards）。 |
| `project_summary_prose` | `project_id, prose` | 项目层面的总结散文。 |
| `parent_report_prose` | `student_user_id, surface, scope_id, prose` | 面向家长/不同受众的投影叙述。 |
| `class_weekly_prose` | （班级维度） | 班级周报散文（教师端聚合）。 |

> 模式（贯穿评估）：**实时数字现算 + LLM 散文一次成文落库（first-open-wins）**；GET 不花钱、只有 POST 才产生一次 `llm_call`。

---

## 8. 过程信号与账单（贯穿全程）

| 表 | 关键列 | 存的是 |
|---|---|---|
| `event` | `project_id, user_id, surface, type, payload, session_id, thread_id, course_id` | **通用埋点事件流**：在哪个 surface 发生了什么 type 的事（payload jsonb）。重建行为时间线用它。 |
| `activity_log_entry` | `project_id, entry_date, text, source` | 人类可读的**活动日志**（「开题四问 / 添加来源 / 印记体检了整稿 / 完成论文正文」…）。回顾页那条时间线。 |
| `conversation_digest` | `project_id, prose, turns_folded, model, tier` | 长对话**压缩摘要**（被 `chat_message.folded_at` 折叠掉的轮次的浓缩）。 |
| `llm_call` | `project_id, user_id, surface, purpose, provider, model, tier, prompt_tokens, completion_tokens, cost_estimate` | **每一次真实模型调用一行**（陪练 / 锚点 / 课程渲染 / suggest_placement / 评估…）。`purpose` 区分用途，`tier` 区分档位。成本聚合的地基。 |
| `project_ai_use` | `project_id, used_for, not_used_for` | 学生的 **AI 使用声明**（我用 AI 做了什么、没做什么）——学术诚信信号。 |

`llm_usage` 视图三路 UNION（`llm_call` + 冻结的 `message`/`evaluation`）供组织成本汇总。

---

## 9. 按用途反查（评估常问的问题 → 去哪张表）

| 想知道… | 去哪 |
|---|---|
| 学生**在哪里、就什么问了 AI** | `chat_message` where `role='user'`，看 `surface`/`stage`/`quoted_fragment` |
| AI **说了什么、克制到什么程度** | `chat_message` where `role='assistant'`；卡内介入看 `intervention.level` |
| 学生**对来源的取舍/证据判断** | `reference.decision / triage / evidence_* / credibility` |
| 学生**怎么找来源、核查没** | `source_log_entry`（停留、lateral_read、tier）+ `exploration_lead.origin` |
| **探索结构**（问题↔子问题↔文献↔关系） | `exploration_lead` + `question_edge` |
| **工具卡怎么用、跳过没** | `card_instances.status/field_values/event_trace` + `card_competence` |
| AI 提议后学生**怎么选** | `disposition`（卡介入）、`question_edge.status`（关系提议）、reference `decision` |
| **写作演进** | `snippet`（分段）→ `draft_snapshot`（版本）→ `writing_finish`（完成） |
| **正式评估结论** | `evaluations` + `project_mirror_prose` |
| **AI 用量/成本/诚信** | `llm_call` + `project_ai_use` |
| **行为时间线** | `event`（机器）+ `activity_log_entry`（人读） |

---

## 10. 拉一个项目全量过程数据（查询配方）

```sql
-- 1) 定位项目
SELECT id, user_id, title, status, studio_state->>'stage' AS stage, created_at
FROM project WHERE user_id = :uid ORDER BY created_at DESC;

-- 2) 完整 AI 对话（按发生顺序，带房间与阶段）
SELECT m.created_at, m.role, m.surface, m.stage, m.quoted_fragment, m.content
FROM chat_message m
JOIN chat_thread t ON t.id = m.thread_id
WHERE t.seeded_project_id = :pid
ORDER BY m.created_at;

-- 3) 探索结构（问题 + 文献 + 关系）
SELECT id, text, origin, status, parent_lead_id, connected_reference_id FROM exploration_lead WHERE project_id = :pid ORDER BY position;
SELECT from_lead_id, to_lead_id, label, status FROM question_edge WHERE project_id = :pid;

-- 4) 来源与证据判断
SELECT title, decision, triage, credibility, reading_status,
       evidence_nature, evidence_argument, evidence_finding, evidence_placement
FROM reference WHERE project_id = :pid ORDER BY position;

-- 5) 卡使用 + 处置
SELECT ci.card_id, ci.status, ci.field_values, ci.rubric_tags,
       i.type AS intervention, d.action AS disposition
FROM card_instances ci
LEFT JOIN intervention i ON i.card_instance_id = ci.id
LEFT JOIN disposition d ON d.intervention_id = i.id
WHERE ci.project_id = :pid ORDER BY ci.created_at;

-- 6) 写作（分段 → 版本 → 完成）
SELECT section, position, length(text) FROM snippet WHERE project_id = :pid ORDER BY position;
SELECT doc_kind, seq, length(content) FROM draft_snapshot WHERE project_id = :pid ORDER BY doc_kind, seq;
SELECT doc_kind, finished_at FROM writing_finish WHERE project_id = :pid;

-- 7) 评估 + 用量
SELECT scores, trigger_milestone, tier, status FROM evaluations WHERE project_id = :pid;
SELECT purpose, surface, tier, prompt_tokens, completion_tokens, cost_estimate FROM llm_call WHERE project_id = :pid ORDER BY created_at;

-- 8) 行为时间线
SELECT entry_date, source, text FROM activity_log_entry WHERE project_id = :pid ORDER BY entry_date;
SELECT created_at, surface, type FROM event WHERE project_id = :pid ORDER BY created_at;
```

> 边界校验原则：Go 只校验标准信封外层（`status` 枚举、id、`field_values` 是对象、`event_trace` 是数组）；内层深结构真相归 `packages/contracts` 的 Zod 契约。评估读 jsonb 列时以契约为形状真相源。
