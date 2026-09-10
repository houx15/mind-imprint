-- 导读：一句话 + 结构 + 每一段的承重。
--
-- 排读法那一次调用（reading_plan.go）本来就要把全文读一遍并挑出重点段，
-- 这次让它顺带产出三样东西，存成一份 jsonb：
--
--   {"oneLine": "这篇在问什么（不是它的结论）",
--    "shape":   "问题 → 证据 → 让步 → 结论",
--    "load":    {"b1":"core","b2":"support","b3":"bridge"}}
--
-- 它落在 reading_source 而不是 reading，因为这三样都是**关于这篇文章**的判断，
-- 和正文、图、小标题是同一类东西；reading 那一行记的是她这一次阅读。
--
-- 为什么要有它：印记 开场那一轮原来的活儿是「介绍一下你排的读法」，同时又被
-- 要求「不要说出这篇文章的结论」。这是一道自相矛盾的题，真模型的答案是把文章
-- 讲了一遍。导读卡由前端确定性地渲染之后，那一轮就没有内容可讲了，
-- 它只剩下递第一张卡片这一件事。见 docs/2026-09-10-reading-guidance-redesign.md。
--
-- 加列，不改列：老的阅读读到 '{}' —— 没有导读卡，其余一切照旧。

-- +goose Up
ALTER TABLE reading_source
    ADD COLUMN outline jsonb NOT NULL DEFAULT '{}'::jsonb;

-- +goose Down
ALTER TABLE reading_source
    DROP COLUMN outline;
