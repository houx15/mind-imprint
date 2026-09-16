-- +goose Up
-- 对话是否跟着这条分享链接一起公开。默认 false —— 她不勾，就什么都没多出去。
--
-- 🚨 这一列推翻了 atom_report_share.go 顶上那条写了很久的裁定：「The payload is
-- the report and nothing else: no account, **no transcript**, …」。2026-09-16
-- 产品负责人明确要她**自己能选**：「and the report and published writing, can be
-- made public.」，并在追问时选了「她可以单独勾选公开」而不是「跟报告一起公开」。
--
-- 那条旧裁定仍然成立的部分一个字没动，而且这一列是照着它设计的：
--   · 默认关 —— 不选就没有；
--   · 撤销分享时跟着归 false（SetAtomReportShare 的 CASE 分支），所以一条撤掉
--     又重开的链接不会继承上一次的公开范围；
--   · 公开负载里仍然没有第三件东西：账号、正文全文、别的 id，一个都不多。
ALTER TABLE atom_report ADD COLUMN include_transcript boolean NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE atom_report DROP COLUMN include_transcript;
