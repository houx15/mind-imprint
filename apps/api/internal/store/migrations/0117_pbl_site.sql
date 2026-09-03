-- +goose Up
-- S5 · 她的主页。spec §15 里那张 `pbl_site`——她接受过的每一轮改动，最后落成的
-- 那一个页面。
--
-- 🚨 主键是 user_id，不是 atom_id。spec §14 把两件东西分得很清楚：kind='site'
-- 的 artifact 是**一轮交付**（我做了什么、我猜了什么、哪里还不对），一轮一轮往上
-- 加；`pbl_site` 是那些轮次最后落成的**那一个**页面，是单数。一个学生只有一个
-- 主页——§3 说的是同一件事：「唯一的例外是她自己的网站」。atom_id 只记住是哪个
-- 项目把它建起来的；项目被删掉主页还在（ON DELETE SET NULL），因为页面是她的，
-- 不是项目的附属物。
--
-- 🚨 没有 revoked_at 这一列，虽然 spec §16 的表格里写了。撤销就是把 share_token
-- 置空——这是 atom_report_share.go 已经在跑的那条路径，而且它的性质更强：下一个
-- 请求就查不到了，没有缓存、没有宽限期。多一列 revoked_at 意味着存在
-- 「token 还在、但应该失效了」的中间态；那是一个可以被读错的状态，也就是一个
-- 迟早会漏的状态。少一列，撤销就不可能只撤一半。
CREATE TABLE pbl_site (
  user_id      uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,

  -- 建起它的那个项目。SET NULL：项目没了，页面还是她的。
  atom_id      uuid REFERENCES atom(id) ON DELETE SET NULL,

  -- 三个真正不同的版式，不是一个版式的三种配色（spec §15）。她挑哪个，页面就
  -- 是哪一页。
  layout       text NOT NULL DEFAULT 'essay'
               CHECK (layout IN ('essay','ledger','magazine')),

  -- 她为什么挑这一个。设计原则：「没有写下理由，什么都不落定」。空串表示还没
  -- 选定；服务端在落定 layout 时强制它非空（见 pbl_site.go 的 gate）。
  layout_why   text NOT NULL DEFAULT '',

  -- 只存**她自己写的那些字**：headline / lead / role / about / now / 联系方式 /
  -- 每条作品与文章她自己写的那句话。文章、作品、在读这三张列表不存在这里——它们
  -- 每次渲染时从她真实的 reading / writing / pbl_project 行现拼（见
  -- internal/pbl/site.go）。存一份快照等于存一份会过期的副本，而这个产品最不该
  -- 做的事，就是把她的主页变成一张与她真实做过的事对不上的旧照片。
  content      jsonb NOT NULL DEFAULT '{}'::jsonb,

  -- 未发布时为 NULL。撤销 = 置回 NULL，见文件头。
  share_token  text,
  published_at timestamptz,

  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now()
);

-- 部分唯一索引：token 必须唯一，但「还没发布」的行有很多，NULL 之间不该互相冲突。
CREATE UNIQUE INDEX pbl_site_share_token_key
  ON pbl_site (share_token) WHERE share_token IS NOT NULL;

-- +goose Down
DROP INDEX pbl_site_share_token_key;
DROP TABLE pbl_site;
