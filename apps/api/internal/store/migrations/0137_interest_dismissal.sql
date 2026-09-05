-- 0137 · 她按过「不感兴趣」的领域。
--
-- 探索地图上的推荐是前端从闭表算出来的纯函数（`explore/recommend.ts`），不需要
-- 后端参与。**只有「不感兴趣」需要落库**：它是她主动说的一句话，AGENTS.md 的
-- 铁律④「过程即数据」说的就是这种东西 —— 她拒绝了什么，和她做了什么一样是信号。
--
-- 为什么不放 localStorage：换一台设备就没了，而且我们读不到。一条推荐被拒绝
-- 是这一屏唯一的负反馈，丢掉它等于这一屏只会说话不会听。
--
-- interest_id 不加外键：领域词表是 go:embed 的内容，不是数据库里的表。同
-- interest_keyword.interest_id 的处理。词表里删掉一条词时，这里留下的孤儿行读
-- 出来只是一个再也不会被推荐的 id，不影响任何东西。

-- +goose Up
CREATE TABLE interest_dismissal (
    user_id     uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    interest_id text        NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, interest_id)
);

-- 取数永远是「这个学生拒过哪些」，所以主键的前缀就够，不再加索引。

-- +goose Down
DROP TABLE interest_dismissal;
