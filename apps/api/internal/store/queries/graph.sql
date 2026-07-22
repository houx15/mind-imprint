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

-- name: DeleteGraphNode :exec
DELETE FROM graph_node WHERE id = $1 AND project_id = $2;

-- name: DeleteStationViewNodes :exec
-- Deletes ONLY the nodes the S0/S1/S2 station views themselves wrote, identified
-- by the body marker origin='station_view'. This scoping is load-bearing: the
-- perspective-matrix tool card also mints `perspective` nodes (agent/card_effects.go),
-- and a re-save of the S2 view must never delete them. Callers pass an explicit
-- type list; there is deliberately no "delete everything for this project" form.
DELETE FROM graph_node
WHERE project_id = $1
  AND type = ANY(@types::text[])
  AND body->>'origin' = 'station_view';
