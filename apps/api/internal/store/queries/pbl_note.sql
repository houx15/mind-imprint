-- 便签板。观察、引语、假设、问题、点子共用一张板（阶段一）。

-- name: CreatePblNote :one
INSERT INTO pbl_note (atom_id, kind, body, author, cluster, x, y, image_key)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: ListPblNotes :many
SELECT * FROM pbl_note
WHERE atom_id = $1 AND archived = false
ORDER BY created_at;

-- name: GetPblNote :one
SELECT n.*, a.user_id
FROM pbl_note n JOIN atom a ON a.id = n.atom_id
WHERE n.id = $1;

-- name: UpdatePblNote :one
-- 她改了印记写的便签，edited 就永久为 true——纠正是最强的过程信号之一，
-- 不该因为她后来又改回去而消失。
UPDATE pbl_note
SET body = $2, kind = $3, cluster = $4,
    edited = (edited OR author = 'yinji')
WHERE id = $1
RETURNING *;

-- name: SetPblNoteCluster :one
-- 🚨 只改归属，不碰 edited。把印记写的便签归进一堆、或者挪个位置，都是"整理"；
-- 只有改掉它的字才是"纠正"。两件事在过程记录里的分量完全不同，混起来会让
-- 每一次整理都看着像一次纠正。
UPDATE pbl_note SET cluster = $2 WHERE id = $1 RETURNING *;

-- name: MovePblNote :one
-- 🚨 dragged 只涨不跌（dragged OR $4）：代码给她排座位（boardSpot）、切换坐标
-- 视图时的单位换算，走的都是同一条 UPDATE，但那两次不是她的判断。只有真的用
-- 手拖过的那一次传 true。见 migration 0128。
UPDATE pbl_note SET x = $2, y = $3, dragged = (dragged OR $4)
WHERE id = $1 RETURNING *;

-- name: ClearPblIdeaPicks :exec
-- 「挑一个先试」是单选：挑新的之前先把旧的松开。
UPDATE pbl_note SET picked_at = NULL, pick_why = ''
WHERE atom_id = $1 AND kind = 'idea';

-- name: PickPblIdea :one
UPDATE pbl_note SET picked_at = now(), pick_why = $2
WHERE id = $1 RETURNING *;

-- name: ArchivePblNote :one
UPDATE pbl_note SET archived = true WHERE id = $1 RETURNING *;

-- name: CountPblNotesByKind :many
SELECT kind, count(*) AS n
FROM pbl_note
WHERE atom_id = $1 AND archived = false
GROUP BY kind;

-- name: PlacePblNote :one
-- 把一条便签放进结构里的某一块；node 给 NULL 就是从结构里拿回来。
-- 见 migration 0125：放不进去的那几条，就是这个结构没盖到的地方。
UPDATE pbl_note SET tree_node_id = $2 WHERE id = $1 RETURNING *;

-- name: SetPblNoteReframeSlot :one
-- 她把这张纸摆进了问题陈述的哪一格（谁 / 需要什么 / 为什么），空 = 拿回证据堆。
-- 🚨 不碰 cluster，也不碰 edited：摆格子是一句判断，不是一次改写。见 migration 0129。
UPDATE pbl_note SET reframe_slot = $2 WHERE id = $1 RETURNING *;
