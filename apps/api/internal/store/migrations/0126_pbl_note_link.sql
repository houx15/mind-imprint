-- +goose Up
-- 两张便签之间的关系。
--
-- 🚨 板上的意义不在单张纸上，在两张纸之间。「走廊上站着 14 个人」和「教室里坐
-- 不住」各自都只是一条记录；把它们连起来说「这个导致那个」，才是一次判断。
--
-- 四种关系里**矛盾**最要紧：两条都是她亲眼看到的，却互相打架——真正的问题几乎
-- 都是从那儿长出来的。所以这张表存在的首要理由，是让「我这儿有两条对不上的
-- 观察」有地方待着，而不是烂在她脑子里。
CREATE TABLE pbl_note_link (
  id       uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  -- 冗余一份 atom_id：回灌要按项目取，不然得两次 join 回便签表。
  atom_id  uuid NOT NULL REFERENCES atom(id) ON DELETE CASCADE,
  from_id  uuid NOT NULL REFERENCES pbl_note(id) ON DELETE CASCADE,
  to_id    uuid NOT NULL REFERENCES pbl_note(id) ON DELETE CASCADE,
  relation text NOT NULL
           CHECK (relation IN ('causes', 'contradicts', 'same', 'supports')),
  created_at timestamptz NOT NULL DEFAULT now(),
  -- 同一对纸之间同一种关系连一次就够。她再连一次是想改关系，不是想要两条线。
  UNIQUE (from_id, to_id, relation)
);
CREATE INDEX pbl_note_link_atom_idx ON pbl_note_link (atom_id);

-- +goose Down
DROP TABLE pbl_note_link;
