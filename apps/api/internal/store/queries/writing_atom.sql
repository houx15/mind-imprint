-- Writing's own atom-kind tables (0099): writing / writing_outline /
-- writing_snippet / writing_draft. Named writing_atom.sql, not writing.sql —
-- queries/writing.sql already exists for the unrelated pro-product edit
-- buffer + draft snapshot feature (project_id-scoped, Slice 8/S5). The two
-- "writing"s share a word, not a table or a domain.

-- name: CreateWriting :one
INSERT INTO writing (atom_id, title, lang) VALUES ($1, $2, $3) RETURNING *;

-- name: GetWriting :one
SELECT * FROM writing WHERE atom_id = $1;

-- name: ListWritingsByUser :many
-- The list the 写作 tab shows. Joins atom for ownership + creation order,
-- same shape as ListReadingsByUser — atom_created_at rides along so the API
-- layer can fill writingDTO.createdAt without an N+1 GetAtom per row (the
-- writing table itself has no created_at column; only atom does).
--
-- last_activity_at rides along the same way (0098's atom.last_activity_at,
-- bumped by loadOwnedAtom on every non-GET against an open atom of EITHER
-- kind) so writingDTO can carry a real "when did she last touch this" the
-- same way readingDTO does — writing.updated_at only moves on rename/stage
-- changes/target-words, never on a turn or a snippet edit.
SELECT w.*, a.created_at AS atom_created_at, a.last_activity_at
FROM writing w
JOIN atom a ON a.id = w.atom_id
WHERE a.user_id = $1 AND a.kind = 'writing'
ORDER BY a.created_at DESC;

-- name: RenameWriting :exec
UPDATE writing SET title = $2, updated_at = now() WHERE atom_id = $1;

-- name: SetWritingStage :one
-- No gate here on purpose (§6.2.3): stages are a map shown to the student,
-- not a checkpoint the server enforces, so any legal value is accepted in any
-- order. The CHECK constraint is the only guard.
UPDATE writing SET stage = $2, updated_at = now() WHERE atom_id = $1 RETURNING *;

-- name: SetWritingTargetWords :exec
UPDATE writing SET target_words = $2, updated_at = now() WHERE atom_id = $1;

-- name: SetWritingAssignedPrompt :exec
-- 老师布置的题目存在这里，不作为她的第一条消息。建写作的同一个事务里写。
UPDATE writing SET assigned_prompt = $2 WHERE atom_id = $1;

-- name: SetWritingFinished :exec
-- Guarded on status, like SetReadingFinished: a second /finish is a genuine
-- no-op so finished_at never drifts after the fact (铁律④ makes it evidence).
-- Deliberately does NOT force stage to 'finished' — a student who skipped
-- straight to a finished draft without ever touching 提纲/片段 keeps that
-- fact recorded in stage for the report, rather than having it silently
-- overwritten at finish time.
UPDATE writing SET status = 'finished', finished_at = now(), updated_at = now()
WHERE atom_id = $1 AND status <> 'finished';

-- name: GetWritingForUpdate :one
-- 完成、放弃修改时先锁住这一行，版本号才不会重复。
SELECT * FROM writing WHERE atom_id = $1 FOR UPDATE;

-- name: SetWritingRevising :one
-- 已在修改中时保留原来的时间。
UPDATE writing SET revising_at = COALESCE(revising_at, now()), updated_at = now()
WHERE atom_id = $1
RETURNING *;

-- name: ClearWritingRevising :exec
UPDATE writing SET revising_at = NULL, updated_at = now() WHERE atom_id = $1;

-- name: ReplaceWritingOutline :many
-- Full replace, not a diff: the whole outline is written as one shape each
-- PUT (学生可改 the derived outline wholesale), so delete-then-insert keeps
-- position contiguous and never leaves a stale tail row behind. The DELETE
-- and INSERT run as ONE statement (a data-modifying CTE), so a concurrent
-- reader never observes a momentarily-empty outline between the two.
WITH deleted AS (
  DELETE FROM writing_outline WHERE atom_id = sqlc.arg(atom_id)
)
-- roles 与 texts 平行传入（0100）：role 是骨架给的通用块名，text 是她自己
-- 写的那句话。一次 PUT 同时重写两列，role 才不会在她编辑正文时被抹掉。
-- source 与它们平行传入（0158）：一条她找来的材料要带着出处走，否则 印记
-- 没法查它说的对不对 —— 而「查一份材料」正是这一列存在的全部理由。
INSERT INTO writing_outline (atom_id, text, role, depth, position, source)
SELECT sqlc.arg(atom_id),
       unnest(sqlc.arg(texts)::text[]),
       unnest(sqlc.arg(roles)::text[]),
       unnest(sqlc.arg(depths)::int[]),
       unnest(sqlc.arg(positions)::int[]),
       unnest(sqlc.arg(sources)::text[])
RETURNING *;

-- name: ListWritingOutline :many
SELECT * FROM writing_outline WHERE atom_id = $1 ORDER BY position;

-- name: UpsertWritingSnippet :one
-- Keyed by (atom_id, position) — writing the same slot again edits it in
-- place, it never duplicates a row (writing_snippet_atom_position_idx, 0099).
INSERT INTO writing_snippet (atom_id, outline_id, position, text)
VALUES ($1, $2, $3, $4)
ON CONFLICT (atom_id, position) DO UPDATE
  SET outline_id = EXCLUDED.outline_id, text = EXCLUDED.text, updated_at = now()
RETURNING *;

-- name: ListWritingSnippets :many
SELECT * FROM writing_snippet WHERE atom_id = $1 ORDER BY position;

-- name: UpsertWritingDraft :one
INSERT INTO writing_draft (atom_id, body)
VALUES ($1, $2)
ON CONFLICT (atom_id) DO UPDATE SET body = EXCLUDED.body, updated_at = now()
RETURNING *;

-- name: GetWritingDraft :one
SELECT * FROM writing_draft WHERE atom_id = $1;

-- name: MarkWritingBrought :exec
-- 她带进来的一篇成稿：来源记成 brought，而且直接落在 draft ——
-- 结构和段落两步对这一篇根本没有发生过，让它假装经过那两步是不诚实的。
-- origin 为什么要存下来，见 0146_writing_origin.sql。
UPDATE writing SET origin = 'brought', stage = 'draft', updated_at = now()
WHERE atom_id = $1;

-- name: RelinkWritingSnippetOutline :exec
-- Re-attach one snippet to an outline row after ReplaceWritingOutline minted
-- fresh ids. Called only with a NEW outline id whose TEXT matches the heading
-- the snippet was written under, so this restores a real link rather than
-- guessing one by position (position guessing silently swaps headings the
-- moment she reorders her outline, which is worse than showing none).
UPDATE writing_snippet SET outline_id = $2, updated_at = now()
WHERE atom_id = $1 AND id = $3;

-- name: SetWritingSetup :one
-- 进入房间的「设定」弹窗：语言 + 目标篇幅一次落库，并盖上 setup_at 时间戳，
-- 这样弹窗只在第一次进入时出现。三件事必须在同一条语句里完成——分成三次
-- 写，中途失败就会留下「定了语言但还会再被弹窗拦一次」的半截状态。
-- target_words 允许为 NULL（她可以不定篇幅，铁律②：篇幅从来不是前置条件）。
UPDATE writing
SET lang = $2, target_words = $3, setup_at = now(), updated_at = now()
WHERE atom_id = $1
RETURNING *;

-- name: SetWritingStructure :one
-- 记下她选中的骨架。与 ReplaceWritingOutline 由调用方放进同一个事务：骨架
-- 与它铺出来的空块必须同生同死，否则 structure_key 会指向一副并不存在的提纲。
UPDATE writing SET structure_key = $2, updated_at = now()
WHERE atom_id = $1
RETURNING *;

-- name: ShiftWritingOutlinePositions :exec
-- 给插入腾位：把 position >= $2 的行整体后移一位。与 InsertWritingOutlineNode
-- 由调用方放进同一个事务——中间断开会留下两行同 position 的提纲。
UPDATE writing_outline SET position = position + 1
WHERE atom_id = $1 AND position >= $2;

-- name: InsertWritingOutlineNode :one
-- 往思维导图里加一个节点。**只加，不改不删**——这是规划对话的硬保证：
-- 印记 能往图上加她刚说过的东西，但永远动不了、也删不掉她已经写下的节点
-- （ReplaceWritingOutline 那条全量替换的路只留给学生自己的编辑）。
--
-- 与 ReplaceWritingOutline 的关键差别是**保住 id**。规划是一轮一轮长出来的，
-- 每一轮都全量重写会重新铸 id，把父子引用和 writing_snippet.outline_id 一起
-- 打断；这里逐个插入，既有的行一个都不动。
INSERT INTO writing_outline (atom_id, text, role, depth, position, source)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: SetWritingOutlineGuide :exec
UPDATE writing_outline SET guide = $2 WHERE id = $1;

-- name: CreateWritingComment :one
INSERT INTO writing_comment (atom_id, snippet_id, scope, summary, points, source_text)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListWritingComments :many
SELECT * FROM writing_comment WHERE atom_id = $1 ORDER BY created_at DESC;

-- name: GetLatestWritingDraftComment :one
SELECT * FROM writing_comment
WHERE atom_id = $1 AND scope = 'draft'
ORDER BY created_at DESC
LIMIT 1;
