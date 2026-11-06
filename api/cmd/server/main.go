package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/Shyam576/chess-mind/api/internal/auth"
	"github.com/Shyam576/chess-mind/api/internal/config"
	"github.com/Shyam576/chess-mind/api/internal/db"
	"github.com/Shyam576/chess-mind/api/internal/db/sqlcgen"
	"github.com/Shyam576/chess-mind/api/internal/game"
	"github.com/Shyam576/chess-mind/api/internal/httpapi"
	"github.com/Shyam576/chess-mind/api/internal/stockfish"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	migrationDB, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("opening migration connection: %v", err)
	}
	if err := goose.SetDialect("postgres"); err != nil {
		log.Fatalf("setting goose dialect: %v", err)
	}
	if err := goose.Up(migrationDB, "internal/db/migrations"); err != nil {
		log.Fatalf("running migrations: %v", err)
	}
	migrationDB.Close()

	pool, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connecting to database: %v", err)
	}
	defer pool.Close()

	queries := sqlcgen.New(pool)
	userRepo := db.NewUserRepository(queries)
	gameRepo := db.NewGameRepository(pool)

	enginePool, err := stockfish.NewPool(cfg.StockfishPath, cfg.StockfishPoolSize)
	if err != nil {
		log.Fatalf("starting stockfish pool: %v", err)
	}
	defer enginePool.Close()

	authSvc := auth.NewService(userRepo, []byte(cfg.JWTSecret), 24*time.Hour)
	gameSvc := game.NewService(gameRepo, enginePool)
	router := httpapi.NewRouter(authSvc, gameSvc, []byte(cfg.JWTSecret))

	srv := &http.Server{Addr: ":" + cfg.Port, Handler: router}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("listening on :%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}
}
