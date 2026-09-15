-- +goose Up
-- 写作的提交版本。每次点「完成」生成一版；版本不修改、不删除。
-- word_count 的数法近似 agent.CountWords（部分汉字子区间、部分空白字符上两者不完全一致）：
-- 汉字、假名、全角字符各算一个，其余连续的非空白字符算一个。
CREATE TABLE writing_version (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  atom_id      uuid NOT NULL REFERENCES atom(id) ON DELETE CASCADE,
  number       integer NOT NULL CHECK (number >= 1),
  title        text NOT NULL,
  body         text NOT NULL,
  word_count   integer NOT NULL CHECK (word_count >= 0),
  submitted_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (atom_id, number)
);

-- 完成之后重新开始修改的时间。NULL = 不在修改中。
ALTER TABLE writing ADD COLUMN revising_at timestamptz;

-- 老师退回修改：退回时间、新的截止时间、说明。再次退回时覆盖这三列。
ALTER TABLE lite_assignment_recipient
  ADD COLUMN returned_at   timestamptz,
  ADD COLUMN return_due_at timestamptz,
  ADD COLUMN return_note   text CHECK (return_note IS NULL OR char_length(return_note) <= 500);

-- 已完成的写作补第一版：当前草稿，提交时间取 finished_at。
-- 正则的第一段是单个 CJK 字符（近似 agent.isCJK 的范围），第二段是一串非空白、非 CJK 字符。
INSERT INTO writing_version (atom_id, number, title, body, word_count, submitted_at)
SELECT w.atom_id, 1, w.title, COALESCE(d.body, ''),
       (SELECT count(*) FROM regexp_matches(
          COALESCE(d.body, ''),
          '[\u2e80-\u2fdf\u3005\u3007\u3021-\u3029\u3038-\u303b\u3040-\u30ff\u3400-\u4dbf\u4e00-\u9fff\uf900-\ufaff\uff00-\uffef\U00020000-\U0003134f]|[^\s\u3000\u2e80-\u2fdf\u3005\u3007\u3021-\u3029\u3038-\u303b\u3040-\u30ff\u3400-\u4dbf\u4e00-\u9fff\uf900-\ufaff\uff00-\uffef\U00020000-\U0003134f]+',
          'g'))::int,
       COALESCE(w.finished_at, w.updated_at)
FROM writing w
LEFT JOIN writing_draft d ON d.atom_id = w.atom_id
WHERE w.status = 'finished';

-- +goose Down
DROP TABLE writing_version;
ALTER TABLE writing DROP COLUMN revising_at;
ALTER TABLE lite_assignment_recipient
  DROP COLUMN return_note,
  DROP COLUMN return_due_at,
  DROP COLUMN returned_at;
