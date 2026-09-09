-- name: CreateReading :one
INSERT INTO reading (atom_id, title, lang) VALUES ($1, $2, $3) RETURNING *;

-- name: GetReading :one
SELECT * FROM reading WHERE atom_id = $1;

-- name: ListReadingsByUser :many
-- The list the 阅读 tab shows. Joins atom for ownership + creation order.
--
-- The LEFT JOIN on reading_source is what keeps the landing's first paint to
-- ONE query: hasSource used to be answered by a GetReadingSource per reading
-- (a textbook N+1 — and it pulled each article's whole BODY across the wire
-- only to test the row's existence). Existence is all the DTO needs, so it is
-- computed here.
SELECT r.*,
       a.created_at AS atom_created_at,
       a.last_activity_at,
       (s.atom_id IS NOT NULL)::bool AS has_source
FROM reading r
JOIN atom a ON a.id = r.atom_id
LEFT JOIN reading_source s ON s.atom_id = r.atom_id
WHERE a.user_id = $1 AND a.kind = 'reading'
ORDER BY a.created_at DESC;

-- name: RenameReading :exec
UPDATE reading SET title = $2, updated_at = now() WHERE atom_id = $1;

-- name: SetReadingFinished :exec
-- Guarded on status, so a SECOND POST /finish is a genuine no-op rather than a
-- re-stamp. finished_at is a fact about when she finished; an idempotent
-- endpoint that quietly moves it lets a completion time drift after the fact,
-- and 铁律④ makes that timestamp evidence like everything else on the atom.
UPDATE reading SET status = 'finished', finished_at = now(), updated_at = now()
WHERE atom_id = $1 AND status <> 'finished';

-- name: UpsertReadingSource :one
INSERT INTO reading_source (atom_id, title, body, source_url)
VALUES ($1, $2, $3, $4)
ON CONFLICT (atom_id) DO UPDATE
  SET title = EXCLUDED.title, body = EXCLUDED.body,
      source_url = EXCLUDED.source_url, ingested_at = now()
RETURNING *;

-- name: GetReadingSource :one
SELECT * FROM reading_source WHERE atom_id = $1;

-- name: UpsertReadingBrief :one
INSERT INTO reading_brief (atom_id, phase_tag, reading_reason, reading_focus)
VALUES ($1, $2, $3, $4)
ON CONFLICT (atom_id) DO UPDATE
  SET phase_tag = EXCLUDED.phase_tag,
      reading_reason = EXCLUDED.reading_reason,
      reading_focus = EXCLUDED.reading_focus,
      updated_at = now()
RETURNING *;

-- name: GetReadingBrief :one
SELECT * FROM reading_brief WHERE atom_id = $1;

-- name: UpsertReadingTakeaway :one
INSERT INTO reading_takeaway (atom_id, text)
VALUES ($1, $2)
ON CONFLICT (atom_id) DO UPDATE SET text = EXCLUDED.text, updated_at = now()
RETURNING *;

-- name: GetReadingTakeaway :one
SELECT * FROM reading_takeaway WHERE atom_id = $1;

-- name: SetReadingRoutine :one
-- 记下这篇用的是哪一套读法。与 ReplaceReadingTasks 由调用方放进同一个事务：
-- routine_key 与它排出的清单必须同生同死，否则会指向一份并不存在的清单。
UPDATE reading SET routine_key = $2 WHERE atom_id = $1 RETURNING *;

-- name: ReplaceReadingTasks :many
-- 全量替换，与 ReplaceWritingOutline 同一个理由：清单是一次排好的一个形状，
-- delete-then-insert 保证 position 连续、不留下上一份的尾巴。DELETE 与
-- INSERT 是**一条语句**（data-modifying CTE），所以并发读者永远看不到一份
-- 空清单。
WITH deleted AS (
  DELETE FROM reading_task WHERE atom_id = sqlc.arg(atom_id)
)
INSERT INTO reading_task (atom_id, position, kind, label, detail, block_id)
SELECT sqlc.arg(atom_id),
       unnest(sqlc.arg(positions)::int[]),
       unnest(sqlc.arg(kinds)::text[]),
       unnest(sqlc.arg(labels)::text[]),
       unnest(sqlc.arg(details)::text[]),
       unnest(sqlc.arg(block_ids)::text[])
RETURNING *;

-- name: ListReadingTasks :many
SELECT * FROM reading_task WHERE atom_id = $1 ORDER BY position;

-- name: SetReadingTaskStatus :one
-- 只改状态，永不改文字：清单排定之后，学生能做的是完成或跳过它，不是重写它。
-- completed_at 在 'done' 时盖章，其余状态清空——一步被改回 pending 却还留着
-- 完成时间，会让报告读出一个从没发生过的完成。
UPDATE reading_task
SET status = $3,
    completed_at = CASE WHEN $3 = 'done' THEN now() ELSE NULL END
WHERE atom_id = $1 AND id = $2
RETURNING *;

-- name: GetReadingBlockNote :one
SELECT * FROM reading_block_note WHERE atom_id = $1 AND block_id = $2 AND tool = $3;

-- name: InsertReadingBlockNote :one
-- ON CONFLICT DO UPDATE 而不是 DO NOTHING：并发两次点同一个工具时，两边都要
-- 拿到一行回来，否则输的那一边会看到「成功了但没有内容」。
INSERT INTO reading_block_note (atom_id, block_id, tool, body)
VALUES ($1, $2, $3, $4)
ON CONFLICT (atom_id, block_id, tool) DO UPDATE SET body = EXCLUDED.body
RETURNING *;

-- name: ListReadingBlockNotes :many
SELECT * FROM reading_block_note WHERE atom_id = $1 ORDER BY created_at;

-- name: ListReadingQuestions :many
SELECT * FROM reading_question WHERE atom_id = $1 ORDER BY position;

-- name: InsertReadingQuestion :one
INSERT INTO reading_question (atom_id, position, text, anchor_quote, anchor_block)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: MarkReadingQuestionsGenerated :one
-- Records that a generation ATTEMPT happened, independent of how many
-- questions survived it. Set unconditionally (even when zero rows were
-- inserted) so a thin article that legitimately yields nothing never looks,
-- to the next open, indistinguishable from "never tried".
UPDATE reading SET questions_at = now() WHERE atom_id = $1 RETURNING *;

-- name: CreateLibraryReading :one
-- 从分级阅读库开一篇。与 CreateReading 分开写，是因为库里来的这一篇一出生
-- 就带着来源（哪一篇、哪一档）—— 补一次 UPDATE 就会出现一个短暂的、来源为
-- 空的窗口，而书架正是靠这两列判断「读过没有」。
INSERT INTO reading (atom_id, title, lang, library_slug, library_tier)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: UpsertLibraryReadingSource :one
-- 库里来的正文连同它的版式（图 + 小标题）一起落库。见迁移 0142 的头注。
INSERT INTO reading_source (atom_id, title, body, source_url, figures, headings)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (atom_id) DO UPDATE
  SET title = EXCLUDED.title, body = EXCLUDED.body,
      source_url = EXCLUDED.source_url, figures = EXCLUDED.figures,
      headings = EXCLUDED.headings, ingested_at = now()
RETURNING *;

-- name: ListLibraryReadingsByUser :many
-- 她在库里读过什么、读到哪一档、读完没有。书架用它划掉读过的，推荐用它选档。
SELECT r.library_slug, r.library_tier, r.status, r.atom_id
FROM reading r
JOIN atom a ON a.id = r.atom_id
WHERE a.user_id = $1 AND a.kind = 'reading' AND r.library_slug <> ''
ORDER BY a.created_at DESC;
