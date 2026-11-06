-- name: CreateGame :one
INSERT INTO games (user_id, status, player_color, difficulty, current_fen)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetGame :one
SELECT * FROM games WHERE id = $1;

-- name: ListGamesByUser :many
SELECT * FROM games WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3;

-- name: UpdateGameState :exec
UPDATE games SET current_fen = $2, status = $3, result = $4, updated_at = now() WHERE id = $1;
