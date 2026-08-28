-- +goose Up
-- 报告，以及它需要的那个「她到底待了多久」。
--
-- active_seconds 放在 atom 上而不是 reading/writing 上，和 last_activity_at 同一个
-- 理由：每一种原子都有「专注了多久」这件事，不是阅读的专属字段。默认 0 表示
-- 「这一条早于心跳存在」——报告端据此回退到时间戳估算，而不是显示 0 分钟。
ALTER TABLE atom ADD COLUMN active_seconds integer NOT NULL DEFAULT 0;

-- 一篇原子一份报告。share_token 为 NULL = 没有分享；撤回就是把它设回 NULL，
-- 于是「撤回」是真的：公开路由按 token 找行，下一次请求就什么也找不到了。
CREATE TABLE atom_report (
  atom_id     uuid PRIMARY KEY REFERENCES atom(id) ON DELETE CASCADE,
  kind        text NOT NULL CHECK (kind IN ('reading','writing')),
  report      jsonb NOT NULL,
  share_token text UNIQUE,
  shared_at   timestamptz,
  created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX atom_report_share_idx ON atom_report (share_token) WHERE share_token IS NOT NULL;

-- +goose Down
DROP TABLE atom_report;
ALTER TABLE atom DROP COLUMN active_seconds;
