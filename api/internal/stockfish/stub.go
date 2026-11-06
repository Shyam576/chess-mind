package stockfish

import (
	"context"
	"time"
)

// StubEngine is a test double for Engine that returns a fixed move or error.
type StubEngine struct {
	Move string
	Err  error
}

func (s *StubEngine) BestMove(ctx context.Context, movesUCI []string, skillLevel int, moveTime time.Duration) (string, error) {
	if s.Err != nil {
		return "", s.Err
	}
	return s.Move, nil
}
