-- +goose Up
-- 「继续深挖」—— 一个关键词后面的四颗种子。
--
-- 四种各一颗（想一想 / 去读 / 去写 / 去做），由模型按**她在这个词上留下的原话**
-- 生成。原型在这里摆的是四个通用动词，而四个空动词摆在一个真的观察后面，教的是
-- 这个模型没有真的在看她。
--
-- 生成一次就存着：种子不该每次打开抽屉都换一批 —— 一个每次都给不同建议的教练，
-- 说明它对你没有看法。
CREATE TABLE keyword_dig (
  keyword_id uuid NOT NULL REFERENCES interest_keyword(id) ON DELETE CASCADE,
  kind       text NOT NULL CHECK (kind IN ('think','read','write','make')),
  -- 会被当作标题 / 立意直接送进 readings / writings / pbl_project 的创建接口，
  -- 所以它必须是一句能独立成立的话。应用层挡空串。
  text       text NOT NULL,
  why        text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (keyword_id, kind)
);

-- 这个词的种子在哪一刻生成过。
--
-- 🚨 语义是「尝试过一次」，不是「长出过种子」——第四次写这条注释了
-- （0104 / 0116 / 0120 / 这里）。一次解析失败的结果和「从没生成过」在
-- keyword_dig 里长得一模一样，按产出判断会让她每次打开这个抽屉都重发一次调用。
ALTER TABLE interest_keyword ADD COLUMN dig_at timestamptz;

-- +goose Down
ALTER TABLE interest_keyword DROP COLUMN dig_at;
DROP TABLE keyword_dig;
