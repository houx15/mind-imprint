-- 0139 · 撤掉 0137。
--
-- 0137 存的是「她对一条推荐说了不感兴趣」。推荐这一层 2026-09-07 从探索地图上
-- 撤掉了：产品负责人看到那一圈星，读不出它们是什么，也不需要在这里被推荐新词
--
--   > is it a recommended keyword? then we don't need to recommend keyword here.
--   > we just show how the five dots connected with students' already existed nodes.
--
-- 地图的外圈现在是**她自己已经有的词**，五颗新闻星连到它们上面。她自己的词
-- 没有「不感兴趣」这个动作 —— 那等于从树上删掉一段真发生过的事。
--
-- 表建了两天，只有冒烟账号的行。留着一张没有代码读的表，比删掉它更难解释。

-- +goose Up
DROP TABLE IF EXISTS interest_dismissal;

-- +goose Down
CREATE TABLE interest_dismissal (
    user_id     uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    interest_id text        NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, interest_id)
);
