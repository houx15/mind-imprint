-- 0141 —— 星球带上 feed 自己给的正文。
--
-- # 为什么要存
--
-- 「现在读」建的那一篇**没有正文**：我们只把标题和链接送进阅读室，然后再去抓
-- 一次原页面。抓得到就有正文，抓不到（出版方 403 / 反爬 / 付费墙）就摆一个
-- 粘贴框，让她自己把文章贴进来。走查里抓的那一次就是 400。
--
-- 而很多源的正文**本来就在 feed 里**（content:encoded），生成星图那一刻就已经
-- 在我们手上了 —— 免费、不会被挡、不用第二次 HTTP。存下来，她点进去就能读。
--
-- 这一列不是阅读室的东西，它就是**取正文的第三条路**，和「抓原页面」「她自己
-- 粘」并列，产出完全一样，最后都落进 reading_source。
--
-- # 为什么允许为空
--
-- 半数源的 feed 里没有正文（Aeon / Psyche / ScienceDaily / Phys.org 实测都只有
-- 摘要）。空串就是「这条路这次没走通」，另外两条路照旧。

-- +goose Up
ALTER TABLE news_planet ADD COLUMN body text NOT NULL DEFAULT '';

COMMENT ON COLUMN news_planet.body IS
  'feed 自带的正文（content:encoded），已清成纯文本、段落之间一个空行。空串 = 这个源的 feed 没带正文。';

-- +goose Down
ALTER TABLE news_planet DROP COLUMN body;
