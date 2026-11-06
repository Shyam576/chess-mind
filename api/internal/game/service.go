package game

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/Shyam576/chess-mind/api/internal/chessrules"
	"github.com/Shyam576/chess-mind/api/internal/stockfish"
)

type Service struct {
	repo   Repository
	engine stockfish.Engine
}

func NewService(repo Repository, engine stockfish.Engine) *Service {
	return &Service{repo: repo, engine: engine}
}

func (s *Service) CreateGame(ctx context.Context, userID string, color Color, difficulty int) (*Game, error) {
	g := &Game{
		ID:          uuid.NewString(),
		UserID:      userID,
		Status:      StatusInProgress,
		PlayerColor: color,
		Difficulty:  difficulty,
		CurrentFEN:  chessrules.NewGameFEN(),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := s.repo.CreateGame(ctx, g); err != nil {
		return nil, err
	}
	if color == ColorBlack {
		if _, _, err := s.playBotMove(ctx, g, nil); err != nil {
			return nil, err
		}
	}
	return g, nil
}

func (s *Service) GetGame(ctx context.Context, userID, gameID string) (*Game, []*Move, error) {
	g, err := s.repo.GetGame(ctx, gameID)
	if err != nil {
		return nil, nil, err
	}
	if g.UserID != userID {
		return nil, nil, ErrForbidden
	}
	moves, err := s.repo.ListMoves(ctx, gameID)
	if err != nil {
		return nil, nil, err
	}
	return g, moves, nil
}

func (s *Service) ListGames(ctx context.Context, userID string, limit, offset int) ([]*Game, error) {
	return s.repo.ListGamesByUser(ctx, userID, limit, offset)
}

type MoveRequest struct {
	From      string
	To        string
	Promotion string
}

func uciList(moves []*Move) []string {
	out := make([]string, len(moves))
	for i, m := range moves {
		out[i] = m.MoveUCI
	}
	return out
}

func plyColor(ply int) Color {
	if ply%2 == 0 {
		return ColorWhite
	}
	return ColorBlack
}

func (s *Service) SubmitMove(ctx context.Context, userID, gameID string, req MoveRequest) (*Game, []*Move, error) {
	g, err := s.repo.GetGame(ctx, gameID)
	if err != nil {
		return nil, nil, err
	}
	if g.UserID != userID {
		return nil, nil, ErrForbidden
	}
	if g.Status != StatusInProgress {
		return nil, nil, ErrGameFinished
	}

	existing, err := s.repo.ListMoves(ctx, gameID)
	if err != nil {
		return nil, nil, err
	}
	if plyColor(len(existing)) != g.PlayerColor {
		return nil, nil, ErrNotYourTurn
	}

	prior := uciList(existing)
	result, err := chessrules.ApplyMove(prior, req.From, req.To, req.Promotion)
	if err != nil {
		return nil, nil, err
	}

	playerMove := &Move{
		ID:        uuid.NewString(),
		GameID:    gameID,
		PlyNumber: len(existing),
		MoveSAN:   result.SAN,
		MoveUCI:   result.UCI,
		FENAfter:  result.FEN,
		CreatedAt: time.Now(),
	}
	if err := s.repo.AppendMove(ctx, playerMove); err != nil {
		return nil, nil, err
	}

	newStatus := Status(result.Status)
	if err := s.repo.UpdateGameState(ctx, gameID, result.FEN, newStatus, result.Result); err != nil {
		return nil, nil, err
	}
	g.CurrentFEN = result.FEN
	g.Status = newStatus
	g.Result = result.Result

	newMoves := []*Move{playerMove}
	if g.Status == StatusInProgress {
		botMove, moves, err := s.playBotMove(ctx, g, append(prior, result.UCI))
		if err != nil {
			return nil, nil, err
		}
		if botMove != nil {
			newMoves = append(newMoves, botMove)
		}
		_ = moves
	}

	return g, newMoves, nil
}

// playBotMove asks the engine for a move in the current position (priorUCI) and
// applies it, mutating g in place and persisting the move and updated game state.
// priorUCI is nil when the bot must play first (player chose black).
func (s *Service) playBotMove(ctx context.Context, g *Game, priorUCI []string) (*Move, []string, error) {
	skillLevel := g.Difficulty
	moveTime := 500 * time.Millisecond
	best, err := s.engine.BestMove(ctx, priorUCI, skillLevel, moveTime)
	if err != nil {
		return nil, nil, err
	}

	from, to, promotion := best[0:2], best[2:4], ""
	if len(best) > 4 {
		promotion = best[4:5]
	}

	result, err := chessrules.ApplyMove(priorUCI, from, to, promotion)
	if err != nil {
		return nil, nil, err
	}

	existing, err := s.repo.ListMoves(ctx, g.ID)
	if err != nil {
		return nil, nil, err
	}

	botMove := &Move{
		ID:        uuid.NewString(),
		GameID:    g.ID,
		PlyNumber: len(existing),
		MoveSAN:   result.SAN,
		MoveUCI:   result.UCI,
		FENAfter:  result.FEN,
		CreatedAt: time.Now(),
	}
	if err := s.repo.AppendMove(ctx, botMove); err != nil {
		return nil, nil, err
	}

	newStatus := Status(result.Status)
	if err := s.repo.UpdateGameState(ctx, g.ID, result.FEN, newStatus, result.Result); err != nil {
		return nil, nil, err
	}
	g.CurrentFEN = result.FEN
	g.Status = newStatus
	g.Result = result.Result

	return botMove, append(priorUCI, result.UCI), nil
}

func (s *Service) Resign(ctx context.Context, userID, gameID string) (*Game, error) {
	g, err := s.repo.GetGame(ctx, gameID)
	if err != nil {
		return nil, err
	}
	if g.UserID != userID {
		return nil, ErrForbidden
	}
	if g.Status != StatusInProgress {
		return nil, ErrGameFinished
	}
	result := "resigned"
	if err := s.repo.UpdateGameState(ctx, gameID, g.CurrentFEN, StatusResigned, &result); err != nil {
		return nil, err
	}
	g.Status = StatusResigned
	g.Result = &result
	return g, nil
}
