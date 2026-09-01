-- +goose Up
-- 方法层：产品负责人 2026-09-01 的七个阶段（docs/2026-09-01-pbl-detail.md）。
--
-- 这七件事以前是 spec §6 的一个占位。现在它们有了形状，所以一次性把地基铺完：
-- 后面每一刀只写 handler 和界面，不再动 schema。这不是图省事——是让七刀真正
-- 互不依赖，可以并行做、单独审、单独回滚。
--
-- 一条贯穿全表的设计：**凡是印记提的，都记 author；凡是她改的，都要理由。**
-- 「AI 提了什么」和「她留下了什么」必须分得开，否则过程树里全是 AI 的话，
-- 评估会把 AI 的思考算成她的。

-- ── ① 工具有了种类，和一条真实的生命线 ──────────────────────────────────
--
-- 两类工具（产品负责人 2026-09-01）：
--   thinking —— 在她和 AI 协作当中支持设计、判断、决策。当场做完。
--   world    —— 支持屏幕之外的真实活动。她会离开，几天后才回来。
--
-- 🚨 这个区分不是叫法，是两处代码要分叉：
--   1. 召完 thinking 工具，印记接着说话；召完 world 工具，这轮对话就该停了——
--      她要出门了，继续追问就是催。
--   2. 挂了三天的 thinking 工具是**放弃了**；挂了三天的 world 工具是**正常的**。
--      不分种类，印记只能二选一地猜错，然后开始催她——铁律②就是这样破的。
ALTER TABLE pbl_tool_instance
  ADD COLUMN kind text NOT NULL DEFAULT 'thinking'
    CHECK (kind IN ('thinking', 'world'));

-- summoned → accepted → done 中间那一段以前是没有的：她答应了、正在做，
-- 这个状态过去无处可存，只能在「刚递出」和「已完成」之间二选一。
ALTER TABLE pbl_tool_instance DROP CONSTRAINT pbl_tool_instance_status_check;
ALTER TABLE pbl_tool_instance
  ADD CONSTRAINT pbl_tool_instance_status_check
    CHECK (status IN ('summoned', 'accepted', 'declined', 'done'));
ALTER TABLE pbl_tool_instance ADD COLUMN accepted_at timestamptz;
ALTER TABLE pbl_tool_instance ADD COLUMN resolved_at timestamptz;
-- 她拒绝时说的那句话。拒绝是记录，不是空白（过程即数据）。
ALTER TABLE pbl_tool_instance ADD COLUMN student_note text NOT NULL DEFAULT '';

-- 两种新的会话：审阅里点开一个词开的那条线，和上线之后每一轮维护。
ALTER TABLE pbl_session DROP CONSTRAINT pbl_session_kind_check;
ALTER TABLE pbl_session
  ADD CONSTRAINT pbl_session_kind_check
    CHECK (kind IN ('free', 'observation', 'reframe', 'brainstorm',
                    'plan_check', 'review', 'keeping'));

-- ── ② 便签板（阶段一）───────────────────────────────────────────────────
--
-- 「observations, quotes, assumptions, and questions，which students can
--   correct, add to, and organize」
--
-- 观察、引语、假设、问题、点子摊在一张板上。头脑风暴用的也是这张板——点子
-- 只是第五种便签，不值得再来一套表。
CREATE TABLE pbl_note (
  id       uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  atom_id  uuid NOT NULL REFERENCES atom(id) ON DELETE CASCADE,
  kind     text NOT NULL
           CHECK (kind IN ('observation', 'quote', 'assumption', 'question', 'idea')),
  body     text NOT NULL,
  -- 🚨 谁写的。印记提的便签她留下了，和她自己写的，是两件不同的事。
  author   text NOT NULL CHECK (author IN ('student', 'yinji')),
  -- 她改过印记的便签——这是"纠正"，是最强的过程信号之一。
  edited   boolean NOT NULL DEFAULT false,
  -- 她自己归的堆。自由字符串：分组怎么叫是她的事，不是我们的枚举。
  cluster  text NOT NULL DEFAULT '',
  -- 板上的位置。这块板归她摆。
  x        real NOT NULL DEFAULT 0,
  y        real NOT NULL DEFAULT 0,
  archived boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX pbl_note_atom_idx ON pbl_note (atom_id, created_at);

-- ── ③ 重构问题（阶段一）─────────────────────────────────────────────────
--
-- 「reframe it into a clear "Who needs what, and why?" statement and then a
--   "How might we?" question」
--
-- 🚨 改写用 supersedes 记，不用 UPDATE 覆盖。产品负责人：「new observations can
-- change the problem, solution, and plan at any time」——问题被重新框定的那一刻
-- 正是这门课的核心事件，覆盖掉就等于把它删了。
CREATE TABLE pbl_reframe (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  atom_id    uuid NOT NULL REFERENCES atom(id) ON DELETE CASCADE,
  -- 谁 / 需要什么 / 为什么
  who        text NOT NULL DEFAULT '',
  needs      text NOT NULL DEFAULT '',
  why        text NOT NULL DEFAULT '',
  -- 我们可以怎样……
  hmw        text NOT NULL DEFAULT '',
  supersedes uuid REFERENCES pbl_reframe(id) ON DELETE SET NULL,
  -- 「every major update reviewed and confirmed by the student」
  confirmed_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX pbl_reframe_atom_idx ON pbl_reframe (atom_id, created_at);

-- ── ④ 审阅（阶段二）─────────────────────────────────────────────────────
--
-- 「highlighted sentences with questions to answer, explanations for each part
--   so that we know what we should care about in each part. Click on any word
--   to have a new chat line with AI」
--
-- 挂在成果上，因为审阅的对象永远是一件交出来的东西。
CREATE TABLE pbl_review_mark (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  artifact_id uuid NOT NULL REFERENCES pbl_artifact(id) ON DELETE CASCADE,
  -- 这一段属于哪个部分，以及在这个部分里该关心什么。
  part        text NOT NULL DEFAULT '',
  part_note   text NOT NULL DEFAULT '',
  -- 被划出来的那句话，和它带的问题。
  quote       text NOT NULL DEFAULT '',
  question    text NOT NULL,
  -- 她的回答；以及她点开这句话时开的那条会话线。
  answer      text NOT NULL DEFAULT '',
  session_id  uuid REFERENCES pbl_session(id) ON DELETE SET NULL,
  ordinal     integer NOT NULL DEFAULT 0,
  created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX pbl_review_mark_artifact_idx ON pbl_review_mark (artifact_id, ordinal);

-- 「a review sugestion, to ask students to answer several dimensions that
--   reviewing this material must pay attention to」——图片和网站没有句子可划，
-- 但每一种材料都有该看的几个维度。
CREATE TABLE pbl_review_dimension (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  artifact_id uuid NOT NULL REFERENCES pbl_artifact(id) ON DELETE CASCADE,
  prompt      text NOT NULL,
  why         text NOT NULL DEFAULT '',
  answer      text NOT NULL DEFAULT '',
  ordinal     integer NOT NULL DEFAULT 0,
  created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX pbl_review_dimension_artifact_idx
  ON pbl_review_dimension (artifact_id, ordinal);

-- ── ⑤ 决策（阶段三）─────────────────────────────────────────────────────
--
-- ⚠️ 产品负责人的这一节是空的（只有标题）。以下是我的设计，等她审。
--
-- 不做加权打分矩阵：那正是被否掉的表单。做的是四句话——
--   摆出选项 → 说清这里到底什么重要 → 每个选项赢在哪、疼在哪 → 选，并说为什么。
-- 最后一格是关键：**什么会让我改主意**。写得出这一句，这个决定才是可复盘的
-- （阶段六要用），也才是一个假设而不是一次表态。
ALTER TABLE pbl_decision ADD COLUMN flip text NOT NULL DEFAULT '';
ALTER TABLE pbl_decision ADD COLUMN settled_at timestamptz;

CREATE TABLE pbl_decision_option (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  decision_id uuid NOT NULL REFERENCES pbl_decision(id) ON DELETE CASCADE,
  label       text NOT NULL,
  wins        text NOT NULL DEFAULT '',
  hurts       text NOT NULL DEFAULT '',
  author      text NOT NULL CHECK (author IN ('student', 'yinji')),
  ordinal     integer NOT NULL DEFAULT 0,
  created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX pbl_decision_option_idx ON pbl_decision_option (decision_id, ordinal);

-- 「这里到底什么重要」——最常被跳过的一步，所以它单独成表，跳过就是空的。
CREATE TABLE pbl_decision_criterion (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  decision_id uuid NOT NULL REFERENCES pbl_decision(id) ON DELETE CASCADE,
  label       text NOT NULL,
  author      text NOT NULL CHECK (author IN ('student', 'yinji')),
  ordinal     integer NOT NULL DEFAULT 0,
  created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX pbl_decision_criterion_idx ON pbl_decision_criterion (decision_id, ordinal);

-- ── ⑥ 结构图（阶段四）───────────────────────────────────────────────────
--
-- 「the most important thing is to let people know that there is a structure,
--   or many people would follow the structure without thinking about it」
--
-- 🚨 和 pbl_session 一样：depth 存下来 + CHECK，不做递归回溯。深度上限让
-- "结构"停在能一眼看完的范围内——这是设计意图，不是技术妥协。
CREATE TABLE pbl_tree_node (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  atom_id    uuid NOT NULL REFERENCES atom(id) ON DELETE CASCADE,
  -- 一个项目可以有好几张结构图（网站一张、文档一张）。
  tree       text NOT NULL DEFAULT 'main',
  parent_id  uuid REFERENCES pbl_tree_node(id) ON DELETE CASCADE,
  depth      smallint NOT NULL DEFAULT 0 CHECK (depth >= 0 AND depth <= 3),
  ordinal    integer NOT NULL DEFAULT 0,
  title      text NOT NULL,
  -- 这一块要放什么。
  body       text NOT NULL DEFAULT '',
  author     text NOT NULL CHECK (author IN ('student', 'yinji')),
  edited     boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX pbl_tree_node_atom_idx ON pbl_tree_node (atom_id, tree, ordinal);

-- 三个问题（产品负责人原文）：盖全了吗 / 顺得下来吗 / 有没有更好的结构。
CREATE TABLE pbl_tree_check (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  atom_id    uuid NOT NULL REFERENCES atom(id) ON DELETE CASCADE,
  tree       text NOT NULL DEFAULT 'main',
  question   text NOT NULL CHECK (question IN ('covers', 'coherent', 'better')),
  answer     text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (atom_id, tree, question)
);

-- ── ⑦ 分工（阶段五）─────────────────────────────────────────────────────
--
-- 「each substep assigned AI, or student, or AI and student, just like those
--   task list, and also the reason. and students can modify and confirm
--   (modify also need to add the reason)」
--
-- 🚨 改一格就要写一句为什么。这一条是整张卡的意义所在：不写理由的分工，
-- 学生只会全部点"同意"，那这张卡就只是在让 AI 领活。
CREATE TABLE pbl_substep (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  step_id        uuid NOT NULL REFERENCES pbl_plan_step(id) ON DELETE CASCADE,
  ordinal        integer NOT NULL DEFAULT 0,
  title          text NOT NULL,
  -- 印记提的分工，和它的理由。
  owner          text NOT NULL CHECK (owner IN ('yinji', 'student', 'both')),
  reason         text NOT NULL DEFAULT '',
  -- 她改成了什么，以及为什么改。owner 为 NULL = 她没动。
  student_owner  text CHECK (student_owner IN ('yinji', 'student', 'both')),
  student_reason text NOT NULL DEFAULT '',
  status         text NOT NULL DEFAULT 'todo'
                 CHECK (status IN ('todo', 'doing', 'done')),
  confirmed_at   timestamptz,
  created_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX pbl_substep_step_idx ON pbl_substep (step_id, ordinal);

-- ── ⑧ 复盘（阶段六）─────────────────────────────────────────────────────
--
-- 产品负责人：「it can be a form」。做成了引导式的一问一答而不是一张字段表——
-- 表单交互 2026-09-01 被否过一次，这里不该再来一遍。每个问题指向项目里**真的
-- 发生过的事**（某个决定、某次改写、某件退回的成果），所以它问的是具体的，
-- 不是空格。
CREATE TABLE pbl_review (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  atom_id    uuid NOT NULL REFERENCES atom(id) ON DELETE CASCADE,
  prompt     text NOT NULL,
  -- 这一问是从哪件真事上长出来的：decision / reframe / artifact / step。
  anchor_kind text NOT NULL DEFAULT 'free',
  anchor_ref  text NOT NULL DEFAULT '',
  answer     text NOT NULL DEFAULT '',
  ordinal    integer NOT NULL DEFAULT 0,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX pbl_review_atom_idx ON pbl_review (atom_id, ordinal);

-- ── ⑨ 维持（阶段七）─────────────────────────────────────────────────────
--
-- 「sometimes students have shipped their website or put their results in some
--   real situations. how to track and use data to iterate? ... when students
--   give some statistics, feedbacks, new thoughts, we can add a new session for
--   this project. (so one project may have several sessions)」
--
-- 一条数据进来，可以就地开一个新会话——这正是"上线之后还在想"的那一步。
CREATE TABLE pbl_keep_entry (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  atom_id    uuid NOT NULL REFERENCES atom(id) ON DELETE CASCADE,
  kind       text NOT NULL CHECK (kind IN ('stat', 'feedback', 'thought')),
  body       text NOT NULL,
  -- 循环里的哪一环：上线 → 看数据 → 读出意思 → 改一件事 → 再上线。
  stage      text NOT NULL DEFAULT 'observe'
             CHECK (stage IN ('ship', 'observe', 'interpret', 'change')),
  session_id uuid REFERENCES pbl_session(id) ON DELETE SET NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX pbl_keep_entry_atom_idx ON pbl_keep_entry (atom_id, created_at);

-- +goose Down
DROP TABLE pbl_keep_entry;
DROP TABLE pbl_review;
DROP TABLE pbl_substep;
DROP TABLE pbl_tree_check;
DROP TABLE pbl_tree_node;
DROP TABLE pbl_decision_criterion;
DROP TABLE pbl_decision_option;
ALTER TABLE pbl_decision DROP COLUMN settled_at;
ALTER TABLE pbl_decision DROP COLUMN flip;
DROP TABLE pbl_review_dimension;
DROP TABLE pbl_review_mark;
DROP TABLE pbl_reframe;
DROP TABLE pbl_note;
ALTER TABLE pbl_session DROP CONSTRAINT pbl_session_kind_check;
ALTER TABLE pbl_session
  ADD CONSTRAINT pbl_session_kind_check
    CHECK (kind IN ('free', 'observation', 'reframe', 'brainstorm', 'plan_check'));
ALTER TABLE pbl_tool_instance DROP COLUMN student_note;
ALTER TABLE pbl_tool_instance DROP COLUMN resolved_at;
ALTER TABLE pbl_tool_instance DROP COLUMN accepted_at;
ALTER TABLE pbl_tool_instance DROP CONSTRAINT pbl_tool_instance_status_check;
ALTER TABLE pbl_tool_instance
  ADD CONSTRAINT pbl_tool_instance_status_check
    CHECK (status IN ('summoned', 'declined', 'done'));
ALTER TABLE pbl_tool_instance DROP COLUMN kind;
