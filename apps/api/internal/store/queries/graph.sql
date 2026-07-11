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

-- name: GetGateStateNode :one
SELECT * FROM graph_node
WHERE project_id = $1 AND type = 'gate_state' AND body->>'contract' = $2::text
LIMIT 1;

-- name: ListGateStateNodes :many
SELECT * FROM graph_node
WHERE project_id = $1 AND type = 'gate_state'
ORDER BY created_at, id;

-- name: GetPlanNode :one
SELECT * FROM graph_node
WHERE project_id = $1 AND type = 'plan'
ORDER BY created_at, id
LIMIT 1;

-- name: UpdateGraphNodeBody :one
UPDATE graph_node SET body = $2 WHERE id = $1 RETURNING *;
