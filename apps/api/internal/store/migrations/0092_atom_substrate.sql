-- +goose Up
-- 轻量版的原子底座。阅读 / 写作（以及之后的 AI 聊天、AI 项目）四者不同的是各自
-- 的专属字段，相同的是四件事：一条消息流、一批工具卡、一层批注、一份小报告。
-- 所以：薄薄一层 atom 身份 + 共享机制表，专属字段各自成表。加一种新形态 =
-- 加一张专属表 + 复用底座。
--
-- 这些表与 pro 的 project 及其子表完全无关，互不引用。

CREATE TABLE atom (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  kind       text NOT NULL CHECK (kind IN ('reading','writing')),
  user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX atom_user_kind_idx ON atom (user_id, kind, created_at DESC);

CREATE TABLE reading (
  atom_id     uuid PRIMARY KEY REFERENCES atom(id) ON DELETE CASCADE,
  title       text NOT NULL DEFAULT '',
  lang        text NOT NULL CHECK (lang IN ('zh','en')),
  status      text NOT NULL DEFAULT 'active' CHECK (status IN ('active','finished')),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  finished_at timestamptz
);

-- 共享机制 ---------------------------------------------------------------

-- seq 由服务端分配；(atom_id, seq) 唯一，保证并发下不会静默乱序。
CREATE TABLE atom_message (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  atom_id    uuid NOT NULL REFERENCES atom(id) ON DELETE CASCADE,
  seq        integer NOT NULL,
  role       text NOT NULL CHECK (role IN ('student','ai','system')),
  content    text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX atom_message_seq_idx ON atom_message (atom_id, seq);

-- 标准信封。结构与 pro 的 card_instances 保持一致——它是过程数据与评估的
-- 共同地基。Go 只做边界校验，内层深结构真相归 packages/contracts 的 Zod 契约。
CREATE TABLE atom_card (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  atom_id      uuid NOT NULL REFERENCES atom(id) ON DELETE CASCADE,
  card_id      text NOT NULL,
  block_id     text,
  status       text NOT NULL CHECK (status IN ('proposed','active','submitted','skipped')),
  field_values jsonb NOT NULL DEFAULT '{}'::jsonb,
  event_trace  jsonb NOT NULL DEFAULT '[]'::jsonb,
  created_at   timestamptz NOT NULL DEFAULT now(),
  submitted_at timestamptz
);
CREATE INDEX atom_card_atom_idx ON atom_card (atom_id, created_at);

CREATE TABLE atom_annotation (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  atom_id    uuid NOT NULL REFERENCES atom(id) ON DELETE CASCADE,
  block_id   text NOT NULL,
  span       jsonb NOT NULL,
  quote      text NOT NULL DEFAULT '',
  note       text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX atom_annotation_atom_idx ON atom_annotation (atom_id, created_at);

-- 阅读专属 ---------------------------------------------------------------

CREATE TABLE reading_source (
  atom_id     uuid PRIMARY KEY REFERENCES atom(id) ON DELETE CASCADE,
  title       text NOT NULL DEFAULT '',
  body        text NOT NULL,
  source_url  text,
  bib         jsonb,
  ingested_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE reading_brief (
  atom_id        uuid PRIMARY KEY REFERENCES atom(id) ON DELETE CASCADE,
  phase_tag      text,
  reading_reason text NOT NULL DEFAULT '',
  reading_focus  text NOT NULL DEFAULT '',
  updated_at     timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE reading_takeaway (
  atom_id    uuid PRIMARY KEY REFERENCES atom(id) ON DELETE CASCADE,
  text       text NOT NULL DEFAULT '',
  updated_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE reading_takeaway;
DROP TABLE reading_brief;
DROP TABLE reading_source;
DROP TABLE atom_annotation;
DROP TABLE atom_card;
DROP TABLE atom_message;
DROP TABLE reading;
DROP TABLE atom;
