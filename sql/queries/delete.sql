-- name: DeleteChripByID :exec
DELETE FROM chirps
WHERE id = $1;
