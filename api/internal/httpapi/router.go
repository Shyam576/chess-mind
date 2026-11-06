package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/Shyam576/chess-mind/api/internal/auth"
	"github.com/Shyam576/chess-mind/api/internal/game"
)

func NewRouter(authSvc *auth.Service, gameSvc *game.Service, jwtSecret []byte) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(recoverMiddleware)

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	ah := &authHandler{svc: authSvc}
	r.Route("/api/auth", func(r chi.Router) {
		r.Post("/register", ah.register)
		r.Post("/login", ah.login)
		r.Post("/logout", ah.logout)
	})

	// The /api/games route group is added in Task 10, once gameHandler exists.
	// gameSvc and jwtSecret are accepted here (unused for now, which is not
	// a compile error for function parameters in Go) so this function's
	// signature never has to change.
	return r
}
