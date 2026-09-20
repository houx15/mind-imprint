-- +goose Up
-- 一条意见现在先说「这一段算什么」：pass ／ polish ／ revise。
--
-- 同事 2026-09-20：
--
--	「当前反馈……将可选优化判为必改。调整为识别实际问题、分级反馈、
--	  给出明确下一步，让学生完成写作。」
--
-- 在这之前产品里没有「可优化」这个档位，于是任何一条意见读起来都像「你得改」。
-- 取值的闭表在 apps/api/internal/api/writing_verdict.go；**不写 CHECK 约束**，
-- 同 course.category / writing_outline.kind 的理由：词表在 Go 和 TS 两处，
-- 加 CHECK 会让加一个取值变成一次迁移。
ALTER TABLE writing_comment ADD COLUMN verdict text NOT NULL DEFAULT '';

-- 老行留空串：前端据此**不渲染**那一行状态标签。
-- 不回填成 'revise' 或 'polish' —— 那是替当初那条意见做一个它没做过的判断，
-- 而她会读到一个凭空出现的等级。空 = 那一版还没有分级，这是实话。

-- +goose Down
ALTER TABLE writing_comment DROP COLUMN verdict;
