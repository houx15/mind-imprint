-- +goose Up
-- S2：session（深挖 / 四种思考模式）、有版本的计划、以及待审阅的结构性变更。
--
-- 命名仍然全部带 pbl 前缀，理由同 0108：pro 占着 project 这个词。

-- ── session ────────────────────────────────────────────────────────────────
--
-- 一次自由深挖和一次 Reframe 是同一种东西的两个 kind，不是两张表。
-- 依据 docs/03-agent-loop-and-thinking-session-protocols.md 十三：每个 kind
-- 只有触发条件、学生动作和写回合同不同，承载它的对话结构完全一样。
--
-- 🚨 depth 是存下来的，不是查出来的。嵌套上限（3 层）于是变成一条 CHECK，
-- 而不是每次插入都要递归走一遍父链。走父链在只有三层时也能跑，但它把一个
-- 常量代价变成了随深度增长的代价，而且没有任何东西拦得住写错的那一次。
CREATE TABLE pbl_session (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  atom_id     uuid NOT NULL REFERENCES atom(id) ON DELETE CASCADE,
  kind        text NOT NULL
              CHECK (kind IN ('free','observation','reframe','brainstorm','plan_check')),
  -- NULL = 挂在主线上。指向另一个 session = 在深挖里再深挖。
  parent_id   uuid REFERENCES pbl_session(id) ON DELETE CASCADE,
  -- 0 是主线上开的第一层。上限 3 层，所以 depth 只能是 0/1/2。
  depth       smallint NOT NULL DEFAULT 0 CHECK (depth >= 0 AND depth <= 2),
  -- 它挂在什么上。free 之外的 kind 也可能由路由器主动提起，此时 anchor_kind
  -- 是 'router'，anchor_ref 记触发它的那条消息。
  anchor_kind text NOT NULL
              CHECK (anchor_kind IN ('approach','hook','step','artifact','free','router')),
  anchor_ref  text NOT NULL DEFAULT '',
  -- 开启它的那句话。钩子问题就是这一句。
  question    text NOT NULL DEFAULT '',
  -- 写回。takeaway 是自由深挖的最小写回；writeback 是有类型 session 按各自
  -- 合同产出的结构化结果（框架、想法卡、Next Bet、计划差异…）。
  -- 🚨 closed_at 非空时，两者至少有一个必须有内容——这条在 Go 里校验，因为
  -- 「够不够」是按 kind 定的，SQL 表达不了。空写回的 session 等于没发生过：
  -- 那正是 doc 03 十四.3 说的「方法论表演」。
  takeaway    text NOT NULL DEFAULT '',
  writeback   jsonb,
  closed_at   timestamptz,
  created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX pbl_session_atom_idx ON pbl_session (atom_id, created_at);
CREATE INDEX pbl_session_parent_idx ON pbl_session (parent_id) WHERE parent_id IS NOT NULL;

-- 对话按 session 分流。NULL = 项目主线，照抄 0102 的 block_id 语义。
ALTER TABLE atom_message ADD COLUMN session_id uuid REFERENCES pbl_session(id) ON DELETE CASCADE;
-- 两个可空的作用域列放在同一张表上，必须把「互斥」说出来：block_id 是写作间
-- 的段落子对话，session_id 是项目的深挖，一条消息不可能同时属于两者。
ALTER TABLE atom_message ADD CONSTRAINT atom_message_one_scope
  CHECK (block_id IS NULL OR session_id IS NULL);
CREATE INDEX atom_message_session_idx ON atom_message (atom_id, session_id, seq);

-- ── 有版本的计划 ───────────────────────────────────────────────────────────
--
-- 版本记的是「理解或决定变了」，不是「某一步打了勾」（doc 02 十.1）。
-- 旧版本、变更依据和她的理由都留着：一个被否掉却不留痕的方向，等于白学一次。
CREATE TABLE pbl_plan_version (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  atom_id    uuid NOT NULL REFERENCES atom(id) ON DELETE CASCADE,
  -- 1, 2, 3… 展示成 v0.1 / v0.2。整数排序，显示层加壳。
  version    integer NOT NULL,
  summary    text NOT NULL DEFAULT '',
  -- 为什么会有这一版。第一版是「她批准了」，之后每一版都连着一条 Plan Check。
  reason     text NOT NULL DEFAULT '',
  -- 谁定的。ai 只能出现在 progress/local 级别的变更上（见 pbl_pending_change）。
  decided_by text NOT NULL DEFAULT 'student' CHECK (decided_by IN ('student','ai')),
  -- 她批准第一版之前，什么都不许跑（spec §12.5）。
  approved_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX pbl_plan_version_idx ON pbl_plan_version (atom_id, version);

CREATE TABLE pbl_plan_step (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  version_id uuid NOT NULL REFERENCES pbl_plan_version(id) ON DELETE CASCADE,
  ordinal    integer NOT NULL,
  title      text NOT NULL,
  blurb      text NOT NULL DEFAULT '',
  goal       text NOT NULL DEFAULT '',
  you_bring  text NOT NULL DEFAULT '',
  i_bring    text NOT NULL DEFAULT '',
  -- 🚨 这一步她判断什么。空的一步是她不该坐在那儿看的一步（spec §12.4）。
  decide     text NOT NULL,
  -- 她那半做完之后交回来什么。没有它，「你来做 X」是派活不是分工。
  -- 列名不叫 then：那是 SQL 关键字，每次都要加引号。
  then_bring text NOT NULL DEFAULT '',
  -- 七种状态（doc 02 十.2）。两个「等待」最值钱：它们让板子说得出为什么
  -- 现在不动——而那恰恰是卡住的学生最讲不清楚的事。
  status     text NOT NULL DEFAULT 'tentative'
             CHECK (status IN ('settled','tentative','awaiting_evidence',
                               'awaiting_decision','done','revised','cancelled')),
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX pbl_plan_step_ordinal_idx ON pbl_plan_step (version_id, ordinal);

-- ── 待审阅的结构性变更 ─────────────────────────────────────────────────────
--
-- 🚨 这张表是整片设计里最要紧的一条不变量：结构性变更**进不了**当前计划。
-- 它只能停在这里，直到一次 Plan Check 记下她的决定。
-- doc 03 十四.4 管这个失败模式叫「隐形重规划」——AI 拿到新信息就直接把目标和
-- 步骤换掉。这张表让它从「不提倡」变成「做不到」。
CREATE TABLE pbl_pending_change (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  atom_id    uuid NOT NULL REFERENCES atom(id) ON DELETE CASCADE,
  -- 差异的形状：新增 / 修改 / 删除 / 暂缓。Plan Check 只展示差异，
  -- 不重新铺一遍整份计划。
  kind       text NOT NULL CHECK (kind IN ('add','modify','remove','defer')),
  diff       jsonb NOT NULL,
  -- 依据。每一条变更都要连到具体的新证据、她的决定或现实约束——
  -- 说不出依据的建议不该出现在她面前。
  evidence   text NOT NULL DEFAULT '',
  -- 她怎么处理的。kept = 保留原计划，那是一个成功的结果，不是没反应。
  resolution text CHECK (resolution IN ('accepted','edited','kept','forked','deferred')),
  reason     text NOT NULL DEFAULT '',
  resolved_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX pbl_pending_change_open_idx ON pbl_pending_change (atom_id) WHERE resolved_at IS NULL;

-- ── 决定 ───────────────────────────────────────────────────────────────────
--
-- 她明确接受、修改、保留或暂缓过的每一件事。gave_up 是「知道自己放弃了什么」，
-- 只有选路那类决定才有；一次 Plan Check 的 kept 没有放弃项。
CREATE TABLE pbl_decision (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  atom_id    uuid NOT NULL REFERENCES atom(id) ON DELETE CASCADE,
  session_id uuid REFERENCES pbl_session(id) ON DELETE SET NULL,
  subject    text NOT NULL,
  choice     text NOT NULL,
  why        text NOT NULL,
  gave_up    text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX pbl_decision_atom_idx ON pbl_decision (atom_id, created_at);

-- +goose Down
DROP TABLE pbl_decision;
DROP TABLE pbl_pending_change;
DROP TABLE pbl_plan_step;
DROP TABLE pbl_plan_version;
DROP INDEX IF EXISTS atom_message_session_idx;
ALTER TABLE atom_message DROP CONSTRAINT atom_message_one_scope;
ALTER TABLE atom_message DROP COLUMN session_id;
DROP TABLE pbl_session;
