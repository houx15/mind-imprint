-- +goose Up
-- 阅读的两处加深。
--
-- 一、收尾那一步从「回答几个问题」变成「回去找一句」。打字回答的问题，她可以
-- 凭印象答；回到文章里点出一句，她必须真的再读一遍。所以 quiz 不是被改名，是
-- 被替换——枚举里不留它，就没有哪套读法还能以打字问答收尾。
--
-- 二、每套读法里都有一步是她自己的（connect）。这是**类型**层面的保证，不是
-- prompt 里的一句叮嘱：库里不存在没有这一步的读法。
ALTER TABLE reading_task DROP CONSTRAINT reading_task_kind_check;
UPDATE reading_task SET kind = 'hunt' WHERE kind = 'quiz';
ALTER TABLE reading_task ADD CONSTRAINT reading_task_kind_check
  CHECK (kind IN ('read','focus_block','lens','reflect','connect','hunt'));

-- 读完之后从这篇文章里长出来的问题。每一条都拴着原文里的一句话：
-- 拴不住的问题就是泛泛而谈，落库前会被丢掉。
CREATE TABLE reading_question (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  atom_id      uuid NOT NULL REFERENCES atom(id) ON DELETE CASCADE,
  position     integer NOT NULL,
  text         text NOT NULL,
  anchor_quote text NOT NULL,
  anchor_block text NOT NULL DEFAULT '',
  created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX reading_question_pos_idx ON reading_question (atom_id, position);

-- +goose Down
DROP TABLE reading_question;
ALTER TABLE reading_task DROP CONSTRAINT reading_task_kind_check;
UPDATE reading_task SET kind = 'quiz' WHERE kind IN ('hunt','connect');
ALTER TABLE reading_task ADD CONSTRAINT reading_task_kind_check
  CHECK (kind IN ('read','focus_block','lens','reflect','quiz'));
