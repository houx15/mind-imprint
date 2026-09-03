-- +goose Up
-- 她自己找到、贴进来的那些个人网站。
--
-- 产品负责人 2026-09-03，主页项目第二关：「collect data about the websites they
-- love. let them search, and give AI the urls.」
--
-- 这一关她不打字：她在自己的浏览器里搜，把网址粘进来，服务端去读那一页
-- （materialize.FetchReadable，读书房用了很久的那条路），印记回一张卡。所以这张
-- 表存的是**两半**：她带回来的那半（url），和印记读完之后说的那半（what /
-- structure / best）。
--
-- 🚨 分成三列而不是一段摘要，是因为第二关要拿它们做两件不同的事：`structure`
-- 那一列是她搭自己结构时的对照，`best` 那一列是「哪里可以是我独有的」那一问的
-- 材料。混成一段之后，这两件事都得靠她重读一遍摘要自己拆——那正是这一关想替
-- 她省掉的力气。
CREATE TABLE pbl_site_ref (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  atom_id    uuid NOT NULL REFERENCES atom(id) ON DELETE CASCADE,
  -- 归一化之后的网址。同一个项目里不重复——她贴第二遍是手滑，不是第二个例子。
  url        text NOT NULL,
  title      text NOT NULL DEFAULT '',
  -- 印记读完那一页之后回的那张卡，三句。
  what       text NOT NULL DEFAULT '',
  structure  text NOT NULL DEFAULT '',
  best       text NOT NULL DEFAULT '',
  -- 她自己在这一站上补的一句。空 = 她没说话，不是她没意见。
  she_said   text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (atom_id, url)
);

CREATE INDEX pbl_site_ref_atom_idx ON pbl_site_ref (atom_id, created_at);

-- +goose Down
DROP TABLE pbl_site_ref;
