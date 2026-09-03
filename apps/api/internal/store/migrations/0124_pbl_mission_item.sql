-- +goose Up
-- 出门前的观察清单。
--
-- 🚨 「观察日记」这件工具一直没有 before-state。她带着 tool.reason 那一段话出门，
-- 回来面对的是几个空白文本框——产品负责人 2026-09-02 说它 boring，根子在这儿。
--
-- docs/2026-09-01-pbl-detail.md 要的是「a small real-world mission」加「a simple
-- observation method」。方法落在这张表上：印记把要看的事拆成 3–5 条具体的观察点，
-- 每条说清楚要带回哪一类东西（数一数 / 听一句原话 / …）。她在现场一条一条点掉，
-- 回来时那几条已经是填好类别的便签底稿。
--
-- 没点掉的那几条同样要紧：「第 2 条（听一句原话）没做到」是铁律④要的信号，
-- 而今天它压根没有地方存，做没做到在系统里毫无区别。
CREATE TABLE pbl_mission_item (
  id      uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  -- 挂在那一次「观察日记」上，不是挂在项目上：同一个项目可以出门看好几趟，
  -- 每一趟有自己的清单。
  tool_id uuid NOT NULL REFERENCES pbl_tool_instance(id) ON DELETE CASCADE,
  -- 要她去看的那一件事，印记写的一句话。
  prompt  text NOT NULL,
  -- 这一条想带回哪一类东西。空 = 印记没指定，她自己判断。
  -- 值跟 pbl_note.kind 对齐，回来时便签的类别就是从这儿来的。
  want_kind text NOT NULL DEFAULT ''
            CHECK (want_kind IN ('', 'observation', 'quote', 'assumption', 'question')),
  ordinal integer NOT NULL DEFAULT 0,
  -- 她在现场点掉这一条的时刻。NULL = 还没做到。
  done_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX pbl_mission_item_tool_idx ON pbl_mission_item (tool_id, ordinal);

-- +goose Down
DROP TABLE pbl_mission_item;
