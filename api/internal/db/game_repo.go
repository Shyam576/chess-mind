package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Shyam576/chess-mind/api/internal/db/sqlcgen"
	"github.com/Shyam576/chess-mind/api/internal/game"
)

type gameRepository struct {
	pool *pgxpool.Pool
	q    *sqlcgen.Queries
}

func NewGameRepository(pool *pgxpool.Pool) game.Repository {
	return &gameRepository{pool: pool, q: sqlcgen.New(pool)}
}

func parseUUID(s string) (pgtype.UUID, error) {
	var u pgtype.UUID
	err := u.Scan(s)
	return u, err
}

// uuidToString formats a pgtype.UUID in canonical 8-4-4-4-12 form.
// pgtype.UUID has no String method in pgx v5.
func uuidToString(u pgtype.UUID) string {
	b := u.Bytes
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func toDomainGame(row sqlcgen.Game) *game.Game {
	var result *string
	if row.Result.Valid {
		r := row.Result.String
		result = &r
	}
	return &game.Game{
		ID:          uuidToString(row.ID),
		UserID:      uuidToString(row.UserID),
		Status:      game.Status(row.Status),
		PlayerColor: game.Color(row.PlayerColor),
		Difficulty:  int(row.Difficulty),
		CurrentFEN:  row.CurrentFen,
		Result:      result,
		CreatedAt:   row.CreatedAt.Time,
		UpdatedAt:   row.UpdatedAt.Time,
	}
}

func (r *gameRepository) CreateGame(ctx context.Context, g *game.Game) error {
	row, err := r.q.CreateGame(ctx, sqlcgen.CreateGameParams{
		UserID:      mustUUID(g.UserID),
		Status:      string(g.Status),
		PlayerColor: string(g.PlayerColor),
		Difficulty:  int32(g.Difficulty),
		CurrentFen:  g.CurrentFEN,
	})
	if err != nil {
		return err
	}
	g.ID = uuidToString(row.ID)
	g.CreatedAt = row.CreatedAt.Time
	g.UpdatedAt = row.UpdatedAt.Time
	return nil
}

func (r *gameRepository) GetGame(ctx context.Context, id string) (*game.Game, error) {
	uid, err := parseUUID(id)
	if err != nil {
		return nil, game.ErrNotFound
	}
	row, err := r.q.GetGame(ctx, uid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, game.ErrNotFound
		}
		return nil, err
	}
	return toDomainGame(row), nil
}

func (r *gameRepository) ListGamesByUser(ctx context.Context, userID string, limit, offset int) ([]*game.Game, error) {
	rows, err := r.q.ListGamesByUser(ctx, sqlcgen.ListGamesByUserParams{
		UserID: mustUUID(userID),
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, err
	}
	out := make([]*game.Game, len(rows))
	for i, row := range rows {
		out[i] = toDomainGame(row)
	}
	return out, nil
}

func (r *gameRepository) ListMoves(ctx context.Context, gameID string) ([]*game.Move, error) {
	rows, err := r.q.ListMoves(ctx, mustUUID(gameID))
	if err != nil {
		return nil, err
	}
	out := make([]*game.Move, len(rows))
	for i, row := range rows {
		out[i] = &game.Move{
			ID:        uuidToString(row.ID),
			GameID:    uuidToString(row.GameID),
			PlyNumber: int(row.PlyNumber),
			MoveSAN:   row.MoveSan,
			MoveUCI:   row.MoveUci,
			FENAfter:  row.FenAfter,
			CreatedAt: row.CreatedAt.Time,
		}
	}
	return out, nil
}

// AppendMoveAndUpdateState persists the move and the game's resulting state
// in a single transaction: if either write fails, both are rolled back, so a
// move is never recorded without its corresponding game-state update.
func (r *gameRepository) AppendMoveAndUpdateState(ctx context.Context, m *game.Move, fen string, status game.Status, result *string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txQueries := sqlcgen.New(tx)

	row, err := txQueries.AppendMove(ctx, sqlcgen.AppendMoveParams{
		GameID:    mustUUID(m.GameID),
		PlyNumber: int32(m.PlyNumber),
		MoveSan:   m.MoveSAN,
		MoveUci:   m.MoveUCI,
		FenAfter:  m.FENAfter,
	})
	if err != nil {
		return err
	}

	var pgResult pgtype.Text
	if result != nil {
		pgResult = pgtype.Text{String: *result, Valid: true}
	}
	if err := txQueries.UpdateGameState(ctx, sqlcgen.UpdateGameStateParams{
		ID:         mustUUID(m.GameID),
		CurrentFen: fen,
		Status:     string(status),
		Result:     pgResult,
	}); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}

	m.ID = uuidToString(row.ID)
	m.CreatedAt = row.CreatedAt.Time
	return nil
}

func (r *gameRepository) UpdateGameState(ctx context.Context, id string, fen string, status game.Status, result *string) error {
	var pgResult pgtype.Text
	if result != nil {
		pgResult = pgtype.Text{String: *result, Valid: true}
	}
	return r.q.UpdateGameState(ctx, sqlcgen.UpdateGameStateParams{
		ID:         mustUUID(id),
		CurrentFen: fen,
		Status:     string(status),
		Result:     pgResult,
	})
}

func mustUUID(s string) pgtype.UUID {
	u, err := parseUUID(s)
	if err != nil {
		panic("invalid uuid passed to repository: " + s)
	}
	return u
}
