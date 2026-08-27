-- +goose Up
-- 写作的专属四张表。写作与阅读同为 atom 的一种形态（0092）——身份、消息、
-- 工具卡、批注、报告全部复用底座，一字不改；这里只加写作自己独有的四件事：
-- 篇章元信息（含四阶段进度）、提纲、片段、成稿。

CREATE TABLE writing (
  atom_id      uuid PRIMARY KEY REFERENCES atom(id) ON DELETE CASCADE,
  title        text NOT NULL DEFAULT '',
  lang         text NOT NULL CHECK (lang IN ('zh','en')),
  -- 五个值，不是四个：'finished' 是终态，与 status 的 'finished' 并存——
  -- status 说这篇结束了没有，stage 说她走到四步里的哪一步。学生可以跳过阶段
  -- 直接完成（铁律④：跳过被记录，不被强制补齐），所以两者可能永远不同步。
  stage        text NOT NULL DEFAULT 'ideate'
               CHECK (stage IN ('ideate','outline','snippets','draft','finished')),
  target_words integer,                    -- 构思阶段敲定的篇幅，NULL 表示还没定
  status       text NOT NULL DEFAULT 'active' CHECK (status IN ('active','finished')),
  updated_at   timestamptz NOT NULL DEFAULT now(),
  finished_at  timestamptz
);

CREATE TABLE writing_outline (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  atom_id    uuid NOT NULL REFERENCES atom(id) ON DELETE CASCADE,
  text       text NOT NULL,
  depth      integer NOT NULL DEFAULT 0,
  position   integer NOT NULL
);
CREATE INDEX writing_outline_atom_idx ON writing_outline (atom_id, position);

CREATE TABLE writing_snippet (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  atom_id     uuid NOT NULL REFERENCES atom(id) ON DELETE CASCADE,
  outline_id  uuid REFERENCES writing_outline(id) ON DELETE SET NULL,
  position    integer NOT NULL,
  text        text NOT NULL DEFAULT '',
  updated_at  timestamptz NOT NULL DEFAULT now()
);
-- UNIQUE, not just an index: 段落是「一段一段来」，学生反复回来改同一段——
-- UpsertWritingSnippet 需要一个自然键去认出「这是同一段的新版本」而不是又插
-- 一行。position 在一次写作里天然是那个键（它就是这段在提纲里的第几位）。
CREATE UNIQUE INDEX writing_snippet_atom_position_idx ON writing_snippet (atom_id, position);

-- 成稿：由片段拼成，可再编辑。只有学生的字。
CREATE TABLE writing_draft (
  atom_id    uuid PRIMARY KEY REFERENCES atom(id) ON DELETE CASCADE,
  body       text NOT NULL DEFAULT '',
  updated_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE writing_draft;
DROP TABLE writing_snippet;
DROP TABLE writing_outline;
DROP TABLE writing;
