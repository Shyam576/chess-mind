package db

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/Shyam576/chess-mind/api/internal/auth"
	"github.com/Shyam576/chess-mind/api/internal/db/sqlcgen"
)

type userRepository struct {
	q *sqlcgen.Queries
}

func NewUserRepository(q *sqlcgen.Queries) auth.UserRepository {
	return &userRepository{q: q}
}

func (r *userRepository) CreateUser(ctx context.Context, u *auth.User) error {
	row, err := r.q.CreateUser(ctx, sqlcgen.CreateUserParams{
		Email:        u.Email,
		PasswordHash: u.PasswordHash,
	})
	if err != nil {
		return err
	}
	u.ID = uuidToString(row.ID)
	u.CreatedAt = row.CreatedAt.Time
	return nil
}

func (r *userRepository) GetUserByEmail(ctx context.Context, email string) (*auth.User, error) {
	row, err := r.q.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, auth.ErrUserNotFound
		}
		return nil, err
	}
	return &auth.User{
		ID:           uuidToString(row.ID),
		Email:        row.Email,
		PasswordHash: row.PasswordHash,
		CreatedAt:    row.CreatedAt.Time,
	}, nil
}
