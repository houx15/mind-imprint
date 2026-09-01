-- +goose Up
-- PBL 项目接进 atom 底座：一个项目和一篇阅读、一篇写作一样，是一颗原子。
--
-- 为什么不是新起一张根表：atom 已经带着轻量版全部的共享机制——归属校验
-- （loadOwnedAtom）、最近活跃（last_activity_at）、专注时长（active_seconds）
-- 与心跳、对话（atom_message）、报告（atom_report）、以及计费用的
-- llm_call.atom_id（0094）。项目自己再长一套，等于把同一件事实现两遍，而且
-- 第二遍是没被测过的那一遍。reading / writing 都是 atom_id 主键的细节表，
-- 项目照做。
--
-- 🚨 表名和路由都带 pbl 前缀：pro 已经有 project 表和 /api/v1/projects 路由，
-- 而且 edition_test.go 里有一条测试明确要求轻量版访问 /api/v1/projects 得到
-- 404。轻量版最容易搞坏 pro 的方式，就是新建一个已经存在的名字。
ALTER TABLE atom DROP CONSTRAINT atom_kind_check;
ALTER TABLE atom ADD CONSTRAINT atom_kind_check
  CHECK (kind IN ('reading','writing','project'));

CREATE TABLE pbl_project (
  atom_id      uuid PRIMARY KEY REFERENCES atom(id) ON DELETE CASCADE,
  -- 她原样写进大输入框的那段话。永远保留：项目改过名之后，这仍然是她当初
  -- 自己的说法，也是过程评估唯一能读到的「起点」。
  idea         text NOT NULL,
  -- 印记读完 idea 判的类型。website 由 spec §4 的规则强制，不由分类器决定。
  kind         text NOT NULL
               CHECK (kind IN ('website','research','design','making','investigation')),
  -- 名字和封面在弹窗里由她给。项目要先存在，她才能给它起名，所以这里默认空串，
  -- 而不是 NOT NULL 无默认。空名字在看板上由 idea 顶替，不是错误状态。
  name         text NOT NULL DEFAULT '',
  cover_ground text NOT NULL DEFAULT '',
  cover_glyph  text NOT NULL DEFAULT '',
  status       text NOT NULL DEFAULT 'talking'
               CHECK (status IN ('talking','running','review','keeping','archived')),
  updated_at   timestamptz NOT NULL DEFAULT now()
);

-- 看板按状态分列；列内的排序用的是 atom.last_activity_at，所以这里只给 status。
CREATE INDEX pbl_project_status_idx ON pbl_project (status);

-- +goose Down
DROP TABLE pbl_project;
ALTER TABLE atom DROP CONSTRAINT atom_kind_check;
ALTER TABLE atom ADD CONSTRAINT atom_kind_check CHECK (kind IN ('reading','writing'));
