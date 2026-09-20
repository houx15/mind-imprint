/* ── 觉醒协议 ────────────────────────────────────────────────────────────── */

-- 开一趟。**先落库再做题**：一次中途退出也留下一行（finished_at 为空），
-- 而「她走到哪一屏就走开了」本身就是数据（铁律④）。
--
-- attempt_no 在插入时由子查询算出来并写死，不在读取时现数 —— 现数会让两个
-- 并发的重复 POST 都读到同一个「已完成 1 次」，于是都写成「第 2 次」。
-- 部分唯一索引 awakening_run_open_idx 另外保证同一个人最多只有一趟没走完。
-- name: StartAwakeningRun :one
INSERT INTO awakening_run (user_id, attempt_no)
VALUES (
  $1,
  1 + (SELECT count(*) FROM awakening_run
       WHERE user_id = $1 AND finished_at IS NOT NULL)
)
RETURNING *;

-- 她还没走完的那一趟。刷新一次、第二天回来，接着这一行走。
-- name: OpenAwakeningRun :one
SELECT * FROM awakening_run
WHERE user_id = $1 AND finished_at IS NULL;

-- 带 user_id 一起匹配，所以一个人读不到、也改不了别人的那一行。
-- name: GetAwakeningRun :one
SELECT * FROM awakening_run
WHERE id = $1 AND user_id = $2;

-- 存一次进度。
--
-- 客户端每次换屏把**整份状态**发上来，服务端整行覆盖。这样设计的理由是它幂等：
-- 一次重复提交、一次网络重试，结果和只提交一次完全一样。代价是客户端必须始终
-- 带全字段 —— 少带一个就等于把它清空，所以 DTO 那一侧全部字段非指针。
-- name: SaveAwakeningProgress :one
UPDATE awakening_run SET
  stage             = $3,
  route             = $4,
  navigator         = $5,
  energy_profile    = $6,
  talent            = $7,
  lens_choice       = $8,
  challenge_choice  = $9,
  archive_attempts  = $10,
  observer_question = $11,
  updated_at        = now()
WHERE id = $1 AND user_id = $2 AND finished_at IS NULL
RETURNING *;

-- 走完了。只盖一次章：WHERE finished_at IS NULL 让重复提交的第二次返回零行，
-- 调用方据此知道报告已经在生成或已经生成过了。
-- name: FinishAwakeningRun :one
UPDATE awakening_run SET finished_at = now(), updated_at = now()
WHERE id = $1 AND user_id = $2 AND finished_at IS NULL
RETURNING *;

-- 她走完过几次。**只数走完的** —— 一次中途退出不算「做过了」，所以树上那条
-- 邀请还会再出现。一个在第三屏关掉页面的学生不该从此再也见不到入口。
-- name: CountFinishedAwakeningRuns :one
SELECT count(*) FROM awakening_run
WHERE user_id = $1 AND finished_at IS NOT NULL;

-- 她最近走完的那一趟。树上那条入口读它，报告做「和上次比」也读它。
-- name: LatestFinishedAwakeningRun :one
SELECT * FROM awakening_run
WHERE user_id = $1 AND finished_at IS NOT NULL
ORDER BY finished_at DESC
LIMIT 1;

-- 上一趟（不含这一趟）。报告第六块「和上次比」用它取上一份 payload。
-- name: PreviousFinishedAwakeningRun :one
SELECT * FROM awakening_run
WHERE user_id = $1 AND finished_at IS NOT NULL AND id <> $2
ORDER BY finished_at DESC
LIMIT 1;

/* ── 终端的每一轮 ────────────────────────────────────────────────────────── */

-- 记一轮。seq 由调用方按已有轮数算出来；UNIQUE (run_id, seq) 会挡掉一次
-- 重复提交，所以同一句话不会被记两遍，也不会被当成两份语料。
-- name: AppendAwakeningTurn :one
INSERT INTO awakening_turn (run_id, seq, node_index, student_text, reply)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- 把这一趟的轮次清空 —— 「新的探索」。
--
-- 她保留着一条没做完的线索，回来却想换一个话题从头问。清空的是**轮次**，
-- 不是这一趟：助手和能量结果留在 awakening_run 上，她不必再选一遍。
--
-- 🚨 这一条删的是她自己写下的字，没有回收站。所以调用方先查一次归属
-- （GetAwakeningRun 带 user_id），界面上也必须先问一次。
-- name: ClearAwakeningTurns :exec
DELETE FROM awakening_turn WHERE run_id = $1;

-- 这一趟说过的全部。它同时是**语料** —— 树上每个词的 evidence 都要能在
-- student_text 里逐字查到。
-- name: ListAwakeningTurns :many
SELECT * FROM awakening_turn
WHERE run_id = $1
ORDER BY seq ASC;

/* ── 报告 ────────────────────────────────────────────────────────────────── */

-- 一趟一份。ON CONFLICT DO NOTHING 之后返回零行，表示别人已经生成过了 ——
-- 调用方这时去读那一份，不再花第二次调用。
-- name: InsertAwakeningReport :one
INSERT INTO awakening_report (run_id, user_id, payload)
VALUES ($1, $2, $3)
ON CONFLICT (run_id) DO NOTHING
RETURNING *;

-- name: GetAwakeningReportByRun :one
SELECT * FROM awakening_report
WHERE run_id = $1 AND user_id = $2;

-- name: GetAwakeningReportForUser :one
SELECT * FROM awakening_report
WHERE id = $1 AND user_id = $2;

-- name: LatestAwakeningReport :one
SELECT r.* FROM awakening_report r
WHERE r.user_id = $1
ORDER BY r.created_at DESC
LIMIT 1;

-- 公开 / 撤回。置 NULL 之后下一个请求就查不到 —— 没有缓存，没有宽限期。
-- name: SetAwakeningReportShare :one
UPDATE awakening_report SET share_token = $3
WHERE id = $1 AND user_id = $2
RETURNING *;

-- 公开读。**不带 user_id**：这条路是给没有登录的人的。返回的行里只有
-- payload 会被投影出去，绝不多带一个字段（见 atom_report_share.go 的 R1）。
-- name: GetAwakeningReportByShareToken :one
SELECT * FROM awakening_report
WHERE share_token = $1;
