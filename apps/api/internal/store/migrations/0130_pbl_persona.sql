-- +goose Up
-- 第一关：她的页面给谁看。
--
-- 产品负责人 2026-09-03：「think about my story from audience's view. compose an
-- audience - who is he/she? why he/she knows you? what they are willing to see?
-- what you are liking to show? what feeling you want to bring to them? draw a
-- persona board with ai generated photos, and tagged keywords.」
--
-- 这一关回答的是整个项目的驱动问题（「我想让谁，看见我的什么？」）。她的动作是
-- **挑和否**，不是填：印记先做出两三个可能的受众，她留下一个。
--
-- 🚨 portrait_key 存的是**我们自己 OSS 的 key**，不是上游那个地址。上游回的地址
-- 带签名、会过期（见 internal/gateway/images.go 的实测记录），存它等于几天后
-- 这块板上全是碎图。
CREATE TABLE pbl_persona (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  atom_id      uuid NOT NULL REFERENCES atom(id) ON DELETE CASCADE,
  -- 这个人是谁。一个短称呼，不是名字——他是一类读者，不是一个具体的人。
  label        text NOT NULL,
  -- 他为什么会知道她。
  why_knows    text NOT NULL DEFAULT '',
  -- 他想看到什么。
  wants        text NOT NULL DEFAULT '',
  -- 这一页该给他什么感觉。
  feeling      text NOT NULL DEFAULT '',
  -- 关键词。第二关拿它对结构，第三关拿它派生配色。
  keywords     jsonb NOT NULL DEFAULT '[]'::jsonb,
  -- 生成的画像在我们自己 OSS 里的 key。空 = 还没画。
  portrait_key text NOT NULL DEFAULT '',
  -- 她留下的那一个。一个项目最多一个。
  chosen       boolean NOT NULL DEFAULT false,
  created_at   timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX pbl_persona_atom_idx ON pbl_persona (atom_id, created_at);

-- 一个项目只能有一个「留下的受众」。在库里拦住，而不是靠每个写入点自己记得先
-- 清掉别的——忘一次，第三关的配色就会从两个互相矛盾的受众里派生。
CREATE UNIQUE INDEX pbl_persona_one_chosen ON pbl_persona (atom_id) WHERE chosen;

-- +goose Down
DROP TABLE pbl_persona;
