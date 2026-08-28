-- +goose Up
-- Task 8 fix round 1：「长出的问题」不能靠 reading_question 有没有行来判断
-- 「生成过没有」。
--
-- 一篇很薄的文章可能一条问题都长不出来——validateReadingQuestions 要求至少
-- 两条能拴住原文的问题才展示，不够两条就一条都不存。如果「有没有生成过」是
-- 靠 reading_question 表里有没有行来判断，这种「生成了、但结果是零条」的
-- 状态和「还没生成过」在数据库里长得一模一样，于是她每次重新打开这篇已完成
-- 的阅读，后端都会当作「还没生成」再调一次旗舰模型——无限重复付费，且每次
-- 屏幕上什么都不会多出来。
--
-- 这条迁移把「有没有生成过」从 reading_question 的行数里挪出来，单独记在
-- reading 行本身：questions_at 非空 = 生成尝试已经跑过一次（无论跑出了几条），
-- NULL = 还没跑过。已完成读完得手写去筛一遍是否「零条也算生成过」；不如让
-- 默认状态本身诚实，谁读这一列都不用记得再过滤一次——这也是为什么
-- ListAtomMessages 把 block_id IS NULL 的筛选写进 SQL 本身，而不是要求七个
-- 调用方各自记得筛一次。
--
-- 不需要回填：迁移前的每一行都真的从未跑过这次生成，NULL 对它们而言就是
-- 事实，不是近似。不给默认值，也是因为「生成过」不该有默认答案。
ALTER TABLE reading ADD COLUMN questions_at timestamptz;

-- +goose Down
ALTER TABLE reading DROP COLUMN questions_at;
