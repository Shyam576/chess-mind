package game

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"

	"github.com/Shyam576/chess-mind/api/internal/chessrules"
	"github.com/Shyam576/chess-mind/api/internal/stockfish"
)

// botMoveTimeout bounds how long we wait for the engine, independent of the
// inbound HTTP request's lifetime.
const botMoveTimeout = 5 * time.Second

// detachedMoveCtx returns a context that is not canceled when the inbound
// request ends. An engine call must be allowed to finish even if the client
// disconnects — otherwise the game is left on the bot's turn with nothing to
// trigger it. It carries its own timeout so a hung engine can't block the
// caller forever.
func detachedMoveCtx(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), botMoveTimeout)
}

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
	s.healBotTurn(ctx, g, nil)
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
	if healed := s.healBotTurn(ctx, g, moves); healed != nil {
		moves = append(moves, healed)
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

	var newMoves []*Move
	if healed := s.healBotTurn(ctx, g, existing); healed != nil {
		existing = append(existing, healed)
		newMoves = append(newMoves, healed)
	}

	if g.Status != StatusInProgress {
		return nil, nil, ErrGameFinished
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
	newStatus := Status(result.Status)
	if err := s.repo.AppendMoveAndUpdateState(ctx, playerMove, result.FEN, newStatus, result.Result); err != nil {
		return nil, nil, err
	}
	g.CurrentFEN = result.FEN
	g.Status = newStatus
	g.Result = result.Result
	newMoves = append(newMoves, playerMove)

	if g.Status == StatusInProgress {
		botCtx, cancel := detachedMoveCtx(ctx)
		botMove, err := s.playBotMove(botCtx, g, append(prior, result.UCI))
		cancel()
		if err != nil {
			log.Printf("game: bot reply failed for game %s, will self-heal on next access: %v", g.ID, err)
		} else if botMove != nil {
			newMoves = append(newMoves, botMove)
		}
	}

	return g, newMoves, nil
}

// healBotTurn plays the bot's move if the stored state says it's the bot's
// turn — the signature of a bot reply that failed after the opponent ply was
// already committed. It mutates g in place when it plays a move, and returns
// the move it played (nil if none was needed or the attempt failed; a
// failure here is logged and left for the next call to retry, it never
// fails the caller's own operation).
func (s *Service) healBotTurn(ctx context.Context, g *Game, moves []*Move) *Move {
	if g.Status != StatusInProgress || plyColor(len(moves)) == g.PlayerColor {
		return nil
	}
	botCtx, cancel := detachedMoveCtx(ctx)
	defer cancel()
	botMove, err := s.playBotMove(botCtx, g, uciList(moves))
	if err != nil {
		log.Printf("game: self-heal bot move failed for game %s: %v", g.ID, err)
		return nil
	}
	return botMove
}

// playBotMove asks the engine for a move in the current position (priorUCI),
// applies it, and atomically persists the move together with the resulting
// game state. It mutates g in place to reflect the new state. priorUCI is nil
// when the bot must play first (player chose black).
func (s *Service) playBotMove(ctx context.Context, g *Game, priorUCI []string) (*Move, error) {
	skillLevel := g.Difficulty
	moveTime := 500 * time.Millisecond
	best, err := s.engine.BestMove(ctx, priorUCI, skillLevel, moveTime)
	if err != nil {
		return nil, err
	}
	if len(best) < 4 {
		return nil, fmt.Errorf("engine returned malformed move: %q", best)
	}

	from, to, promotion := best[0:2], best[2:4], ""
	if len(best) > 4 {
		promotion = best[4:5]
	}

	result, err := chessrules.ApplyMove(priorUCI, from, to, promotion)
	if err != nil {
		return nil, err
	}

	existing, err := s.repo.ListMoves(ctx, g.ID)
	if err != nil {
		return nil, err
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

	newStatus := Status(result.Status)
	if err := s.repo.AppendMoveAndUpdateState(ctx, botMove, result.FEN, newStatus, result.Result); err != nil {
		return nil, err
	}
	g.CurrentFEN = result.FEN
	g.Status = newStatus
	g.Result = result.Result

	return botMove, nil
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
