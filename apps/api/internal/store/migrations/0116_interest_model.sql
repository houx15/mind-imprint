-- +goose Up
-- 兴趣模型 —— 她的那棵树，以及它和学科表之间的边。
--
-- 树做的断言很强，必须挣得到：**树上每一个词，都来自她真的做过的一件事。**
-- 所以 keyword_source.evidence（她自己的那句话）是 NOT NULL：没有原话的词是
-- 装饰，抽屉里也没有东西可以给她看。应用层还会把空串一起挡掉。
--
-- 学科表本身不在这里 —— 它是内容不是用户数据，住在
-- packages/contracts/disciplines/disciplines.json（Go 侧 go:embed 一份副本）。
-- 进库的只有「边」：keyword_discipline。
CREATE TABLE interest_keyword (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id       uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  text_zh       text NOT NULL,
  text_en       text NOT NULL DEFAULT '',
  -- 归一化后的 text_zh。去重、别名匹配、唯一约束都靠它，实现是
  -- internal/disciplines.Normalize —— 一个实现，所以几处不会对「两个词是不是
  -- 同一个」有不同看法。
  norm          text NOT NULL,
  field         text NOT NULL CHECK (field IN
                  ('formal','science','making','society','humanities','arts','self')),
  -- 读数，不是等级。由不同来源的条数推出来（internal/interest.Strength），
  -- 任何地方都不该手填。
  strength      int  NOT NULL DEFAULT 1 CHECK (strength BETWEEN 1 AND 5),
  -- 印记对这个词之于她的一句话。
  note          text NOT NULL DEFAULT '',
  first_seen_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (user_id, norm)
);

-- 一个词的来源。同一篇只算一次（UNIQUE），否则反复打开一篇阅读就能把强度刷满。
CREATE TABLE keyword_source (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  keyword_id  uuid NOT NULL REFERENCES interest_keyword(id) ON DELETE CASCADE,
  kind        text NOT NULL CHECK (kind IN ('reading','writing','project','news','quiz')),
  -- 对应的 atom。news 与 quiz 可以没有 atom，所以可空。
  ref_id      uuid,
  label       text NOT NULL,
  -- 她自己的一句话，或让这个词出现的那句话。空的来源不该存在。
  evidence    text NOT NULL,
  happened_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (keyword_id, kind, ref_id)
);

-- 词 → 学科的边。
--
-- how 记下这条边是怎么来的，因为几档的可信度不一样，界面上也要说得出「凭什么
-- 把这个词放进统计推断」：
--   alias   别名直接命中，免费且确定
--   cooccur 和一个已路由的词共享 >=2 条来源，继承它的学科
--   llm     前两档都没命中时才花的那一次模型调用
--   student 她自己在抽屉里改的 —— 永远压过前三档
CREATE TABLE keyword_discipline (
  keyword_id    uuid NOT NULL REFERENCES interest_keyword(id) ON DELETE CASCADE,
  -- 指向 disciplines.json 里的 id。不是外键：学科表不在库里，Go 侧用
  -- disciplines.ByID 校验之后才允许写进来。
  discipline_id text NOT NULL,
  confidence    real NOT NULL CHECK (confidence >= 0 AND confidence <= 1),
  how           text NOT NULL CHECK (how IN ('alias','cooccur','llm','student')),
  rationale     text NOT NULL DEFAULT '',
  PRIMARY KEY (keyword_id, discipline_id)
);

CREATE INDEX interest_keyword_user_idx ON interest_keyword (user_id);
CREATE INDEX keyword_source_keyword_idx ON keyword_source (keyword_id);

-- 采集在哪一刻对这个 atom 跑过。
--
-- 语义是「尝试过一次，无论结果如何」，不是「长出过词」—— 一篇很薄的阅读完全
-- 可能一个词都采不出来，而那个结果和「从没采过」在 interest_keyword 里长得一
-- 模一样。按「有没有长出词」来判断，就会对同一篇薄阅读每次打开都重发一次旗舰
-- 调用，永远采不到东西，永远重来。这条教训来自 reading.questions_at（迁移
-- 0104），代价一样，解法一样。
ALTER TABLE atom ADD COLUMN interest_harvested_at timestamptz;

-- +goose Down
ALTER TABLE atom DROP COLUMN interest_harvested_at;
DROP TABLE keyword_discipline;
DROP TABLE keyword_source;
DROP TABLE interest_keyword;
