-- +goose Up
-- 领域闭表 —— 树上的词从模型自由造，改成从一张两百多条的表里选。
--
-- 为什么要清库：2026-09-02 那版采集器让模型自己命名她的兴趣，长出来的是
-- 「例外与代表性」「一个结论要多少证据」这种粒度的词。它们和新词表里的
-- 「金融」「裁缝」「游戏」不是一回事，混在同一棵树上，那棵树说不清自己是什么。
-- Owner 的决定是**重采**：清掉，让扫尾任务按新词表重新长一遍。
--
-- 清掉的是学生看得见的东西，所以先整表归档。归档表不进任何查询，只是让这一步
-- 可逆 —— 一次不可逆的删除不该因为「反正会重新长出来」就省掉退路。

CREATE TABLE interest_keyword_v1_archive AS SELECT * FROM interest_keyword;
CREATE TABLE keyword_source_v1_archive   AS SELECT * FROM keyword_source;
CREATE TABLE keyword_discipline_v1_archive AS SELECT * FROM keyword_discipline;

-- interest_id：这个词在 packages/contracts/interests/interests.json 里的 id。
--
-- 不是外键：词表和学科表一样不在库里（它是内容不是用户数据），Go 侧用
-- interests.Exists 校验之后才允许写进来。可空是为了让归档之前的历史行仍然合法；
-- 清库之后每一行都会有值。
ALTER TABLE interest_keyword ADD COLUMN interest_id text;
CREATE INDEX interest_keyword_interest_idx ON interest_keyword (interest_id);

-- how 多一档 'catalog'：这条边来自词表里写好的 disciplines[]，不是模型判定的。
-- 它比 alias/cooccur/llm 都确定，因为它是人写的。
ALTER TABLE keyword_discipline DROP CONSTRAINT keyword_discipline_how_check;
ALTER TABLE keyword_discipline ADD CONSTRAINT keyword_discipline_how_check
  CHECK (how IN ('alias', 'cooccur', 'llm', 'student', 'catalog'));

-- 清空。三张表有级联，但显式按顺序删，读的人不用去查约束才知道会发生什么。
DELETE FROM keyword_discipline;
DELETE FROM keyword_source;
DELETE FROM interest_keyword;

-- 让扫尾任务重新采一遍。语义仍然是「尝试过一次」，只是这一轮的尝试作废了。
UPDATE atom SET interest_harvested_at = NULL;

-- 星图上的关键词同样要出自词表，否则收藏一颗星就把闭表之外的词放回树上。
-- 旧值是自由文本，不是 id，所以连同当天的星图一起作废 —— 星图本来就是按天
-- 惰性生成的，明天（或下一次打开探索地图）会用新 prompt 重新生成。
ALTER TABLE news_planet RENAME COLUMN keyword TO interest_id;
ALTER TABLE news_planet ALTER COLUMN interest_id DROP NOT NULL;
ALTER TABLE news_planet ALTER COLUMN interest_id DROP DEFAULT;
UPDATE news_planet SET interest_id = NULL;
DELETE FROM news_day;

-- +goose Down
DELETE FROM news_day;
ALTER TABLE news_planet ALTER COLUMN interest_id SET DEFAULT '';
UPDATE news_planet SET interest_id = '' WHERE interest_id IS NULL;
ALTER TABLE news_planet ALTER COLUMN interest_id SET NOT NULL;
ALTER TABLE news_planet RENAME COLUMN interest_id TO keyword;

ALTER TABLE keyword_discipline DROP CONSTRAINT keyword_discipline_how_check;
ALTER TABLE keyword_discipline ADD CONSTRAINT keyword_discipline_how_check
  CHECK (how IN ('alias', 'cooccur', 'llm', 'student'));

DROP INDEX interest_keyword_interest_idx;
ALTER TABLE interest_keyword DROP COLUMN interest_id;

DROP TABLE keyword_discipline_v1_archive;
DROP TABLE keyword_source_v1_archive;
DROP TABLE interest_keyword_v1_archive;
