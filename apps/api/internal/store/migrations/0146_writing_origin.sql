-- 0146_writing_origin.sql —— 这一篇是在这儿写的，还是她带进来的。
--
-- 产品负责人 2026-09-11：
--
--   > we also make students available to upload a written one to seek for advice
--
-- 带一篇已经写完的进来，是一条真实的需求（写完了想要意见，而不是从零开始）。
-- 但它同时是**过程评估上的一件大事**：这一篇的结构和段落两步根本没有发生过，
-- 印记 一个字都没参与。报告如果不说这件事，就等于把她自己在别处写的东西
-- 算成了这里的过程——而「诚实介绍 AI 和人的分工」正是铁律①最后半句。
--
-- 所以这一列不是元数据洁癖，它是报告那一节能不能说真话的前提。
--
-- 🚨 **不写 CHECK 约束**，和 writing.stage 同一个理由（0100:29-30）：
-- Go 那一侧的闭表是执行点（writing_origins，writings.go），而一个还容得下
-- 将来某个值的数据库不花钱。反过来立了 CHECK，将来加一种来源就要重建约束，
-- 而重建约束是一次会锁表的迁移。
--
-- additive：老行全部回填 'here'，没有一行会因为这次迁移变得不可读。

-- +goose Up
ALTER TABLE writing
    ADD COLUMN origin text NOT NULL DEFAULT 'here';

COMMENT ON COLUMN writing.origin IS
  'here = 在这个房间里写的；brought = 她带进来的成稿。报告据此说明哪几步没有发生过。';

-- +goose Down
ALTER TABLE writing
    DROP COLUMN origin;
