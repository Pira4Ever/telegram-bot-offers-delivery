-- name: GebById :one
SELECT * FROM tabloides WHERE id = $1 LIMIT 1;

-- name: CreateTabloide :one
INSERT INTO tabloides (id, mercado) VALUES ($1, $2) RETURNING *;

-- name: DeleteOld :exec
DELETE FROM tabloides WHERE protected = FALSE AND created_at < NOW() - INTERVAL '30 days';

-- name: GetLastWeek :many
SELECT * FROM tabloides WHERE protected = FALSE AND created_at >= NOW() - INTERVAL '7 days';