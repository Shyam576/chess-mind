package game

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Shyam576/chess-mind/api/internal/chessrules"
	"github.com/Shyam576/chess-mind/api/internal/stockfish"
)

type memRepo struct {
	games map[string]*Game
	moves map[string][]*Move
}

func newMemRepo() *memRepo {
	return &memRepo{games: map[string]*Game{}, moves: map[string][]*Move{}}
}

func (r *memRepo) CreateGame(ctx context.Context, g *Game) error {
	r.games[g.ID] = g
	return nil
}

func (r *memRepo) GetGame(ctx context.Context, id string) (*Game, error) {
	g, ok := r.games[id]
	if !ok {
		return nil, ErrNotFound
	}
	return g, nil
}

func (r *memRepo) ListGamesByUser(ctx context.Context, userID string, limit, offset int) ([]*Game, error) {
	var out []*Game
	for _, g := range r.games {
		if g.UserID == userID {
			out = append(out, g)
		}
	}
	return out, nil
}

func (r *memRepo) ListMoves(ctx context.Context, gameID string) ([]*Move, error) {
	return r.moves[gameID], nil
}

func (r *memRepo) AppendMove(ctx context.Context, m *Move) error {
	r.moves[m.GameID] = append(r.moves[m.GameID], m)
	return nil
}

func (r *memRepo) UpdateGameState(ctx context.Context, id string, fen string, status Status, result *string) error {
	g := r.games[id]
	g.CurrentFEN = fen
	g.Status = status
	g.Result = result
	return nil
}

func TestCreateGame(t *testing.T) {
	repo := newMemRepo()
	svc := NewService(repo, &stockfish.StubEngine{Move: "e7e5"})
	g, err := svc.CreateGame(context.Background(), "user-1", ColorWhite, 5)
	require.NoError(t, err)
	assert.Equal(t, chessrules.NewGameFEN(), g.CurrentFEN)
	assert.Equal(t, StatusInProgress, g.Status)
}

func TestSubmitMove_PlayerThenBotReply(t *testing.T) {
	repo := newMemRepo()
	svc := NewService(repo, &stockfish.StubEngine{Move: "e7e5"})
	ctx := context.Background()
	g, err := svc.CreateGame(ctx, "user-1", ColorWhite, 5)
	require.NoError(t, err)

	updated, moves, err := svc.SubmitMove(ctx, "user-1", g.ID, MoveRequest{From: "e2", To: "e4"})
	require.NoError(t, err)
	require.Len(t, moves, 2)
	assert.Equal(t, "e4", moves[0].MoveSAN)
	assert.Equal(t, "e5", moves[1].MoveSAN)
	assert.Equal(t, StatusInProgress, updated.Status)
}

func TestSubmitMove_WrongTurn(t *testing.T) {
	repo := newMemRepo()
	svc := NewService(repo, &stockfish.StubEngine{Move: "e7e5"})
	ctx := context.Background()
	g, err := svc.CreateGame(ctx, "user-1", ColorWhite, 5)
	require.NoError(t, err)

	// Simulate an out-of-band move (e.g. a concurrent request) so the
	// recorded ply count says it is Black's turn, even though the
	// registered player for this game is White.
	require.NoError(t, repo.AppendMove(ctx, &Move{GameID: g.ID, PlyNumber: 0, MoveSAN: "e4", MoveUCI: "e2e4", FENAfter: g.CurrentFEN}))

	_, _, err = svc.SubmitMove(ctx, "user-1", g.ID, MoveRequest{From: "d2", To: "d4"})
	assert.ErrorIs(t, err, ErrNotYourTurn)
}

func TestSubmitMove_NotOwner(t *testing.T) {
	repo := newMemRepo()
	svc := NewService(repo, &stockfish.StubEngine{Move: "e7e5"})
	ctx := context.Background()
	g, err := svc.CreateGame(ctx, "user-1", ColorWhite, 5)
	require.NoError(t, err)

	_, _, err = svc.SubmitMove(ctx, "someone-else", g.ID, MoveRequest{From: "e2", To: "e4"})
	assert.ErrorIs(t, err, ErrForbidden)
}

func TestSubmitMove_IllegalMove(t *testing.T) {
	repo := newMemRepo()
	svc := NewService(repo, &stockfish.StubEngine{Move: "e7e5"})
	ctx := context.Background()
	g, err := svc.CreateGame(ctx, "user-1", ColorWhite, 5)
	require.NoError(t, err)

	_, _, err = svc.SubmitMove(ctx, "user-1", g.ID, MoveRequest{From: "e2", To: "e5"})
	assert.ErrorIs(t, err, chessrules.ErrIllegalMove)
}

func TestResign(t *testing.T) {
	repo := newMemRepo()
	svc := NewService(repo, &stockfish.StubEngine{Move: "e7e5"})
	ctx := context.Background()
	g, err := svc.CreateGame(ctx, "user-1", ColorWhite, 5)
	require.NoError(t, err)

	updated, err := svc.Resign(ctx, "user-1", g.ID)
	require.NoError(t, err)
	assert.Equal(t, StatusResigned, updated.Status)

	_, _, err = svc.SubmitMove(ctx, "user-1", g.ID, MoveRequest{From: "e2", To: "e4"})
	assert.ErrorIs(t, err, ErrGameFinished)
}
