-- +goose Up
-- 成果，以及工具的端点。
--
-- 这一刀只建地基：成果怎么渲染、工具长什么样，都还等产品负责人的交互设计。
-- 但"没有理由就不落地"这条规则本身是定了的，所以它现在就进库、进 handler。

-- ── 成果 ───────────────────────────────────────────────────────────────────
--
-- 印记做出来交给她判断的东西：几个方案、一份草稿、一份规格、一张图、一个页面。
--
-- guessed / admits 是**必填**的设计，不是可选的礼貌：一份把自己的假设藏起来的
-- 交付物，只能被接受，不能被评审——而评审正是这件事的意义。
CREATE TABLE pbl_artifact (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  atom_id    uuid NOT NULL REFERENCES atom(id) ON DELETE CASCADE,
  session_id uuid REFERENCES pbl_session(id) ON DELETE SET NULL,
  kind       text NOT NULL
             CHECK (kind IN ('options','draft','spec','image','site','html')),
  title      text NOT NULL DEFAULT '',
  -- 🚨 只放文本 / JSON。二进制一律进 OSS，这里存 key（spec §11）。
  -- 图片字节进 Postgres 会把每一次读成果的查询都撑大，撑坏已经假设行很小的
  -- 报告和导出路径，还会把二进制塞进备份流。
  payload    jsonb NOT NULL DEFAULT '{}'::jsonb,
  -- 印记自己说的：我猜了什么，我这版还有什么不对。
  guessed    jsonb NOT NULL DEFAULT '[]'::jsonb,
  admits     jsonb NOT NULL DEFAULT '[]'::jsonb,
  -- 她的判断。verdict 为 NULL = 还没定。
  verdict    text CHECK (verdict IN ('kept','revise','dropped')),
  -- 🚨 没有理由就不落地。verdict 非空时 why 必须非空——这一条在 Go 里校验，
  -- 因为"非空"要按 trim 之后算，SQL 的 CHECK 拦不住一串空格。
  why        text NOT NULL DEFAULT '',
  settled_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX pbl_artifact_atom_idx ON pbl_artifact (atom_id, created_at);

-- ── 工具 ───────────────────────────────────────────────────────────────────
--
-- 🚨 端点留着，交互延后。产品负责人 2026-09-01 的裁决：
-- 「previous card system is a failure, because no one would like those
--   form-like things. we do need a tool box, but the detailed interaction may
--   need to be redesigned.」——延后的是**交互**，不是这件事本身。
--
-- 所以这里记的是"印记召了一件工具、为什么、后来怎么样了"，而没有 spec、没有
-- 字段定义、没有渲染器。等新的交互定下来，它需要的形状进 payload，不用改表。
CREATE TABLE pbl_tool_instance (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  atom_id    uuid NOT NULL REFERENCES atom(id) ON DELETE CASCADE,
  session_id uuid REFERENCES pbl_session(id) ON DELETE SET NULL,
  -- 自由字符串，不是枚举：工具箱是开放的，新工具不该需要一次迁移。
  tool       text NOT NULL,
  -- 印记为什么这时候递这件工具。没有理由的工具是伏击。
  reason     text NOT NULL DEFAULT '',
  -- 用完之后剩下什么。形状等交互定稿；现在是自由 JSON。
  result     jsonb,
  status     text NOT NULL DEFAULT 'summoned'
             CHECK (status IN ('summoned','declined','done')),
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX pbl_tool_atom_idx ON pbl_tool_instance (atom_id, created_at);

-- +goose Down
DROP TABLE pbl_tool_instance;
DROP TABLE pbl_artifact;
