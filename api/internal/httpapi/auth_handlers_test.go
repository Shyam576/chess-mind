package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Shyam576/chess-mind/api/internal/auth"
)

type memUserRepo struct {
	byEmail map[string]*auth.User
}

func newMemUserRepo() *memUserRepo { return &memUserRepo{byEmail: map[string]*auth.User{}} }

func (m *memUserRepo) CreateUser(ctx context.Context, u *auth.User) error {
	if _, ok := m.byEmail[u.Email]; ok {
		return auth.ErrUserExists
	}
	m.byEmail[u.Email] = u
	return nil
}

func (m *memUserRepo) GetUserByEmail(ctx context.Context, email string) (*auth.User, error) {
	u, ok := m.byEmail[email]
	if !ok {
		return nil, auth.ErrUserNotFound
	}
	return u, nil
}

func TestRegisterAndLogin(t *testing.T) {
	authSvc := auth.NewService(newMemUserRepo(), []byte("test-secret"), time.Hour)
	router := NewRouter(authSvc, nil, []byte("test-secret"))

	body, _ := json.Marshal(map[string]string{"email": "a@example.com", "password": "hunter2hunter2"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)

	body, _ = json.Marshal(map[string]string{"email": "a@example.com", "password": "hunter2hunter2"})
	req = httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.NotEmpty(t, rec.Result().Cookies())
}

func TestLogin_InvalidCredentials(t *testing.T) {
	authSvc := auth.NewService(newMemUserRepo(), []byte("test-secret"), time.Hour)
	router := NewRouter(authSvc, nil, []byte("test-secret"))

	body, _ := json.Marshal(map[string]string{"email": "nobody@example.com", "password": "wrongpassword"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}
