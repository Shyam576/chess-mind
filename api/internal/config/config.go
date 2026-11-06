package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	DatabaseURL       string
	JWTSecret         string
	Port              string
	StockfishPath     string
	StockfishPoolSize int
}

func Load() (*Config, error) {
	cfg := &Config{
		DatabaseURL:       os.Getenv("DATABASE_URL"),
		JWTSecret:         os.Getenv("JWT_SECRET"),
		Port:              getOr("PORT", "8080"),
		StockfishPath:     getOr("STOCKFISH_PATH", "/usr/games/stockfish"),
		StockfishPoolSize: 4,
	}
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}
	if v := os.Getenv("STOCKFISH_POOL_SIZE"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("STOCKFISH_POOL_SIZE must be an integer: %w", err)
		}
		cfg.StockfishPoolSize = n
	}
	return cfg, nil
}

func getOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
