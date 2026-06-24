# 参考 · LLM 对话历史的存储结构（Postgres 后端）

> 参考文档，为指导「思维印记」后端的 `messages` 表设计而生成 ｜ 2026-06-24
>
> 范围：Go 后端（`pgx/v5` + `sqlc` + Postgres），学生与 AI 陪练的多轮对话。每个 `task`（一份学生工作）拥有一段会话，每轮把历史回灌给模型。配套 [database-schema.md](./database-schema.md) 与 [api-design.md](./api-design.md)。
>
> 写作约定：中文叙述 + 英文技术名词。每个主题给「理由 + 选型 + 主要替代」。结论用 2025/2026 现状（Postgres 14+ TOAST/lz4、声明式分区、pg_partman、LiteLLM/Langfuse/Helicone 计量实践、LangGraph/Anthropic 上下文管理）核对，不只凭记忆。
>
> 一句话先行：**「messages 表膨胀太快」对这个项目的体量来说基本是杞人忧天。** 真正要做对的是「append-only 行级存储 + keyset 分页 + 把回灌给模型的上下文与持久化的全量记录解耦」。分区、冷归档都属于「以后再说、且迁移很便宜」。

---

## 1. Row-per-message vs JSONB-blob vs hybrid

**三种形态：**

- **Row-per-message（每条消息一行）** — 现有 schema 的做法。append-only，每轮只 INSERT 一行。
- **JSONB-blob-per-conversation（整段会话塞进一个 JSONB 列）** — `tasks.transcript jsonb`，每轮 UPDATE 这个数组。
- **Hybrid（混合）** — 行级存全量真相，另存一份派生的「窗口/摘要」给模型用（见 §7）。

**为什么 JSONB-blob 是陷阱（这是本节最重要的结论）：** Postgres 的 MVCC + TOAST 在「不断往一个大 JSONB 上追加」时会触发严重的 **WAL write-amplification**。一旦 JSONB 超过 ~2KB 被 TOAST 出表，每次 append 都要重写整个 out-of-line 值：往一个 1MB 的会话里追加 100 字节，会产生 **1MB+ 的 WAL 流量**。Postgres 官方文档也明说：任何 UPDATE 都会对整行加 row-level lock，「请把 JSON 文档控制在可管理的大小以降低锁竞争」。对一个「每轮都要写、会话会长到几十轮」的多轮陪练，blob 形态是确定的反模式。

**为什么 row-per-message 对本项目是对的：**
- **append（追加）** — 每轮纯 INSERT，无 write-amplification，无整行锁竞争。
- **query（查询）** — 「按 `created_at` 取一个 task 的全量历史」是 `(task_id, created_at)` 上的一次 range scan，天然有序。
- **partial update（局部更新）** — 改某条消息（极少发生）只动一行，不碰其余历史。
- **analytics（分析）** — 过程事件、计量、评估都按行聚合；blob 形态做这些要先把 JSON 拆开，得不偿失。

**Hybrid 的角色：** 不是替代行级存储，而是叠在它上面的优化——全量真相永远是 `messages` 行（append-only、不可变），「回灌给模型的那一份」是请求时重建的窗口/摘要（§7）。本项目应采纳这层「读时投影」的 hybrid，但**不要**把会话主体塞进 blob。

> **推荐：row-per-message，保持现有 `messages` 表的行级形态；在其上叠一层「读时重建的上下文窗口」（§7）。绝不用 JSONB-blob-per-conversation 存会话主体。**

---

## 2. 现实增长数学（给「膨胀太快」校准）

把恐惧换成数字。先估**单行字节**，再乘规模。

**单行字节（main heap，body 不算 TOAST 时）：**

| 部分 | 字节 |
|---|---|
| tuple header（含对齐） | 24 |
| line pointer（页内指针） | 4 |
| 固定列：`id` uuid(16) + `task_id` uuid(16) + `role`(~1) + `created_at` timestamptz(8) + 杂项对齐 | ~48 |
| `content` 文本（中英混排陪练消息，**一次只问一个**，短）≈ 150–400 B inline | ~300 |
| `tool_call` jsonb（多数为 NULL） | ~0–60 |
| **合计（典型行）** | **≈ 380 B** |

关键 TOAST 行为：body 超过 ~2KB 才被压缩/移出表，届时 **main-heap 行反而缩回 ~70–80B + 18B TOAST 指针**——所以长消息不会拖慢「列表/元数据扫描」（§5）。算总量按 ~400 B/行 估偏保守即可。

**规模假设：** 每个 task ~20–40 轮 → 取 **60 行/task**（user+assistant+少量 system，外加 SSE/卡片相关行的冗余）。每行 ~400 B（含索引开销，整体按 ~600 B/行 放大估）。

| 学生数 | tasks/人 | 行数 | 存储（含索引，~600B/行） |
|---|---|---|---|
| 1k | 10 | 60 万 | ~0.35 GB |
| 10k | 10 | 600 万 | ~3.5 GB |
| 100k | 10 | 6000 万 | ~35 GB |
| 100k | 30 | 1.8 亿 | ~110 GB |

**Postgres 在这些量级完全没问题。** 单表到 **几千万到一亿行**、几十 GB，配上 `(task_id, created_at)` 索引、keyset 分页，仍是平凡负载——而且本项目的核心查询永远带 `task_id` 等值条件（取某 task 的历史），命中索引后扫的是几十行，跟全表多大无关。

**什么时候才真正成为问题？** 经验阈值大致在**单表 1 亿行以上、或表+索引体量到几百 GB**，开始出现：autovacuum/`VACUUM` 时间拉长、索引膨胀、单分区备份/`pg_dump` 变慢、想批量删旧数据时一条 `DELETE` 锁太久。注意：这些痛点是**运维性**的（vacuum、归档、删除），**不是查询延迟**——查询有 `task_id` 索引一直很快。这正是分区（§3）解决的问题域，而不是「查询慢了才分区」。

> **推荐：恐惧不成立。** 按 10 万学生 × 30 tasks 仍只是亿级行 / 百 GB 级，Postgres 单表稳吃。把精力放在索引与分页（§4）上，分区留到运维信号出现再做（§3）。

---

## 3. 分区（Partitioning）

**两种分区键：**
- **RANGE on `created_at`（按时间范围）** — 月/周一个分区。优点：旧数据成块，归档/删除变成 `DETACH`/`DROP PARTITION`（瞬时、不产生 dead tuples、不锁活表）；新写入集中在最新分区，索引更热更小。**这是聊天/日志类时序数据的标准选择。**
- **HASH on `task_id`（按 task 哈希）** — 把负载摊到 N 个分区。优点：单 task 历史落在同一分区，且写入均匀。缺点：**对归档/保留毫无帮助**（旧数据散落各分区，删不动），而本项目的运维痛点恰恰是「删旧数据/冷归档」。

对本项目，痛点是保留与归档，所以**若分区则按 `created_at` RANGE**，不用 hash。

**现在做还是以后做？** **以后做。** 理由：
1. §2 表明在可预见体量内单表不分区毫无压力；现在分区是 premature optimization，还会给 `summon_card` 的过程树查询、`sqlc` 查询和 PK/唯一约束（分区表的 unique 必须含分区键）平添复杂度。
2. **从普通表迁到分区表很便宜，且可在线做**，迁移路径成熟：
   - 新建 `messages_partitioned`（`PARTITION BY RANGE (created_at)`，PK 改为 `(id, created_at)` 以含分区键）；
   - 用 pg_partman 自动建分区 + `run_maintenance_proc()` 滚动维护、按 `retention` 过期；
   - 分批把老数据 `INSERT ... SELECT` 迁入，新写双写或切到新表，校验后 `RENAME`；
   - 或更省事：当老数据可整段冷藏时，直接在新表上**只对新数据分区**，旧表保留为「历史只读分区」`ATTACH` 进来。

**何时扣扳机：** 出现以下任一信号——单表逼近 ~5000 万–1 亿行、autovacuum 在 `messages` 上明显吃力、想做时间保留策略却被「`DELETE` 太慢/锁太久」卡住。在此之前，分区的收益是负的。

> **推荐：现在不分区。** 预留迁移路径（PK 设计上留意将来要含 `created_at`），到运维信号出现时用 **pg_partman + RANGE(created_at) + pg_cron** 一次性迁移。

---

## 4. 索引与分页（Indexing & Pagination）

**两个核心访问模式：**

1. **载入某 task 的全量历史（按时间正序）** — `GET /tasks/:id` 的 `messages`。
   ```sql
   SELECT ... FROM messages WHERE task_id = $1 ORDER BY created_at, id;
   ```
   命中现有复合索引 **`(task_id, created_at)`**——这是最重要的索引，已在 schema 里，保持。建议把 `id` 作为 ORDER BY 的 tie-breaker（同毫秒多行时稳定排序），并考虑索引升级为 `(task_id, created_at, id)`。

2. **列出最近的 tasks** — `GET /tasks`。这查的是 `tasks` 表，不是 `messages`：
   ```sql
   SELECT ... FROM tasks WHERE user_id = $1 ORDER BY last_active_at DESC;
   ```
   命中 `tasks(user_id, last_active_at desc)`——已在 schema 里。**不要**为「最近 tasks」去扫 `messages`；`last_active_at` 冗余在 `tasks` 上就是为此。

**keyset/cursor 分页 vs OFFSET：** 对会话历史，多数情况一次性取全量（一段陪练几十轮，~400B/行，几十 KB，直接全取最简单，无需分页）。**若**某些超长 task 需要分页，**用 keyset（cursor）而非 OFFSET**：
```sql
-- 向下翻页：游标是上一页最后一行的 (created_at, id)
SELECT ... FROM messages
WHERE task_id = $1 AND (created_at, id) > ($2, $3)
ORDER BY created_at, id
LIMIT 50;
```
OFFSET 在深翻页时要扫并丢弃前面所有行，O(offset)，越往后越慢且在并发插入下会跳行/重复；keyset 用复合索引直接定位，O(log n) 且稳定。游标编码 `(created_at, id)` 即可。

> **推荐：** 保留 `messages(task_id, created_at)`（可加 `, id` 收尾做稳定排序），`tasks(user_id, last_active_at desc)` 服务「最近 tasks」。历史默认一次全取；需要分页时用 keyset，绝不用 OFFSET。

---

## 5. 长文本处理（Large Text / TOAST）

**TOAST 基础：** 页 8KB，单 tuple 不能跨页。行超过 `TOAST_TUPLE_THRESHOLD`（~2000 B）时，Postgres 把最宽的 varlena 列**压缩并移出表**，直到行缩回 target 以下。`text`/`jsonb` 默认 **EXTENDED**（压缩 + 必要时出表）。out-of-line 值是分块存的，**只在真正读那一列时才取回**——这就是「不 `SELECT` body 的扫描天然快」的原因。

**压缩：** Postgres 14+ 的 `default_toast_compression` 可选 `pglz`（老）或 **`lz4`**。lz4 压缩快约 5×、解压快约 20%，压缩比仅差约 7%。对可压缩的中英文陪练文本，**lz4 是更优默认**（只影响新写入）。

**要不要把 body 拆到单独的表？** 一般**不需要**——TOAST 已经在透明地做「垂直分区」：只读元数据列的扫描根本不会碰到被 TOAST 的 body。真正的坑是 **「medium text」问题**：长度卡在 ~0.5–2KB、刚好不触发出表的值会**留在 heap 里撑大主表**，让元数据扫描变慢。两条对策：
1. **永远显式列名 `SELECT`，绝不 `SELECT *`**——避免无谓地把 body de-chunk 取回。这是性价比最高的一招。
2. 仅当 body 大量聚集在那个 ~0.5–2KB「中等」区间、或 body 与元数据的更新热路径不同时，才考虑把 body 拆成 1:1 子表。本项目陪练消息短（一次只问一个），不在此列。

**何时外置到对象存储？** 仅限**大二进制附件**（图片/音频）——blob 进 S3/R2，Postgres 只存元数据 + 对象 key。纯文本消息留在 `text` 列即可（可查、可 join、事务一致，1GB 上限远够）。

> **推荐：** body 留 `content text`，开 `default_toast_compression = lz4`（PG14+），列表/元数据查询一律显式选列、排除 `content`。不拆 body 子表（本项目消息短）。

---

## 6. 保留 / 归档 / 冷存储 / 删除（Retention & Privacy）

**时间归档：** 由 §3 的 RANGE 分区承载。用 **pg_partman** 配 `retention`（如 `'365 days'`）、`retention_keep_table`，`run_maintenance_proc()` 滚动建分区 + 过期旧分区。冷数据 → 对象存储：`DETACH` 旧分区后导出，或写 **S3 上的 Parquet**、需要时经 `parquet_fdw` 再挂回查询。

**与「回灌给模型」的关系：** 冷/归档消息**不应原样回灌**。热数据保留最近若干轮 verbatim；更早的内容靠一份滚动**摘要**（§7）代表；冷的全量历史只为审计 / 过程树留存。这正好对上 hot/cold 拆分。

**隐私删除（学生或学校删数据）：**
- **soft delete ≠ 真删除（erasure）。** 仅打标记的行仍可被查询/导出，合规上视为「未删除」。GDPR 式删除要落到**真正抹除**。
- 务实做法是**两段式 soft-TTL → hard-TTL**：soft-delete 立刻在 UI 隐藏（保住即时体验），稍后一道 hard 任务真正 purge/匿名化。
- **crypto-shredding（密钥粉碎）** 是 2025-2026 处理「数据散落在主库/备份/导出」的主流：敏感字段用**每学生/每 org 一把密钥**加密，删除时只销毁那把密钥，所有密文即不可恢复，审计链仍在。
- **关键陷阱（与本项目强相关）：** crypto-shredding 只抹除「用被销毁密钥加密的数据」。**摘要消息、评估 rubric/叙述、过程树节点、向量 embedding 这些含个人信息的派生物，如果不在同一把密钥下，就不会被粉碎**——必须在删除流程里显式删除/匿名化。设计密钥边界时要把这些派生物圈进去。

> **推荐：** 保留用 §3 的 pg_partman 时间分区（`DETACH`/`DROP` 旧分区，冷数据进 S3-Parquet），喂模型的是滚动摘要而非冷的原始轮次。删除走 soft-TTL（即时隐藏）→ hard-TTL（真删/匿名），并确保删除流程覆盖 `evaluations`、摘要、过程树等派生个人数据。

---

## 7. 上下文窗口管理（Context-Window Management）

每轮回灌全量历史是 O(n) 成本/延迟，迟早溢出。三种标准模式，关键在「**什么必须持久化 vs 什么请求时重建**」：

| 模式 | 持久化（DB 里） | 请求时重建 |
|---|---|---|
| **Sliding window（滑动窗口）** | 全量 raw transcript（真相源） | 发送前裁出最近 N 轮 / token 预算；**绝不改动已存历史** |
| **Running summary / 压缩** | raw transcript；可缓存最新摘要 | 每次（或越阈值时）重生成摘要 + 拼最近 verbatim 轮 |
| **周期性摘要消息** | raw transcript + 摘要文本 + **watermark（最后被摘要的 message id）** | prompt = 存好的摘要 + watermark 之后的 raw 消息 |

**总原则：持久化不可变的 raw transcript 作真相源，发送时重建窗口/摘要视图。** 本项目 `messages` 表就是真相源；回灌给模型的是请求时构造的窗口。

**2025-2026 生态现状：**
- **LangChain/LangGraph：** 旧的 `ConversationSummaryMemory`/`ConversationBufferMemory` 已废弃（1.0 移除），改用 checkpointer + `trim_messages`（硬裁）/`SummarizationMiddleware`（按 token 阈值触发压缩、保留最近 N 轮）。
- **Anthropic：** context editing（清理陈旧 tool result）+ compaction（把历史压成 `<summary>` 块）+ memory tool（把事实落到文件）。

**与 prompt caching 的关键互动：** 缓存前缀顺序是 `tools → system → messages`，**改动靠前的消息会让从那里起的整个前缀缓存失效**。所以：历史保持 **append-only**；压缩时把摘要作为**新的 cached system block** 追加，而**不要回头改写消息历史**。这与本项目「`messages` append-only、`summon_card` 工具结果靠下一轮回灌」的 SSE 回合循环天然契合。

**对持久化的影响（落到本项目）：** `messages` 全量 raw 必须持久化（过程树、评估、审计都依赖它）；窗口是请求时算的、不落库。**唯一需要新增持久化的是「周期性摘要」**——若采纳压缩，加一种 `role='summary'` 的消息行（或 `tasks.summary` + `summary_watermark_message_id`），其余靠重建。建议起步阶段先做**纯滑动窗口 + token 预算**（零额外 schema），会话真的长到溢出再上摘要压缩。

> **推荐：** 持久化全量 raw transcript 作真相源；每轮发送 token 预算内的滑动窗口；越阈值时做一次压缩，把摘要作为 cached system block 追加、并记一个 watermark。保持 append-only 以维持 prompt caching。起步先只做滑动窗口。

---

## 8. 把 token 计量并进 turn 行（vs 独立 ledger）

**问题：** 把 `model` / `tier` / `prompt_tokens` / `completion_tokens` / `cost` 直接放到 assistant message 行（一次模型调用 == 一条 assistant 消息），而不是单独的 `llm_calls` 账本表，合理吗？

**结论：对「聊天这条线」合理且是主流。** 生产系统普遍是「**每次 LLM 调用一行、计量就挂在那行上**」，而非规范化的 join 表：
- **LiteLLM** `LiteLLM_SpendLogs`：`model`、`prompt_tokens`、`completion_tokens`、`spend`、`team_id`、`organization_id` 全在每请求行上；rollup 走派生的 `DailyView`。
- **Langfuse** `usage_details`/`cost_details` 是 generation/observation 行上的字段；聚合按需算。
- **Helicone** 每请求一行带计量；后来才把数据另写 ClickHouse 仅为加速聚合，per-request 粒度不变。
- **OpenAI** 把 usage 与「对账过的 Costs」分成两个端点，并提示二者「未必完全对得上」——启示：**计量挂在行上是真相源，billing 是另一层对账**。

**Pros：** 写入原子（消息 + 成本一起落库）；1:1 粒度下最简；`SUM(...) GROUP BY org, day, model` 配 `(org_id, created_at, model)` 索引就能出 rollup。

**Cons / 必须处理的点（与本项目强相关）：**
1. **不是每次模型调用都是 assistant 消息。** 本项目的**旗舰评估**就是一次有成本、但没有 chat 行的 LLM 调用。→ **把同一组计量列也放到 `evaluations` 行上。** 这是把计量并进消息行的前提。
2. **消息可能被编辑/重生成/软删**，billing 不喜欢可变行——但本项目消息 append-only，问题很小。
3. **若某条 assistant 消息将来跨多次模型调用**（重试 / tool 循环），1:1 破裂，行上的计量会少算。本项目「一卡一轮、turn 结束」的设计目前是 1:1，但要记住这个边界。

**若将来需要独立 ledger，迁移路径（全程 additive、零 backfill）：**
1. 先建一个 SQL view，把消息计量 + 评估计量 **UNION ALL** 成逻辑账本 `llm_usage`（即 LiteLLM DailyView 的思路）：
   ```sql
   CREATE VIEW llm_usage AS
   SELECT id AS source_id, 'chat'  AS kind, task_id, model, tier,
          prompt_tokens, completion_tokens, cost, created_at
   FROM messages WHERE role='assistant' AND prompt_tokens IS NOT NULL
   UNION ALL
   SELECT id, 'eval', task_id, model, tier,
          prompt_tokens, completion_tokens, cost, created_at
   FROM evaluations WHERE prompt_tokens IS NOT NULL;
   ```
   org 维度通过 `task → user → membership` join 出来（或在写入时把 `org_id` 冗余到行上以省 join）。
2. rollup 慢了再加 materialized view / `daily_org_spend` 表。
3. 只有当需要**不可变计费账本**时，才落地 append-only `llm_calls` 表（即 OpenAI 的 Usage-vs-Costs 拆分）。每步都加表/加视图，不动既有数据。

**对现有 schema 的明确改动：** 现 `database-schema.md` 设了独立 `llm_calls` 表。本文建议**改为：把计量列并进 `messages`（assistant 行）与 `evaluations` 行，删去 `llm_calls` 表，需要账本视图时用上面的 `llm_usage` UNION view。** 这更贴主流、写入更原子、且 demo→小型生产阶段少维护一张表；待真要按 org 计费时再走上面的渐进路径。

> **推荐：YES，并进去。** 计量列上 `messages`（assistant）与 `evaluations` 两处，删 `llm_calls` 表，rollup 用 `llm_usage` UNION view，独立账本表延后到真有计费需求时再加（迁移是 additive 的）。

---

## 9. 本项目推荐的 messages schema（含与现 schema 的差异）

下面是综合 §1–§8 后，针对本项目的具体建议。**与 `database-schema.md` 一致为主，差异显式标注。**

```sql
CREATE TABLE messages (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id     uuid NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    role        text NOT NULL CHECK (role IN ('user','assistant','system','summary')),
    content     text NOT NULL DEFAULT '',         -- TOAST 透明处理长文；lz4 压缩
    tool_call   jsonb,                             -- summon_card 调用参数 + tool_call id

    -- ↓↓↓ 新增：把 LLM 计量并进 assistant 行（删除独立 llm_calls 表，见 §8）
    model              text,                       -- 仅 assistant 行非空
    tier               text,                       -- 'chaperone' / 'flagship'
    prompt_tokens      integer,
    completion_tokens  integer,
    cost               numeric(12,6),

    -- ↓↓↓ 可选（§7，做摘要压缩时才需要）
    summary_watermark_message_id uuid,             -- 仅 role='summary' 行用，指向最后被摘要的消息

    created_at  timestamptz NOT NULL DEFAULT now()
);

-- 核心索引：取某 task 的有序历史（id 收尾做稳定排序 + keyset 游标）
CREATE INDEX idx_messages_task_created ON messages (task_id, created_at, id);

-- 计量 rollup 用（§8）。org_id 不冗余在行上，则靠 task→membership join，
-- 此索引退化为按时间/模型；若将来冗余 org_id，再建 (org_id, created_at, model)。
CREATE INDEX idx_messages_usage ON messages (created_at, model)
    WHERE role = 'assistant' AND prompt_tokens IS NOT NULL;
```

并在 `evaluations` 表补同一组计量列（§8）：
```sql
ALTER TABLE evaluations
  ADD COLUMN tier text,
  ADD COLUMN prompt_tokens integer,
  ADD COLUMN completion_tokens integer,
  ADD COLUMN cost numeric(12,6);
-- evaluations 已有 model 列，复用。
```

数据库层一次性设置（§5）：
```sql
-- 新库初始化即可；只影响后续写入
SET default_toast_compression = lz4;   -- 或在 postgresql.conf 设
```

**与 `database-schema.md` 的差异清单（仅这几处）：**

| 项 | 现 schema | 本文建议 | 理由 |
|---|---|---|---|
| `llm_calls` 表 | 独立一张表，每次调用一行 | **删除**；计量并进 `messages`(assistant) + `evaluations` | §8：贴主流、写入原子、少一张表；需要账本时用 `llm_usage` UNION view |
| `messages` 计量列 | 无 | 加 `model/tier/prompt_tokens/completion_tokens/cost` | 同上 |
| `evaluations` 计量列 | 仅 `model` | 加 `tier/prompt_tokens/completion_tokens/cost` | 评估也是带成本的 LLM 调用，不能漏出 rollup |
| `messages.role` | `('user','assistant','system')` | 加 `'summary'` | §7：周期性摘要落为一类消息行（做压缩时才用） |
| `messages` 索引 | `(task_id, created_at)` | `(task_id, created_at, id)` | §4：稳定排序 + keyset 游标 tie-breaker |
| 计量视图 | 无 | 新增 `llm_usage` UNION view（按需） | §8：跨 chat+eval 的逻辑账本 |
| 分区 | 无 | **保持无分区**（留迁移路径） | §3：当前体量不需要；以后用 pg_partman 迁 |
| TOAST 压缩 | 未指定 | `default_toast_compression = lz4` | §5 |

**不变的部分（刻意对齐）：** `card_instances` 的标准信封（`field_values`/`event_trace` jsonb）原样保留——过程树/评估的地基不动；`tasks(user_id, last_active_at desc)` 服务「最近 tasks」原样保留；过程树仍是 `card_instances.parent_node_id` + `messages` 时间线的读时投影，无 `process_node` 表。

---

## 结论与推荐（Verdict）

**(a) 「messages 表膨胀太快」的担忧成立吗？——不成立。** 按 10 万学生 × 30 tasks × ~60 行/task 也只是**亿级行 / 百 GB 级**，且核心查询永远带 `task_id` 等值条件命中索引，与表多大无关。真正的痛点是「删旧数据/冷归档/vacuum」这类**运维性**问题，出现在单表逼近 ~5000 万–1 亿行之后——那时再分区，不是现在。

**(b) 推荐的 messages schema：** 维持 **row-per-message、append-only** 的行级形态（绝不用 JSONB-blob 存会话主体）；索引 `(task_id, created_at, id)`；`content text` 配 lz4 TOAST；历史默认全取、需要时用 keyset 分页；`role` 增加 `'summary'` 以备压缩。全量 raw 是真相源，回灌给模型的窗口请求时重建。

**(c) 把计量并进 messages？——YES。** 计量列上 `messages`(assistant) 与 `evaluations` 两处，**删掉独立 `llm_calls` 表**；按 org 出账时用 `llm_usage` UNION view，独立账本表延后到真有计费需求时再加（迁移全程 additive、零 backfill）。这是 LiteLLM/Langfuse/Helicone 的主流做法。唯一硬要求：评估行也要带同一组计量列，否则旗舰评估的成本会从 rollup 里漏掉。

**(d) 现在分区还是以后？——以后。** 当前分区是 premature optimization，还会拖累过程树查询与约束设计。到运维信号出现（~5000 万–1 亿行 / vacuum 吃力 / 删旧数据被锁卡住）时，用 **pg_partman + RANGE(created_at) + pg_cron** 在线迁移即可——成本低且路径成熟。现在只需在 PK/索引设计上留意将来要含 `created_at`。
