-- name: AppendMove :one
INSERT INTO moves (game_id, ply_number, move_san, move_uci, fen_after)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListMoves :many
SELECT * FROM moves WHERE game_id = $1 ORDER BY ply_number ASC;
