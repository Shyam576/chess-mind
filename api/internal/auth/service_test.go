package auth

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type memRepo struct {
	byEmail map[string]*User
}

func newMemRepo() *memRepo { return &memRepo{byEmail: map[string]*User{}} }

func (m *memRepo) CreateUser(ctx context.Context, u *User) error {
	if _, ok := m.byEmail[u.Email]; ok {
		return ErrUserExists
	}
	m.byEmail[u.Email] = u
	return nil
}

func (m *memRepo) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	u, ok := m.byEmail[email]
	if !ok {
		return nil, ErrUserNotFound
	}
	return u, nil
}

func TestRegister_Success(t *testing.T) {
	svc := NewService(newMemRepo(), []byte("secret"), time.Hour)
	u, err := svc.Register(context.Background(), "a@example.com", "hunter2hunter2")
	require.NoError(t, err)
	assert.Equal(t, "a@example.com", u.Email)
	assert.NotEqual(t, "hunter2hunter2", u.PasswordHash)
}

func TestRegister_DuplicateEmail(t *testing.T) {
	svc := NewService(newMemRepo(), []byte("secret"), time.Hour)
	ctx := context.Background()
	_, err := svc.Register(ctx, "a@example.com", "hunter2hunter2")
	require.NoError(t, err)
	_, err = svc.Register(ctx, "a@example.com", "otherpassword")
	assert.ErrorIs(t, err, ErrUserExists)
}

func TestLogin_Success(t *testing.T) {
	svc := NewService(newMemRepo(), []byte("secret"), time.Hour)
	ctx := context.Background()
	_, err := svc.Register(ctx, "a@example.com", "hunter2hunter2")
	require.NoError(t, err)

	token, user, err := svc.Login(ctx, "a@example.com", "hunter2hunter2")
	require.NoError(t, err)
	assert.NotEmpty(t, token)
	assert.Equal(t, "a@example.com", user.Email)

	userID, err := VerifyToken([]byte("secret"), token)
	require.NoError(t, err)
	assert.Equal(t, user.ID, userID)
}

func TestLogin_WrongPassword(t *testing.T) {
	svc := NewService(newMemRepo(), []byte("secret"), time.Hour)
	ctx := context.Background()
	_, err := svc.Register(ctx, "a@example.com", "hunter2hunter2")
	require.NoError(t, err)

	_, _, err = svc.Login(ctx, "a@example.com", "wrongpassword")
	assert.ErrorIs(t, err, ErrInvalidCredentials)
}

func TestLogin_UnknownEmail(t *testing.T) {
	svc := NewService(newMemRepo(), []byte("secret"), time.Hour)
	_, _, err := svc.Login(context.Background(), "nobody@example.com", "hunter2hunter2")
	assert.ErrorIs(t, err, ErrInvalidCredentials)
}
