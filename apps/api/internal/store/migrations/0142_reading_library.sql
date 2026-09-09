-- 0142 · 分级阅读库：文章带来的图，以及她读的是哪一篇、哪一档。
--
-- 库本身不在这里。二十篇文章、一百个难度版本是内容，存在
-- apps/api/internal/library/articles.json 里由 go:embed 读（那个包的头注解释
-- 了为什么是文件不是表）。进 Postgres 的只有「边」：她开了哪一篇、开的哪一档。
--
-- reading_source 上加的两列是版式，不是正文：
--
--   figures   正文里的图。图不能留在 body 里 —— 阅读室按空行切段、把工具卡挂
--             在段 id 上，一张留在正文里的图会占掉一个段 id，然后被当成一段
--             课文引回给学生。所以每张图记住自己跟在哪一段之后（after，空串
--             表示题图），渲染时插在段与段之间。
--   headings  要当小标题渲染的段 id。小标题仍然是段（Go 的 SplitBlocks 不认
--             识 Markdown），只是渲染成小标题；把 "## " 留在正文里，学生会
--             在页面上看见两个井号。
--
-- 两列都给了默认值，所以粘贴进来的、上传的、从链接抓的那些阅读一行都不用改。
-- reading_source 只有轻量版在用（0092 的原子底座），与 pro 的 project 无关。
--
-- reading 上的两列记来源：library_slug 空串表示这一篇不是从库里来的。有了它，
-- 书架能把读过的划掉，推荐能按她读完过的最高档往下走一档。

-- +goose Up
ALTER TABLE reading_source
    ADD COLUMN figures jsonb NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN headings jsonb NOT NULL DEFAULT '[]'::jsonb;

ALTER TABLE reading
    ADD COLUMN library_slug text NOT NULL DEFAULT '',
    ADD COLUMN library_tier smallint NOT NULL DEFAULT 0
        CHECK (library_tier BETWEEN 0 AND 5);

-- 书架每次打开都要问「这些我读过没有」，问的是这个学生的全部阅读。
CREATE INDEX reading_library_slug_idx ON reading (library_slug)
    WHERE library_slug <> '';

-- +goose Down
DROP INDEX reading_library_slug_idx;
ALTER TABLE reading DROP COLUMN library_tier, DROP COLUMN library_slug;
ALTER TABLE reading_source DROP COLUMN headings, DROP COLUMN figures;
