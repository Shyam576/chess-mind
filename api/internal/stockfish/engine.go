package stockfish

import (
	"context"
	"time"
)

type Engine interface {
	BestMove(ctx context.Context, movesUCI []string, skillLevel int, moveTime time.Duration) (string, error)
}
