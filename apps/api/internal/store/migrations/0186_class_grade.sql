-- 0186_class_grade.sql —— 这个班是几年级。
--
-- 产品负责人 2026-09-22：「during a class creating we can add 学段。」
-- 不按每篇作文推断，也不问学生 —— 建班的人知道这件事，问一次就够了。
--
-- 为什么要到年级这一层，而不是初中／高中两档：`初中语文作文批改` 的标准
-- 是按年级给的（初一 500–600 字「叙事完整」、初二 550–650「描写生动」、
-- 初三 600–700「立意深刻」），两档表达不了这三行。一个班本来就是一个年级。
--
-- 🚨 **不写 CHECK 约束**，和 writing.origin（0146）、writing.stage（0100）
-- 同一个理由：闭表的执行点在 Go（internal/api/class_grade.go），而一个还
-- 容得下将来某个值的数据库不花钱；立了 CHECK，将来加一个年级就要重建约束，
-- 那是一次会锁表的迁移。
--
-- 🚨 **和 writing.stage 不是一回事**。那一列是写作流程走到哪儿
-- （ideate/outline/…），这一列是她读几年级。名字不同是故意的。
--
-- additive：老班全部回填空串 = 不知道年级，行为和这次迁移之前一模一样。

-- +goose Up
ALTER TABLE classes
    ADD COLUMN grade text NOT NULL DEFAULT '';

COMMENT ON COLUMN classes.grade IS
  '这个班几年级：junior1..3 / senior1..3；空串 = 没填。闭表在 internal/api/class_grade.go，不在 DB 上设 CHECK。';

-- +goose Down
ALTER TABLE classes
    DROP COLUMN grade;
