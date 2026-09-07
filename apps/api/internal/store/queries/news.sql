-- news.sql —— 今日新闻星图。
--
-- 生成是**惰性**的，和兴趣采集同一个理由：这个 repo 里没有 river worker，
-- 而一个每天定时跑的任务需要一个跑它的东西。第一个打开星图的学生触发生成，
-- 拿 advisory lock，其余人直接读。见 internal/api/explore.go。

-- name: GetNewsDay :one
SELECT * FROM news_day WHERE day = $1;

-- 盖章。**在生成之前调**：语义是「今天试过一次」，不是「今天出过星图」。
-- 所有源都挂掉的那天，如果按产出判断，每个打开星图的学生都会再触发一次全量
-- 抓取 + 一次旗舰调用。
--
-- 计次而不是只盖一次章（migration 0135）：一次失败不该锁死一整天。允不允许再试
-- 由 explore.go 的 starmapRetryable 判定，这里只负责把次数和时间记准。
-- name: MarkNewsDayAttempted :exec
INSERT INTO news_day (day) VALUES ($1)
ON CONFLICT (day) DO UPDATE
  SET attempts = news_day.attempts + 1, attempted_at = now();

-- name: FinishNewsDay :exec
UPDATE news_day SET planet_count = $2, note = $3 WHERE day = $1;

-- name: InsertNewsPlanet :one
INSERT INTO news_planet (
  day, rank, title_zh, title_en, summary, hook, url, source_name,
  field, discipline_id, interest_id, published_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
ON CONFLICT (day, rank) DO NOTHING
RETURNING *;

-- name: ListNewsPlanets :many
SELECT * FROM news_planet WHERE day = $1 ORDER BY rank;

-- 她最近几天见过的星图。星图是**每天一屏**，往回翻是看昨天，不是一个信息流，
-- 所以调用方给一个很小的 limit。
-- name: ListRecentNewsDays :many
SELECT DISTINCT day FROM news_planet ORDER BY day DESC LIMIT $1;

-- name: GetNewsPlanet :one
SELECT * FROM news_planet WHERE id = $1;

-- name: ListSavedPlanets :many
SELECT planet_id, reading_id FROM news_saved WHERE user_id = $1;

-- 收一颗星球是一次动作，不是开关；重复收什么也不做。**第一次收下的那篇阅读
-- 就是这颗星球的那篇**（迁移 0138）—— 不覆盖，否则她点两次「稍后读」会在阅读
-- 室里攒下两篇同名的、其中一篇再也找不到。
-- name: SavePlanet :exec
INSERT INTO news_saved (user_id, planet_id, reading_id) VALUES ($1, $2, $3)
ON CONFLICT (user_id, planet_id) DO NOTHING;

-- name: GetPlanetSaveReading :one
SELECT reading_id FROM news_saved WHERE user_id = $1 AND planet_id = $2;

-- name: HasSavedPlanet :one
SELECT EXISTS (SELECT 1 FROM news_saved WHERE user_id = $1 AND planet_id = $2);
