-- 审阅印记交出来的东西（阶段二），以及项目结束时的复盘（阶段六）。

-- ── 阶段二：划出来的句子 ───────────────────────────────────────────────

-- name: CreatePblReviewMark :one
INSERT INTO pbl_review_mark (artifact_id, part, part_note, quote, question, ordinal)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListPblReviewMarks :many
SELECT * FROM pbl_review_mark WHERE artifact_id = $1 ORDER BY ordinal, created_at;

-- name: GetPblReviewMark :one
-- 归属要一路查到 atom：成果挂在项目上，项目挂在人身上。
SELECT m.*, at.user_id, at.id AS atom_id
FROM pbl_review_mark m
JOIN pbl_artifact ar ON ar.id = m.artifact_id
JOIN atom at ON at.id = ar.atom_id
WHERE m.id = $1;

-- name: AnswerPblReviewMark :one
UPDATE pbl_review_mark SET answer = $2 WHERE id = $1 RETURNING *;

-- name: LinkPblReviewMarkSession :one
-- 她点开一句话，就地开一条会话线聊它。
UPDATE pbl_review_mark SET session_id = $2 WHERE id = $1 RETURNING *;

-- ── 阶段二：每种材料该看的几个方面 ─────────────────────────────────────

-- name: CreatePblReviewDimension :one
INSERT INTO pbl_review_dimension (artifact_id, prompt, why, ordinal)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListPblReviewDimensions :many
SELECT * FROM pbl_review_dimension WHERE artifact_id = $1 ORDER BY ordinal, created_at;

-- name: GetPblReviewDimension :one
SELECT d.*, at.user_id, at.id AS atom_id
FROM pbl_review_dimension d
JOIN pbl_artifact ar ON ar.id = d.artifact_id
JOIN atom at ON at.id = ar.atom_id
WHERE d.id = $1;

-- name: AnswerPblReviewDimension :one
UPDATE pbl_review_dimension SET answer = $2 WHERE id = $1 RETURNING *;

-- ── 阶段六：复盘 ───────────────────────────────────────────────────────

-- name: CreatePblReviewPrompt :one
INSERT INTO pbl_review (atom_id, prompt, anchor_kind, anchor_ref, ordinal)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListPblReviewPrompts :many
SELECT * FROM pbl_review WHERE atom_id = $1 ORDER BY ordinal, created_at;

-- name: GetPblReviewPrompt :one
SELECT p.*, a.user_id
FROM pbl_review p JOIN atom a ON a.id = p.atom_id
WHERE p.id = $1;

-- name: AnswerPblReviewPrompt :one
UPDATE pbl_review SET answer = $2 WHERE id = $1 RETURNING *;
