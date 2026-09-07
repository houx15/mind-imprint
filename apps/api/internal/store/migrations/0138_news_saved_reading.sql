-- 0138 · 收一颗星球 = 把它放进阅读室，不再是往树上种一个词。
--
-- 原来按下「收进我的树」会直接种一个词，evidence 用这颗星的钩子。产品负责人
-- 2026-09-07 指出这一步来得太早：她只是在地图上看到一个标题和两句摘要，还没
-- 读过任何东西，树上就多了一个词。树该长在她真的读完之后。
--
-- 所以这一屏的出口改成「现在读 / 稍后读」，两个都落到阅读室里的一篇。这一列
-- 记的就是落到了哪一篇 —— 有了它，重复点不会攒出一堆同名的待读，而且「已经
-- 在阅读室了」这句话点得开。
--
-- 空值是允许的：0138 之前收藏过的行没有对应的阅读。界面按「有没有 reading_id」
-- 决定给「打开」还是给「现在读」。

-- +goose Up
ALTER TABLE news_saved
    ADD COLUMN reading_id uuid REFERENCES atom (id) ON DELETE SET NULL;

-- +goose Down
ALTER TABLE news_saved DROP COLUMN reading_id;
