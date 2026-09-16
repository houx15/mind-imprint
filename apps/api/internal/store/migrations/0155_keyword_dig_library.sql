-- 0155 —— 「去读」那颗种子指向阅读库里真的有的那一篇。
--
-- # 它修的是什么
--
-- 「去读」原来是一句模型现写的话（prompt 里的示范就是「反对建更多太阳能农场的
-- 人在担心什么」），点下去用这句话当标题新建一篇空的阅读。写成一个话题的时候
-- 还算诚实，写成一个标题的时候就不是了 —— 产品负责人 2026-09-16 的原话：
--
--   > in the interest tree, sometimes we would recommend some papers that don't
--   > exist. we can only recommend readings in our database, if there is not
--   > suitable ones, then we don't recommend. don't fake these articles.
--
-- 所以这颗种子现在**必须**落在 internal/library 的目录上：候选由服务端按她的
-- 兴趣算出来送进 prompt，模型只在里面挑一个 slug，挑了目录里没有的就整颗丢掉。
-- 一篇都没挑中就是没有「去读」那一颗 —— 三颗种子比四颗里有一颗是假的好。
--
-- # 为什么要存这两列
--
-- 种子是生成一次就存着的（见 0121），而「点下去开哪一篇」必须和当初生成的那次
-- 是同一篇。把 slug 放进 text 里解析回来是第二份真相；存成两列，点击那一步就是
-- 一次 POST /library/{slug}/levels/{tier}，和书架走的是同一条路。
--
-- 两列都允许为空：think / write / make 三种种子本来就没有文章，而 0121 之前
-- 已经生成过的那些 read 种子也没有。空 slug = 这颗种子没有落到库上，前端据此
-- 不显示「在阅读室打开」。

-- +goose Up
ALTER TABLE keyword_dig ADD COLUMN library_slug text NOT NULL DEFAULT '';
ALTER TABLE keyword_dig ADD COLUMN library_tier int  NOT NULL DEFAULT 0;

COMMENT ON COLUMN keyword_dig.library_slug IS
  '分级阅读库里的 slug（internal/library/articles.json）。空串 = 这颗种子没有落到库上。';
COMMENT ON COLUMN keyword_dig.library_tier IS
  '开哪一档（1..5）。0 = 没有落到库上。';

-- +goose Down
ALTER TABLE keyword_dig DROP COLUMN library_tier;
ALTER TABLE keyword_dig DROP COLUMN library_slug;
