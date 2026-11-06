package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Shyam576/chess-mind/api/internal/auth"
	"github.com/Shyam576/chess-mind/api/internal/game"
	"github.com/Shyam576/chess-mind/api/internal/stockfish"
)

type memGameRepo struct {
	games map[string]*game.Game
	moves map[string][]*game.Move
}

func newMemGameRepo() *memGameRepo {
	return &memGameRepo{games: map[string]*game.Game{}, moves: map[string][]*game.Move{}}
}

func (r *memGameRepo) CreateGame(ctx context.Context, g *game.Game) error {
	r.games[g.ID] = g
	return nil
}
func (r *memGameRepo) GetGame(ctx context.Context, id string) (*game.Game, error) {
	g, ok := r.games[id]
	if !ok {
		return nil, game.ErrNotFound
	}
	return g, nil
}
func (r *memGameRepo) ListGamesByUser(ctx context.Context, userID string, limit, offset int) ([]*game.Game, error) {
	var out []*game.Game
	for _, g := range r.games {
		if g.UserID == userID {
			out = append(out, g)
		}
	}
	return out, nil
}
func (r *memGameRepo) ListMoves(ctx context.Context, gameID string) ([]*game.Move, error) {
	return r.moves[gameID], nil
}
func (r *memGameRepo) AppendMoveAndUpdateState(ctx context.Context, m *game.Move, fen string, status game.Status, result *string) error {
	r.moves[m.GameID] = append(r.moves[m.GameID], m)
	g := r.games[m.GameID]
	g.CurrentFEN = fen
	g.Status = status
	g.Result = result
	return nil
}
func (r *memGameRepo) UpdateGameState(ctx context.Context, id string, fen string, status game.Status, result *string) error {
	g := r.games[id]
	g.CurrentFEN = fen
	g.Status = status
	g.Result = result
	return nil
}

type memUserRepo2 struct{ byEmail map[string]*auth.User }

func newMemUserRepo2() *memUserRepo2 { return &memUserRepo2{byEmail: map[string]*auth.User{}} }
func (m *memUserRepo2) CreateUser(ctx context.Context, u *auth.User) error {
	m.byEmail[u.Email] = u
	return nil
}
func (m *memUserRepo2) GetUserByEmail(ctx context.Context, email string) (*auth.User, error) {
	u, ok := m.byEmail[email]
	if !ok {
		return nil, auth.ErrUserNotFound
	}
	return u, nil
}

func setupRouterWithUser(t *testing.T) (http.Handler, string) {
	t.Helper()
	authSvc := auth.NewService(newMemUserRepo2(), []byte("test-secret"), time.Hour)
	gameSvc := game.NewService(newMemGameRepo(), &stockfish.StubEngine{Move: "e7e5"})
	router := NewRouter(authSvc, gameSvc, []byte("test-secret"))

	body, _ := json.Marshal(map[string]string{"email": "player@example.com", "password": "hunter2hunter2"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)

	req = httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	cookies := rec.Result().Cookies()
	require.NotEmpty(t, cookies)
	return router, cookies[0].Value
}

func authedRequest(method, path, token string, body []byte) *http.Request {
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.AddCookie(&http.Cookie{Name: "session", Value: token})
	return req
}

func TestCreateAndPlayGame(t *testing.T) {
	router, token := setupRouterWithUser(t)

	body, _ := json.Marshal(map[string]interface{}{"color": "white", "difficulty": 5})
	req := authedRequest(http.MethodPost, "/api/games", token, body)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)

	var created gameResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&created))

	moveBody, _ := json.Marshal(map[string]string{"from": "e2", "to": "e4"})
	req = authedRequest(http.MethodPost, "/api/games/"+created.ID+"/moves", token, moveBody)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]interface{}
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	moves := resp["moves"].([]interface{})
	require.Len(t, moves, 2)
}

func TestSubmitMove_IllegalMoveReturns400(t *testing.T) {
	router, token := setupRouterWithUser(t)

	body, _ := json.Marshal(map[string]interface{}{"color": "white", "difficulty": 5})
	req := authedRequest(http.MethodPost, "/api/games", token, body)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	var created gameResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&created))

	moveBody, _ := json.Marshal(map[string]string{"from": "e2", "to": "e5"})
	req = authedRequest(http.MethodPost, "/api/games/"+created.ID+"/moves", token, moveBody)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestSubmitMove_Unauthenticated(t *testing.T) {
	router, _ := setupRouterWithUser(t)
	req := httptest.NewRequest(http.MethodGet, "/api/games", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}
