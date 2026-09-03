-- +goose Up
-- 今日新闻星图 —— 每天五颗星。
--
-- **全局，不分学校。** 今天的科学新闻对每个学生都是同一批，按学校分只会把同一次
-- 抓取和同一次模型调用乘以学校数。将来若要按学校调口味，加一列比拆一张表容易。
--
-- 一天一行 × 五。`UNIQUE (day, rank)` 是幂等的抓手：两个学生在同一秒第一次打开
-- 星图，谁先拿到 advisory lock 谁生成，另一个直接读（见 internal/api/explore.go）。
CREATE TABLE news_planet (
  id       uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  -- 这颗星属于哪一天。按服务器时区取日期即可 —— 学生和服务器都在东八区。
  day      date NOT NULL,
  rank     int  NOT NULL CHECK (rank BETWEEN 1 AND 5),

  -- title_zh 是**重写过的**，不是原标题的翻译：一句 20 字以内的中文，说出这件事
  -- 本身。原标题留在 title_en 里，因为她点进去看到的是英文，两个对不上会很怪。
  title_zh text NOT NULL,
  title_en text NOT NULL DEFAULT '',
  summary  text NOT NULL DEFAULT '',

  -- hook 是这一屏存在的理由：一个**她能自己追问的问题**。NOT NULL 且应用层挡空
  -- 串——一颗没有钩子的星球是一条只能被记住、不能被追问的新闻。
  hook text NOT NULL,

  url         text NOT NULL,
  source_name text NOT NULL,

  field         text NOT NULL CHECK (field IN
                  ('formal','science','making','society','humanities','arts','self')),
  -- 指向 disciplines.json 的 id，可为空串：学科连错比没连上糟，所以模型给了一个
  -- 不存在的 id 时我们清空这条边，但**不因此扔掉一条好新闻**。
  discipline_id text NOT NULL DEFAULT '',
  -- 她收藏这颗星时，种进树的那个词。
  keyword text NOT NULL DEFAULT '',

  published_at timestamptz,
  created_at   timestamptz NOT NULL DEFAULT now(),
  UNIQUE (day, rank)
);

CREATE INDEX news_planet_day_idx ON news_planet (day DESC);

-- 她把哪几颗星收进了自己的树。
--
-- 收藏是**一次动作**，不是一个书签开关：它会往 interest_keyword 里种一个词，
-- 附上这颗星的钩子作为 evidence。所以这里没有「取消收藏」——那个词来自一件
-- 真的发生过的事。她想改自己的树是另一件事（学生覆写，how='student'）。
CREATE TABLE news_saved (
  user_id   uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  planet_id uuid NOT NULL REFERENCES news_planet(id) ON DELETE CASCADE,
  saved_at  timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, planet_id)
);

-- 一天的生成尝试。**盖章在生成之前**，语义是「今天试过一次」而不是「今天出过
-- 星图」——所有源都挂掉的那天，结果和「从没试过」在 news_planet 里长得一模一样，
-- 按产出判断会让每个打开星图的学生都触发一次全量抓取 + 一次旗舰调用。
-- 这条教训来自 reading.questions_at（0104）和 atom.interest_harvested_at（0116）。
CREATE TABLE news_day (
  day          date PRIMARY KEY,
  attempted_at timestamptz NOT NULL DEFAULT now(),
  -- 实际出了几颗星。0 是合法值，它的意思是「今天试过，没凑出来」。
  planet_count int NOT NULL DEFAULT 0,
  -- 失败时的原话，直接显示给她和我们看（界面文案 §8）。
  note text NOT NULL DEFAULT ''
);

-- +goose Down
DROP TABLE news_day;
DROP TABLE news_saved;
DROP TABLE news_planet;
