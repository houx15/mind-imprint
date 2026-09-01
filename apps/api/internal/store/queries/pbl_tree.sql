-- 结构图（阶段四）。先看结构，再写内容。

-- name: CreatePblTreeNode :one
INSERT INTO pbl_tree_node (atom_id, tree, parent_id, depth, ordinal, title, body, author)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: ListPblTreeNodes :many
SELECT * FROM pbl_tree_node
WHERE atom_id = $1 AND tree = $2
ORDER BY depth, ordinal, created_at;

-- name: GetPblTreeNode :one
SELECT n.*, a.user_id
FROM pbl_tree_node n JOIN atom a ON a.id = n.atom_id
WHERE n.id = $1;

-- name: UpdatePblTreeNode :one
UPDATE pbl_tree_node
SET title = $2, body = $3, edited = (edited OR author = 'yinji')
WHERE id = $1
RETURNING *;

-- name: MovePblTreeNode :one
-- 换父节点要同时给新的 depth——depth 是存下来的，CHECK 会挡住超深的移动。
UPDATE pbl_tree_node
SET parent_id = $2, depth = $3, ordinal = $4
WHERE id = $1
RETURNING *;

-- name: DeletePblTreeNode :exec
DELETE FROM pbl_tree_node WHERE id = $1;

-- name: ListPblTreeChecks :many
SELECT * FROM pbl_tree_check WHERE atom_id = $1 AND tree = $2 ORDER BY question;

-- name: AnswerPblTreeCheck :one
INSERT INTO pbl_tree_check (atom_id, tree, question, answer)
VALUES ($1, $2, $3, $4)
ON CONFLICT (atom_id, tree, question) DO UPDATE SET answer = EXCLUDED.answer
RETURNING *;
