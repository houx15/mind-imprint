-- +goose Up
-- 阅读房间的「任务清单 + 段落工具」（2026-08-27 产品裁定）。
--
-- 轻量版的学生不是 IB 学生。给他一篇文章和一个聊天框，他只会做两件事：读一遍
-- 说「读完了」，或者直接问 AI 这篇讲什么。两件都不是阅读。
--
-- 缺的那一级台阶是**段落**：能透镜地读之前，得先能读懂一段——它在说什么、
-- 哪里难、它在整篇里干什么。这条迁移加的就是那一级：
--
--   1. reading_task —— 印记 给这篇文章排出的任务清单（通读 → 精读某一两段 →
--      用一个透镜再看 → 想想信息 → 回答问题）。清单来自一个**写死的 routine
--      库**，模型只负责挑一套并调参（哪几段是重点、用哪个透镜、要几步）。
--      与写作的结构库同一套道理：写死才不会漂进她的内容、才认得出、才便宜。
--   2. reading_block_note —— 段落工具的结果缓存。翻译一段的结果不会变，所以
--      按 (block_id, tool) 存一份重放，第二次点开是瞬时的，也不会重复付钱。
--
-- 只加表、加一列，不动任何既有列，因此对 pro 零影响。

-- routine_key：她这篇用的是哪一套阅读流程（''=还没排过）。既是渲染依据，也是
-- 过程信号——「这篇用的是哪套读法」本身就是 铁律④ 要的证据。
ALTER TABLE reading ADD COLUMN routine_key text NOT NULL DEFAULT '';

CREATE TABLE reading_task (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  atom_id      uuid NOT NULL REFERENCES atom(id) ON DELETE CASCADE,
  position     integer NOT NULL,
  -- kind 决定这一步渲染成什么。加一个 kind 要改代码；加一套 routine 只是加
  -- 数据——这是「schema 驱动是否成立」的同一条验收标准（AGENTS.md 工具卡那条）。
  kind         text NOT NULL CHECK (kind IN ('read','focus_block','lens','reflect','quiz')),
  label        text NOT NULL,
  -- 一句话说这一步要干什么。来自 routine 库，或由模型按这篇文章调过。
  detail       text NOT NULL DEFAULT '',
  -- focus_block 专用：印记 挑出的那一两段。其余 kind 留空。
  block_id     text NOT NULL DEFAULT '',
  -- 'pending' | 'done' | 'skipped'。skipped 是**记录**，不是漏洞：铁律④ 说
  -- 跳过要被记下来，不是被拦住。
  status       text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','done','skipped')),
  completed_at timestamptz
);
CREATE INDEX reading_task_atom_idx ON reading_task (atom_id, position);

CREATE TABLE reading_block_note (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  atom_id    uuid NOT NULL REFERENCES atom(id) ON DELETE CASCADE,
  block_id   text NOT NULL,
  -- 英文段：translate / vocabulary / grammar / craft
  -- 中文段：rhetoric / examples / structure
  tool       text NOT NULL,
  -- 模型写的**讲解**，讲的是这篇文章，不是她的话。与 atom_annotation（她自己
  -- 的批注）分开存，正是为了让报告永远分得清哪句是她想的、哪句是讲给她听的。
  body       text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);
-- 一段一个工具只存一份：重放而不是重算。
CREATE UNIQUE INDEX reading_block_note_key_idx ON reading_block_note (atom_id, block_id, tool);

-- +goose Down
DROP TABLE reading_block_note;
DROP TABLE reading_task;
ALTER TABLE reading DROP COLUMN routine_key;
