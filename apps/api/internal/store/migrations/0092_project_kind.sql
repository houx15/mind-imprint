-- +goose Up
-- 轻量版（lite edition）的写作/阅读原子各自静默持有一行 project 作为存储锚点
-- （reference / material / card_instances / outline_node / snippet /
-- draft_snapshot 等全部外键到 project）。kind 把这些容器行与真正的项目分开，
-- 好让每一处项目列表与聚合都能把它们排除掉。
ALTER TABLE project ADD COLUMN kind text NOT NULL DEFAULT 'project'
  CHECK (kind IN ('project','container'));
CREATE INDEX project_kind_idx ON project (kind);

-- +goose Down
DROP INDEX IF EXISTS project_kind_idx;
ALTER TABLE project DROP COLUMN kind;
