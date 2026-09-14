-- name: AddAtomActiveDay :exec
INSERT INTO atom_active_day (atom_id, day, seconds)
VALUES ($1, $2, $3)
ON CONFLICT (atom_id, day) DO UPDATE SET seconds = atom_active_day.seconds + EXCLUDED.seconds;
