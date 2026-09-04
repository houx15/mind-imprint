-- +goose Up
-- 星图生成失败之后还能再试。
--
-- 0120 把章盖在生成之前，理由是对的：零颗星和「今天从没试过」在 news_planet 里
-- 长得一模一样，按产出判断会让每个打开星图的学生都触发一次全量抓取加一次模型
-- 调用。但那一版的代价是**一次失败就锁死一整天**——上游一个 503、模型回了一段
-- 不是 JSON 的话，这一天就再也不会生成，界面上那个「重试」按钮按下去什么都不
-- 会发生。2026-09-04 的模拟学生走查里，六次启动撞上两次。
--
-- 所以章还是盖在生成之前，但改成计次：失败的那天允许再试，次数和间隔由
-- explore.go 的 starmapRetryable 判定。成功那天 planet_count > 0，一次都不会再试。
ALTER TABLE news_day ADD COLUMN attempts int NOT NULL DEFAULT 1;

-- 已经存在的行都是「至少试过一次」，默认值就是对的。

-- +goose Down
ALTER TABLE news_day DROP COLUMN attempts;
