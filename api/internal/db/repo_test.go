package db

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/Shyam576/chess-mind/api/internal/auth"
	"github.com/Shyam576/chess-mind/api/internal/db/sqlcgen"
	"github.com/Shyam576/chess-mind/api/internal/game"
)

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping repository integration test")
	}
	pool, err := pgxpool.New(context.Background(), url)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

func TestUserRepository_CreateAndGet(t *testing.T) {
	pool := testPool(t)
	q := sqlcgen.New(pool)
	repo := NewUserRepository(q)

	email := "repo-test-" + time.Now().Format("150405.000000") + "@example.com"
	u := &auth.User{Email: email, PasswordHash: "hash"}
	require.NoError(t, repo.CreateUser(context.Background(), u))
	require.NotEmpty(t, u.ID)

	fetched, err := repo.GetUserByEmail(context.Background(), email)
	require.NoError(t, err)
	require.Equal(t, u.ID, fetched.ID)
}

func TestUserRepository_DuplicateEmailReturnsErrUserExists(t *testing.T) {
	pool := testPool(t)
	q := sqlcgen.New(pool)
	repo := NewUserRepository(q)
	ctx := context.Background()

	email := "repo-dup-test-" + time.Now().Format("150405.000000") + "@example.com"
	require.NoError(t, repo.CreateUser(ctx, &auth.User{Email: email, PasswordHash: "hash"}))

	err := repo.CreateUser(ctx, &auth.User{Email: email, PasswordHash: "hash"})
	require.True(t, errors.Is(err, auth.ErrUserExists), "want auth.ErrUserExists, got %v", err)
}

func TestGameRepository_CreateMoveAndUpdate(t *testing.T) {
	pool := testPool(t)
	q := sqlcgen.New(pool)
	userRepo := NewUserRepository(q)
	gameRepo := NewGameRepository(pool)
	ctx := context.Background()

	email := "repo-game-test-" + time.Now().Format("150405.000000") + "@example.com"
	u := &auth.User{Email: email, PasswordHash: "hash"}
	require.NoError(t, userRepo.CreateUser(ctx, u))

	g := &game.Game{
		UserID:      u.ID,
		Status:      game.StatusInProgress,
		PlayerColor: game.ColorWhite,
		Difficulty:  5,
		CurrentFEN:  "startpos-fen",
	}
	require.NoError(t, gameRepo.CreateGame(ctx, g))
	require.NotEmpty(t, g.ID)

	m := &game.Move{GameID: g.ID, PlyNumber: 0, MoveSAN: "e4", MoveUCI: "e2e4", FENAfter: "fen-after"}
	require.NoError(t, gameRepo.AppendMoveAndUpdateState(ctx, m, "fen-after", game.StatusInProgress, nil))

	moves, err := gameRepo.ListMoves(ctx, g.ID)
	require.NoError(t, err)
	require.Len(t, moves, 1)
	require.Equal(t, "e4", moves[0].MoveSAN)

	result := "1-0"
	require.NoError(t, gameRepo.UpdateGameState(ctx, g.ID, "new-fen", game.StatusCheckmate, &result))
	updated, err := gameRepo.GetGame(ctx, g.ID)
	require.NoError(t, err)
	require.Equal(t, game.StatusCheckmate, updated.Status)
	require.Equal(t, "new-fen", updated.CurrentFEN)
}
