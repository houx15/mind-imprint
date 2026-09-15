-- Lite 写作的提交版本（0153）。只插入、只读，没有 UPDATE 或 DELETE。

-- name: NextWritingVersionNumber :one
-- 与 CreateWritingVersion 放在同一个事务里，并先锁住 writing 行（GetWritingForUpdate）。
SELECT (COALESCE(max(number), 0) + 1)::int FROM writing_version WHERE atom_id = $1;

-- name: CreateWritingVersion :one
INSERT INTO writing_version (atom_id, number, title, body, word_count)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListWritingVersions :many
-- 版本列表不带正文，新的在前。
SELECT id, atom_id, number, title, word_count, submitted_at
FROM writing_version
WHERE atom_id = $1
ORDER BY number DESC;

-- name: GetWritingVersion :one
SELECT * FROM writing_version WHERE atom_id = $1 AND number = $2;

-- name: GetLatestWritingVersion :one
SELECT * FROM writing_version WHERE atom_id = $1 ORDER BY number DESC LIMIT 1;

-- name: CountWritingVersions :one
SELECT count(*)::int FROM writing_version WHERE atom_id = $1;
