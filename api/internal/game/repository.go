package game

import "context"

type Repository interface {
	CreateGame(ctx context.Context, g *Game) error
	GetGame(ctx context.Context, id string) (*Game, error)
	ListGamesByUser(ctx context.Context, userID string, limit, offset int) ([]*Game, error)
	ListMoves(ctx context.Context, gameID string) ([]*Move, error)
	AppendMove(ctx context.Context, m *Move) error
	UpdateGameState(ctx context.Context, id string, fen string, status Status, result *string) error
}
