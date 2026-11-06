package game

import "context"

type Repository interface {
	CreateGame(ctx context.Context, g *Game) error
	GetGame(ctx context.Context, id string) (*Game, error)
	ListGamesByUser(ctx context.Context, userID string, limit, offset int) ([]*Game, error)
	ListMoves(ctx context.Context, gameID string) ([]*Move, error)
	// AppendMoveAndUpdateState atomically records a move and the game's
	// resulting state together, so a move is never persisted without its
	// corresponding game-state update (or vice versa) if either write fails.
	AppendMoveAndUpdateState(ctx context.Context, m *Move, fen string, status Status, result *string) error
	UpdateGameState(ctx context.Context, id string, fen string, status Status, result *string) error
}
