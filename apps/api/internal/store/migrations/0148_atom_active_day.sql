-- +goose Up
-- 每一次心跳按北京日期落一格。atom.active_seconds 是累计值，拆不出「上周」；
-- 教师端的本周时长与上周表现总结都从这张表按日期求和。
-- 不回填：迁移之前的日子没有格子，界面显示「—」，不估算。
CREATE TABLE atom_active_day (
  atom_id uuid NOT NULL REFERENCES atom(id) ON DELETE CASCADE,
  day     date NOT NULL,
  seconds integer NOT NULL DEFAULT 0 CHECK (seconds >= 0),
  PRIMARY KEY (atom_id, day)
);
CREATE INDEX atom_active_day_day_idx ON atom_active_day (day);

-- +goose Down
DROP TABLE atom_active_day;
