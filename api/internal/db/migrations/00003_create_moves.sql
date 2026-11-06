-- +goose Up
CREATE TABLE moves (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    game_id UUID NOT NULL REFERENCES games(id),
    ply_number INT NOT NULL,
    move_san TEXT NOT NULL,
    move_uci TEXT NOT NULL,
    fen_after TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_moves_game_ply ON moves (game_id, ply_number);

-- +goose Down
DROP TABLE moves;
