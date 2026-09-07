-- 0140 · 读完一篇之后提出来的候选词，等她自己认。
--
-- 在这之前，采集出来的词是**直接种上树的**：她读完一篇，后台一次调用抽出几个
-- 领域，树上就多了几个词，她既没被问过，也不一定知道。产品负责人 2026-09-07：
--
--   > after reading finished, we would get a report, on that we can propose
--   > several keywords, that students can agree to add to their tree
--   > (similar in writing, do you think this would be better?)
--
-- 是更好。这棵树的整个说法是「这就是你的模型」，而一个她没点过头的模型只是我们
-- 对她的记录。让她认一遍，「你凭什么这么说我」这个问题就有了她自己给的答案。
-- 而且**她拒掉的那几个同样是过程数据**（铁律④）—— 拒绝落在这张表上，不是丢掉。
--
-- # 只有新词要问
--
-- 树上已经有的那个词不进这张表：那是给一个已经认过的词再添一条来源，强度往上
-- 走。为一个她三个月前就认过的词再问一次，是把同意变成了打卡。
--
-- # 只有阅读和写作走这条路
--
-- 它们有报告，而报告是这几个候选词唯一有地方待的位置。项目和兴趣测试照旧直接
-- 种 —— 兴趣测试本身就是她在挑词，那一步已经是同意了。

-- +goose Up
CREATE TABLE interest_proposal (
    user_id     uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    -- 提出它的那件事（一篇阅读 / 一篇写作）。报告按它取这一组。
    atom_id     uuid        NOT NULL REFERENCES atom (id) ON DELETE CASCADE,
    -- 领域词表里的 id。同 interest_keyword.interest_id，不加外键：词表是
    -- go:embed 的内容，不是库里的表。写进来之前由 interests.Exists 校验。
    interest_id text        NOT NULL,
    -- 印记对这个词之于她的一句话，和她自己写的那一句。她要看着这两句决定认不认。
    note        text        NOT NULL DEFAULT '',
    evidence    text        NOT NULL DEFAULT '',
    created_at  timestamptz NOT NULL DEFAULT now(),
    -- 她做决定的那一刻。NULL = 还没决定。
    decided_at  timestamptz,
    -- 认了没有。NULL = 还没决定；false = 她说不要，这一条留着。
    accepted    boolean,
    PRIMARY KEY (user_id, atom_id, interest_id)
);

-- 取数永远是「这一篇提了哪几个」，主键的前两段就够。

-- +goose Down
DROP TABLE interest_proposal;
