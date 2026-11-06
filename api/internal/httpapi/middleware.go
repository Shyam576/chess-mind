package httpapi

import (
	"context"
	"log"
	"net/http"
	"os"
	"runtime/debug"
	"strings"

	"github.com/Shyam576/chess-mind/api/internal/auth"
)

type contextKey string

const userIDContextKey contextKey = "userID"

func UserIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(userIDContextKey).(string)
	return id, ok
}

func requireAuth(secret []byte) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie("session")
			var tokenString string
			if err == nil {
				tokenString = cookie.Value
			} else if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
				tokenString = strings.TrimPrefix(h, "Bearer ")
			}
			if tokenString == "" {
				writeError(w, http.StatusUnauthorized, "unauthenticated", "missing session")
				return
			}
			userID, err := auth.VerifyToken(secret, tokenString)
			if err != nil {
				writeError(w, http.StatusUnauthorized, "unauthenticated", "invalid or expired session")
				return
			}
			ctx := context.WithValue(r.Context(), userIDContextKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// corsAllowedOrigin returns the single origin the API accepts cross-origin
// requests from. The frontend is a separate origin (its own Docker service
// and host port) from the API, and the browser enforces CORS for the
// cookie-authenticated requests it makes, so the API must explicitly allow
// that origin. Defaults to the frontend's local dev/compose origin; override
// with CORS_ALLOWED_ORIGIN for other environments.
func corsAllowedOrigin() string {
	if v := os.Getenv("CORS_ALLOWED_ORIGIN"); v != "" {
		return v
	}
	return "http://localhost:3001"
}

// corsMiddleware allows the configured frontend origin to make
// credentialed (cookie-based) cross-origin requests to the API, and answers
// the browser's CORS preflight (OPTIONS) requests that precede them. Only
// an exact-matching origin is ever echoed back (never "*"), since
// Access-Control-Allow-Credentials forbids a wildcard origin.
func corsMiddleware(next http.Handler) http.Handler {
	allowedOrigin := corsAllowedOrigin()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && origin == allowedOrigin {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Vary", "Origin")
		}
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func recoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("panic: %v\n%s", rec, debug.Stack())
				writeError(w, http.StatusInternalServerError, "internal_error", "an unexpected error occurred")
			}
		}()
		next.ServeHTTP(w, r)
	})
}
