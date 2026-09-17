-- +goose Up
-- 阅读报告上「段落工具」那一节是否跟着分享链接一起公开。默认 false。
--
-- 那一节里有她在想一想 / 仿写 底下**自己写的**那几段。产品负责人 2026-09-17
-- 选的是「像公开对话那样单独勾选」，所以这一列照着 0174 的 include_transcript
-- 办：默认关；撤销分享时跟着归 false（SetAtomReportShare 的 CASE 分支），一条
-- 撤掉又重开的链接不继承上一次的公开范围。
ALTER TABLE atom_report ADD COLUMN include_toolkit boolean NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE atom_report DROP COLUMN include_toolkit;
