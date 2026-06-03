-- name: CreateChirps :one
INSERT INTO chirps (id, created_at, updated_at, body, user_id)
VALUES (gen_random_uuid(), Now(), Now(), $1, $2)
RETURNING *;

-- name: GetChirps :many
SELECT *
FROM chirps
ORDER BY created_at ASC;

-- name: GetByID :one
SELECT *
FROM chirps
WHERE ID = $1;

-- name: GetByID2 :many
SELECT *
FROM chirps
WHERE user_id = $1;

