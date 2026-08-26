-- +goose Up
-- edition 属于学校，不属于账号：一所学校买的是轻量版还是现有版本，校内账号在
-- 注册（凭 join code 进班级 → 班级属于学校）那一刻随之确定。组织不变式不变。
-- 默认 'pro'，存量学校行为完全不变。
ALTER TABLE schools ADD COLUMN edition text NOT NULL DEFAULT 'pro'
  CHECK (edition IN ('pro','lite'));

-- +goose Down
ALTER TABLE schools DROP COLUMN edition;
