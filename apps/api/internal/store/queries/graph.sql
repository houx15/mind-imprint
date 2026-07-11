-- name: InsertGraphNode :one
INSERT INTO graph_node (project_id, type, body, author, span_ref)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListGraphNodesByProject :many
SELECT * FROM graph_node
WHERE project_id = $1
ORDER BY created_at, id;

-- name: InsertGraphEdge :one
INSERT INTO graph_edge (project_id, type, from_kind, from_id, to_kind, to_id)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListGraphEdgesByProject :many
SELECT * FROM graph_edge
WHERE project_id = $1
ORDER BY created_at, id;
