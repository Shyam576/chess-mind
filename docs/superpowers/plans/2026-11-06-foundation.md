# ChessMind Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a complete, correct, playable chess application — auth, gameplay against Stockfish, persistence — with the Go backend authoritative over chess legality. No coaching features (future sub-projects).

**Architecture:** Monorepo (`/web` Next.js + TS + Tailwind, `/api` Go). Go backend independently validates every move with `notnil/chess` before persisting it (never trusts the client). Layered Go backend: thin HTTP handlers → services (business logic) → repository interfaces → sqlc/pgx-backed Postgres implementations. Stockfish runs as a small pool of long-lived UCI processes managed by the Go backend.

**Tech Stack:** Go 1.23, chi v5, pgx v5 + sqlc, goose (migrations), notnil/chess v1.10.0, golang-jwt v5, bcrypt, google/uuid, testify. Next.js 14 (App Router), TypeScript, Tailwind CSS, shadcn/ui, react-chessboard, chess.js, TanStack Query, Vitest + React Testing Library, Playwright. Postgres 16. Docker Compose for local dev.

**Spec:** `docs/superpowers/specs/2026-11-06-foundation-design.md` — read it alongside this plan; this plan implements it task by task.

## Global Constraints

- Backend is authoritative for chess legality: every move is re-validated server-side via `notnil/chess`, never trusted from the client alone.
- All external input (auth payloads, move submissions, pagination params) is validated at the HTTP handler boundary.
- Errors use the envelope `{"error": {"code": "...", "message": "..."}}`; no stack traces or internal details exposed to clients.
- No secrets (passwords, password hashes, JWTs) are ever logged.
- Append-only `moves` table — rows are never updated or deleted.
- Status codes: 400 invalid input/illegal move, 401 unauthenticated, 403 not the game's owner, 404 unknown game, 409 invalid state transition.
- Go backend layering: handlers are thin (parse/validate → call service → write response); services hold business logic; repositories are interfaces implemented by the sqlc/pgx-backed `internal/db` package, so services can be unit-tested against in-memory fakes without a real database.
- Commit after every task (and after meaningful sub-steps within large tasks) with a focused, meaningful message — never one bulk commit at the end.

---

## Task 1: Repo scaffold and local dev shell

**Files:**
- Create: `.gitignore`
- Create: `.env.example`
- Create: `docker-compose.yml`
- Modify: `README.md`

**Interfaces:**
- Produces: `docker-compose.yml` with a `postgres` service other tasks will extend with `api` and `web` services.

- [ ] **Step 1: Write `.gitignore`**

```gitignore
# Go
/api/tmp/
*.test

# Node
/web/node_modules/
/web/.next/
/web/.turbo/

# Env
.env
.env.local

# OS
.DS_Store
```

- [ ] **Step 2: Write `.env.example`**

```bash
DATABASE_URL=postgres://chessmind:chessmind@127.0.0.1:5434/chessmind?sslmode=disable
JWT_SECRET=dev-secret-change-me
PORT=8080
STOCKFISH_PATH=/usr/games/stockfish
STOCKFISH_POOL_SIZE=4
NEXT_PUBLIC_API_URL=http://localhost:8080
```

- [ ] **Step 3: Write `docker-compose.yml` (postgres only for now; `api`/`web` added in later tasks)**

```yaml
version: "3.9"
services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: chessmind
      POSTGRES_PASSWORD: chessmind
      POSTGRES_DB: chessmind
    ports:
      - "127.0.0.1:5434:5432"
    volumes:
      - pgdata:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U chessmind"]
      interval: 5s
      timeout: 5s
      retries: 10

volumes:
  pgdata:
```

Host port `5434` (not the default `5432`) is used because `5432` and `5433` may already be bound by other Postgres instances on the development machine; the container still listens on `5432` internally, so container-to-container traffic (the `api` service's `DATABASE_URL`, added in Task 11) is unaffected. The mapping is bound to `127.0.0.1` only (not `0.0.0.0`) so this dev database with well-known credentials isn't reachable from the LAN.

- [ ] **Step 4: Verify Postgres starts**

Run: `docker compose up -d postgres && docker compose ps`
Expected: `postgres` service is `running (healthy)`.

- [ ] **Step 5: Commit**

```bash
git add .gitignore .env.example docker-compose.yml README.md
git commit -m "chore: scaffold repo with env template and postgres compose service"
```

---

## Task 2: Go module, schema migrations

**Files:**
- Create: `api/go.mod`
- Create: `api/internal/db/migrations/00001_create_users.sql`
- Create: `api/internal/db/migrations/00002_create_games.sql`
- Create: `api/internal/db/migrations/00003_create_moves.sql`
- Create: `api/Makefile`

**Interfaces:**
- Produces: `users`, `games`, `moves` tables exactly as specified in the design spec §5.

- [ ] **Step 1: Initialize the Go module**

The `api/` directory does not exist yet — create it before `cd`-ing into it.

Run:
```bash
mkdir -p api
cd api
go mod init github.com/Shyam576/chess-mind/api
go get github.com/pressly/goose/v3@v3.21.1
```

- [ ] **Step 2: Write migration `00001_create_users.sql`**

```sql
-- +goose Up
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE users;
```

- [ ] **Step 3: Write migration `00002_create_games.sql`**

```sql
-- +goose Up
CREATE TABLE games (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    status TEXT NOT NULL,
    player_color TEXT NOT NULL,
    difficulty INT NOT NULL,
    current_fen TEXT NOT NULL,
    result TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_games_user_created ON games (user_id, created_at DESC);

-- +goose Down
DROP TABLE games;
```

- [ ] **Step 4: Write migration `00003_create_moves.sql`**

```sql
-- +goose Up
CREATE TABLE moves (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    game_id UUID NOT NULL REFERENCES games(id),
    ply_number INT NOT NULL,
    move_san TEXT NOT NULL,
    move_uci TEXT NOT NULL,
    fen_after TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_moves_game_ply ON moves (game_id, ply_number);

-- +goose Down
DROP TABLE moves;
```

- [ ] **Step 5: Write `api/Makefile` with a migrate target**

```makefile
include ../.env.example
-include ../.env

.PHONY: migrate-up migrate-down sqlc-generate test

migrate-up:
	goose -dir internal/db/migrations postgres "$(DATABASE_URL)" up

migrate-down:
	goose -dir internal/db/migrations postgres "$(DATABASE_URL)" down

sqlc-generate:
	sqlc generate

test:
	go test ./... -v
```

- [ ] **Step 6: Run migrations against local Postgres and verify**

Run:
```bash
cd api
go install github.com/pressly/goose/v3/cmd/goose@v3.21.1
export DATABASE_URL=postgres://chessmind:chessmind@127.0.0.1:5434/chessmind?sslmode=disable
make migrate-up
docker compose exec postgres psql -U chessmind -d chessmind -c '\dt'
```
Expected: `users`, `games`, `moves`, and goose's `goose_db_version` tables listed.

- [ ] **Step 7: Commit**

```bash
git add api/go.mod api/go.sum api/internal/db/migrations api/Makefile
git commit -m "feat: add Go module and users/games/moves schema migrations"
```

---

## Task 3: sqlc query generation

**Files:**
- Create: `api/sqlc.yaml`
- Create: `api/internal/db/queries/users.sql`
- Create: `api/internal/db/queries/games.sql`
- Create: `api/internal/db/queries/moves.sql`
- Create (generated): `api/internal/db/sqlcgen/*.go`

**Interfaces:**
- Produces: `sqlcgen.Queries` with methods `CreateUser`, `GetUserByEmail`, `CreateGame`, `GetGame`, `ListGamesByUser`, `UpdateGameState`, `AppendMove`, `ListMoves`, and generated structs `sqlcgen.User`, `sqlcgen.Game`, `sqlcgen.Move`. `sqlcgen.New(db DBTX) *Queries` where `DBTX` is satisfied by `*pgxpool.Pool`.

- [ ] **Step 1: Write `api/sqlc.yaml`**

```yaml
version: "2"
sql:
  - engine: "postgresql"
    queries: "internal/db/queries"
    schema: "internal/db/migrations"
    gen:
      go:
        package: "sqlcgen"
        out: "internal/db/sqlcgen"
        sql_package: "pgx/v5"
        emit_json_tags: true
        emit_interface: true
```

- [ ] **Step 2: Write `api/internal/db/queries/users.sql`**

```sql
-- name: CreateUser :one
INSERT INTO users (email, password_hash)
VALUES ($1, $2)
RETURNING *;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1;
```

- [ ] **Step 3: Write `api/internal/db/queries/games.sql`**

```sql
-- name: CreateGame :one
INSERT INTO games (user_id, status, player_color, difficulty, current_fen)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetGame :one
SELECT * FROM games WHERE id = $1;

-- name: ListGamesByUser :many
SELECT * FROM games WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3;

-- name: UpdateGameState :exec
UPDATE games SET current_fen = $2, status = $3, result = $4, updated_at = now() WHERE id = $1;
```

- [ ] **Step 4: Write `api/internal/db/queries/moves.sql`**

```sql
-- name: AppendMove :one
INSERT INTO moves (game_id, ply_number, move_san, move_uci, fen_after)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListMoves :many
SELECT * FROM moves WHERE game_id = $1 ORDER BY ply_number ASC;
```

- [ ] **Step 5: Generate code**

Run:
```bash
cd api
go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.27.0
make sqlc-generate
go build ./...
```
Expected: `internal/db/sqlcgen/` populated with `db.go`, `models.go`, `users.sql.go`, `games.sql.go`, `moves.sql.go`; build succeeds.

- [ ] **Step 6: Commit**

```bash
git add api/sqlc.yaml api/internal/db/queries api/internal/db/sqlcgen
git commit -m "feat: add sqlc queries and generated data access code"
```

---

## Task 4: Chess rules wrapper (`internal/chessrules`)

**Files:**
- Create: `api/internal/chessrules/chessrules.go`
- Test: `api/internal/chessrules/chessrules_test.go`

**Interfaces:**
- Consumes: `github.com/notnil/chess` (`chess.NewGame`, `chess.UCINotation{}.Decode/Encode`, `chess.AlgebraicNotation{}.Encode`, `game.Move`, `game.FEN`, `game.Outcome`, `game.Method`, `game.Position().Turn()`, `chess.StartingPosition().String()`).
- Produces:
  - `type Status string` with constants `StatusInProgress`, `StatusCheckmate`, `StatusStalemate`, `StatusDrawRepetition`, `StatusDrawFiftyMove`, `StatusDrawInsufficientMaterial`.
  - `type MoveResult struct { SAN, UCI, FEN string; Status Status; Result *string; Turn string }`
  - `var ErrIllegalMove = errors.New("illegal move")`
  - `func NewGameFEN() string`
  - `func ApplyMove(priorMovesUCI []string, from, to, promotion string) (MoveResult, error)`

- [ ] **Step 1: Add the dependency**

Run: `cd api && go get github.com/notnil/chess@v1.10.0`

- [ ] **Step 2: Write the failing tests**

```go
package chessrules

import "testing"

func TestNewGameFEN(t *testing.T) {
	fen := NewGameFEN()
	want := "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"
	if fen != want {
		t.Fatalf("got %q, want %q", fen, want)
	}
}

func TestApplyMove_LegalOpening(t *testing.T) {
	res, err := ApplyMove(nil, "e2", "e4", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.SAN != "e4" {
		t.Errorf("SAN = %q, want %q", res.SAN, "e4")
	}
	if res.UCI != "e2e4" {
		t.Errorf("UCI = %q, want %q", res.UCI, "e2e4")
	}
	if res.Status != StatusInProgress {
		t.Errorf("Status = %q, want %q", res.Status, StatusInProgress)
	}
	if res.Turn != "black" {
		t.Errorf("Turn = %q, want %q", res.Turn, "black")
	}
}

func TestApplyMove_IllegalMove(t *testing.T) {
	_, err := ApplyMove(nil, "e2", "e5", "")
	if err == nil {
		t.Fatal("expected an error for an illegal move")
	}
}

func TestApplyMove_SequenceAndCheckmate(t *testing.T) {
	// Fool's mate: 1. f3 e5 2. g4 Qh4#
	moves := [][3]string{
		{"f2", "f3", ""},
		{"e7", "e5", ""},
		{"g2", "g4", ""},
		{"d8", "h4", ""},
	}
	var prior []string
	var last MoveResult
	for _, m := range moves {
		res, err := ApplyMove(prior, m[0], m[1], m[2])
		if err != nil {
			t.Fatalf("move %v failed: %v", m, err)
		}
		prior = append(prior, res.UCI)
		last = res
	}
	if last.Status != StatusCheckmate {
		t.Errorf("Status = %q, want %q", last.Status, StatusCheckmate)
	}
	if last.Result == nil || *last.Result != "0-1" {
		t.Errorf("Result = %v, want \"0-1\"", last.Result)
	}
}

func TestApplyMove_Castling(t *testing.T) {
	prior := []string{}
	setup := [][3]string{
		{"g1", "f3", ""}, {"g8", "f6", ""},
		{"g2", "g3", ""}, {"g7", "g6", ""},
		{"f1", "g2", ""}, {"f8", "g7", ""},
	}
	for _, m := range setup {
		res, err := ApplyMove(prior, m[0], m[1], m[2])
		if err != nil {
			t.Fatalf("setup move %v failed: %v", m, err)
		}
		prior = append(prior, res.UCI)
	}
	res, err := ApplyMove(prior, "e1", "g1", "")
	if err != nil {
		t.Fatalf("kingside castle failed: %v", err)
	}
	if res.UCI != "e1g1" {
		t.Errorf("UCI = %q, want %q", res.UCI, "e1g1")
	}
}

func TestApplyMove_EnPassant(t *testing.T) {
	prior := []string{}
	setup := [][3]string{
		{"e2", "e4", ""}, {"a7", "a6", ""},
		{"e4", "e5", ""}, {"d7", "d5", ""},
	}
	for _, m := range setup {
		res, err := ApplyMove(prior, m[0], m[1], m[2])
		if err != nil {
			t.Fatalf("setup move %v failed: %v", m, err)
		}
		prior = append(prior, res.UCI)
	}
	res, err := ApplyMove(prior, "e5", "d6", "")
	if err != nil {
		t.Fatalf("en passant capture failed: %v", err)
	}
	if res.UCI != "e5d6" {
		t.Errorf("UCI = %q, want %q", res.UCI, "e5d6")
	}
}

func TestApplyMove_Promotion(t *testing.T) {
	// Build a position where white can promote: clear the board down to a pawn on the 7th rank
	// via a long forced sequence is brittle; instead verify promotion string plumbing using
	// notnil/chess's own UCI decode on a reachable promotion position reached from a few plies.
	prior := []string{}
	setup := [][3]string{
		{"a2", "a4", ""}, {"b7", "b5", ""},
		{"a4", "b5", ""}, {"a7", "a6", ""},
		{"b5", "a6", ""}, {"b8", "a6", ""},
		{"a1", "a6", ""}, {"c7", "c6", ""},
		{"a6", "a7", ""}, {"d7", "d6", ""},
	}
	for _, m := range setup {
		res, err := ApplyMove(prior, m[0], m[1], m[2])
		if err != nil {
			t.Fatalf("setup move %v failed: %v", m, err)
		}
		prior = append(prior, res.UCI)
	}
	res, err := ApplyMove(prior, "a7", "a8", "q")
	if err != nil {
		t.Fatalf("promotion move failed: %v", err)
	}
	if res.UCI != "a7a8q" {
		t.Errorf("UCI = %q, want %q", res.UCI, "a7a8q")
	}
}
```

- [ ] **Step 3: Run tests to verify they fail (package doesn't exist yet)**

Run: `cd api && go test ./internal/chessrules/... -v`
Expected: FAIL — `ApplyMove`/`NewGameFEN` undefined. (This step covers all seven test functions written in Step 2: starting FEN, a legal opening move, an illegal move, a full checkmate sequence, castling, en passant, and promotion.)

- [ ] **Step 4: Implement `api/internal/chessrules/chessrules.go`**

```go
package chessrules

import (
	"errors"

	"github.com/notnil/chess"
)

type Status string

const (
	StatusInProgress               Status = "in_progress"
	StatusCheckmate                Status = "checkmate"
	StatusStalemate                Status = "stalemate"
	StatusDrawRepetition           Status = "draw_repetition"
	StatusDrawFiftyMove            Status = "draw_fifty_move"
	StatusDrawInsufficientMaterial Status = "draw_insufficient_material"
)

var ErrIllegalMove = errors.New("illegal move")

type MoveResult struct {
	SAN    string
	UCI    string
	FEN    string
	Status Status
	Result *string
	Turn   string
}

func NewGameFEN() string {
	return chess.StartingPosition().String()
}

func replay(priorMovesUCI []string) (*chess.Game, error) {
	game := chess.NewGame()
	for _, mv := range priorMovesUCI {
		move, err := chess.UCINotation{}.Decode(game.Position(), mv)
		if err != nil {
			return nil, ErrIllegalMove
		}
		if err := game.Move(move); err != nil {
			return nil, ErrIllegalMove
		}
	}
	return game, nil
}

// ApplyMove replays priorMovesUCI (long algebraic, e.g. "e2e4", "e7e8q") from the
// standard starting position, then attempts to play from+to+promotion as the next
// move. promotion is "", "q", "r", "b", or "n".
func ApplyMove(priorMovesUCI []string, from, to, promotion string) (MoveResult, error) {
	game, err := replay(priorMovesUCI)
	if err != nil {
		return MoveResult{}, err
	}

	pos := game.Position()
	uciStr := from + to + promotion
	move, err := chess.UCINotation{}.Decode(pos, uciStr)
	if err != nil {
		return MoveResult{}, ErrIllegalMove
	}

	san := chess.AlgebraicNotation{}.Encode(pos, move)

	if err := game.Move(move); err != nil {
		return MoveResult{}, ErrIllegalMove
	}

	status, result := statusFromGame(game)
	turn := "white"
	if game.Position().Turn() == chess.Black {
		turn = "black"
	}

	return MoveResult{
		SAN:    san,
		UCI:    uciStr,
		FEN:    game.FEN(),
		Status: status,
		Result: result,
		Turn:   turn,
	}, nil
}

func statusFromGame(game *chess.Game) (Status, *string) {
	outcome := game.Outcome()
	if outcome == chess.NoOutcome {
		return StatusInProgress, nil
	}
	result := string(outcome)
	switch game.Method() {
	case chess.Checkmate:
		return StatusCheckmate, &result
	case chess.Stalemate:
		return StatusStalemate, &result
	case chess.ThreefoldRepetition, chess.FivefoldRepetition:
		return StatusDrawRepetition, &result
	case chess.FiftyMoveRule, chess.SeventyFiveMoveRule:
		return StatusDrawFiftyMove, &result
	case chess.InsufficientMaterial:
		return StatusDrawInsufficientMaterial, &result
	default:
		return StatusInProgress, nil
	}
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd api && go test ./internal/chessrules/... -v`
Expected: PASS for all seven tests.

- [ ] **Step 6: Commit**

```bash
git add api/go.mod api/go.sum api/internal/chessrules
git commit -m "feat: add authoritative chess rules wrapper around notnil/chess"
```

---

## Task 5: Stockfish engine interface, stub, and real UCI pool

**Files:**
- Create: `api/internal/stockfish/engine.go`
- Create: `api/internal/stockfish/stub.go`
- Create: `api/internal/stockfish/pool.go`
- Test: `api/internal/stockfish/pool_test.go`

**Interfaces:**
- Produces:
  - `type Engine interface { BestMove(ctx context.Context, movesUCI []string, skillLevel int, moveTime time.Duration) (string, error) }`
  - `type StubEngine struct { Move string; Err error }` implementing `Engine` (test double).
  - `func NewPool(binaryPath string, size int) (*Pool, error)` where `*Pool` implements `Engine`.
  - `func (p *Pool) Close() error`

- [ ] **Step 1: Write `api/internal/stockfish/engine.go`**

```go
package stockfish

import (
	"context"
	"time"
)

type Engine interface {
	BestMove(ctx context.Context, movesUCI []string, skillLevel int, moveTime time.Duration) (string, error)
}
```

- [ ] **Step 2: Write `api/internal/stockfish/stub.go`**

```go
package stockfish

import (
	"context"
	"time"
)

// StubEngine is a test double for Engine that returns a fixed move or error.
type StubEngine struct {
	Move string
	Err  error
}

func (s *StubEngine) BestMove(ctx context.Context, movesUCI []string, skillLevel int, moveTime time.Duration) (string, error) {
	if s.Err != nil {
		return "", s.Err
	}
	return s.Move, nil
}
```

- [ ] **Step 3: Write the failing pool test (uses a fake process via an injectable command, not a real Stockfish binary)**

```go
package stockfish

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// fakeEngineScript is a minimal shell "UCI engine" used only to test the Pool's
// process-management and parsing logic without depending on a real Stockfish binary.
const fakeEngineScript = `#!/bin/sh
while read -r line; do
  case "$line" in
    uci) echo "id name fake"; echo "uciok" ;;
    isready) echo "readyok" ;;
    go*) echo "bestmove e2e4" ;;
  esac
done
`

func writeFakeEngine(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := dir + "/fake-engine.sh"
	if err := exec.Command("sh", "-c", "cat > "+path).Run(); err != nil {
		t.Skip("sh not available to build fake engine script")
	}
	f, err := exec.Command("sh", "-c", "printf '%s' "+quote(fakeEngineScript)+" > "+path+" && chmod +x "+path).CombinedOutput()
	if err != nil {
		t.Fatalf("failed to write fake engine: %v (%s)", err, f)
	}
	return path
}

func quote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func TestPool_BestMove(t *testing.T) {
	path := writeFakeEngine(t)
	pool, err := NewPool(path, 2)
	if err != nil {
		t.Fatalf("NewPool failed: %v", err)
	}
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	move, err := pool.BestMove(ctx, nil, 10, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("BestMove failed: %v", err)
	}
	if move != "e2e4" {
		t.Errorf("move = %q, want %q", move, "e2e4")
	}
}
```

- [ ] **Step 4: Run test to verify it fails**

Run: `cd api && go test ./internal/stockfish/... -v`
Expected: FAIL — `NewPool` undefined.

- [ ] **Step 5: Implement `api/internal/stockfish/pool.go`**

```go
package stockfish

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type process struct {
	cmd    *exec.Cmd
	stdin  *bufio.Writer
	stdout *bufio.Scanner
	mu     sync.Mutex
}

func startProcess(binaryPath string) (*process, error) {
	cmd := exec.Command(binaryPath)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	p := &process{
		cmd:    cmd,
		stdin:  bufio.NewWriter(stdin),
		stdout: bufio.NewScanner(stdout),
	}
	if err := p.send("uci"); err != nil {
		return nil, err
	}
	if _, err := p.readUntil("uciok"); err != nil {
		return nil, err
	}
	return p, nil
}

func (p *process) send(cmd string) error {
	if _, err := p.stdin.WriteString(cmd + "\n"); err != nil {
		return err
	}
	return p.stdin.Flush()
}

func (p *process) readUntil(prefix string) (string, error) {
	for p.stdout.Scan() {
		line := p.stdout.Text()
		if strings.HasPrefix(line, prefix) {
			return line, nil
		}
	}
	if err := p.stdout.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("engine closed output before %q", prefix)
}

func (p *process) bestMove(ctx context.Context, movesUCI []string, skillLevel int, moveTime time.Duration) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if err := p.send("ucinewgame"); err != nil {
		return "", err
	}
	if err := p.send(fmt.Sprintf("setoption name Skill Level value %d", skillLevel)); err != nil {
		return "", err
	}
	posCmd := "position startpos"
	if len(movesUCI) > 0 {
		posCmd += " moves " + strings.Join(movesUCI, " ")
	}
	if err := p.send(posCmd); err != nil {
		return "", err
	}
	if err := p.send(fmt.Sprintf("go movetime %d", moveTime.Milliseconds())); err != nil {
		return "", err
	}

	type result struct {
		line string
		err  error
	}
	done := make(chan result, 1)
	go func() {
		line, err := p.readUntil("bestmove")
		done <- result{line, err}
	}()

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case r := <-done:
		if r.err != nil {
			return "", r.err
		}
		fields := strings.Fields(r.line)
		if len(fields) < 2 {
			return "", fmt.Errorf("malformed bestmove line: %q", r.line)
		}
		return fields[1], nil
	case <-time.After(moveTime + 5*time.Second):
		return "", fmt.Errorf("engine did not respond within deadline")
	}
}

func (p *process) close() error {
	_ = p.send("quit")
	return p.cmd.Wait()
}

type Pool struct {
	binaryPath string
	procs      chan *process
}

func NewPool(binaryPath string, size int) (*Pool, error) {
	pool := &Pool{
		binaryPath: binaryPath,
		procs:      make(chan *process, size),
	}
	for i := 0; i < size; i++ {
		p, err := startProcess(binaryPath)
		if err != nil {
			pool.Close()
			return nil, fmt.Errorf("starting engine process %d: %w", i, err)
		}
		pool.procs <- p
	}
	return pool, nil
}

func (p *Pool) BestMove(ctx context.Context, movesUCI []string, skillLevel int, moveTime time.Duration) (string, error) {
	var proc *process
	select {
	case proc = <-p.procs:
	case <-ctx.Done():
		return "", ctx.Err()
	}

	move, err := proc.bestMove(ctx, movesUCI, skillLevel, moveTime)
	if err != nil {
		_ = proc.close()
		fresh, startErr := startProcess(p.binaryPath)
		if startErr == nil {
			p.procs <- fresh
		}
		return "", err
	}
	p.procs <- proc
	return move, nil
}

func (p *Pool) Close() error {
	close(p.procs)
	for proc := range p.procs {
		_ = proc.close()
	}
	return nil
}
```

- [ ] **Step 6: Run tests**

Run: `cd api && go vet ./internal/stockfish/... && go test ./internal/stockfish/... -v`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add api/internal/stockfish
git commit -m "feat: add Stockfish Engine interface with stub and real UCI process pool"
```

---

## Task 6: Auth domain — hashing, JWT, service (repository-agnostic)

**Files:**
- Create: `api/internal/auth/auth.go`
- Create: `api/internal/auth/jwt.go`
- Create: `api/internal/auth/service.go`
- Test: `api/internal/auth/service_test.go`

**Interfaces:**
- Produces:
  - `type User struct { ID, Email, PasswordHash string; CreatedAt time.Time }`
  - `type UserRepository interface { CreateUser(ctx, *User) error; GetUserByEmail(ctx, email string) (*User, error) }`
  - `var ErrUserExists, ErrInvalidCredentials, ErrUserNotFound error`
  - `type Service struct{...}`, `func NewService(repo UserRepository, secret []byte, ttl time.Duration) *Service`
  - `func (s *Service) Register(ctx, email, password string) (*User, error)`
  - `func (s *Service) Login(ctx, email, password string) (token string, user *User, err error)`
  - `func IssueToken(secret []byte, userID string, ttl time.Duration) (string, error)`
  - `func VerifyToken(secret []byte, tokenString string) (string, error)` (returns userID)

- [ ] **Step 1: Add dependencies**

Run:
```bash
cd api
go get github.com/golang-jwt/jwt/v5@v5.2.1
go get golang.org/x/crypto@v0.27.0
go get github.com/google/uuid@v1.6.0
go get github.com/stretchr/testify@v1.9.0
```

- [ ] **Step 2: Write failing tests `api/internal/auth/service_test.go`**

```go
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
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `cd api && go test ./internal/auth/... -v`
Expected: FAIL — package types undefined.

- [ ] **Step 4: Implement `api/internal/auth/auth.go`**

```go
package auth

import (
	"context"
	"errors"
	"time"
)

type User struct {
	ID           string
	Email        string
	PasswordHash string
	CreatedAt    time.Time
}

type UserRepository interface {
	CreateUser(ctx context.Context, u *User) error
	GetUserByEmail(ctx context.Context, email string) (*User, error)
}

var (
	ErrUserExists         = errors.New("user already exists")
	ErrUserNotFound       = errors.New("user not found")
	ErrInvalidCredentials = errors.New("invalid credentials")
)
```

- [ ] **Step 5: Implement `api/internal/auth/jwt.go`**

```go
package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type claims struct {
	UserID string `json:"user_id"`
	jwt.RegisteredClaims
}

func IssueToken(secret []byte, userID string, ttl time.Duration) (string, error) {
	c := claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, c)
	return token.SignedString(secret)
}

func VerifyToken(secret []byte, tokenString string) (string, error) {
	var c claims
	token, err := jwt.ParseWithClaims(tokenString, &c, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return secret, nil
	})
	if err != nil || !token.Valid {
		return "", errors.New("invalid token")
	}
	return c.UserID, nil
}
```

- [ ] **Step 6: Implement `api/internal/auth/service.go`**

```go
package auth

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type Service struct {
	repo   UserRepository
	secret []byte
	ttl    time.Duration
}

func NewService(repo UserRepository, secret []byte, ttl time.Duration) *Service {
	return &Service{repo: repo, secret: secret, ttl: ttl}
}

func (s *Service) Register(ctx context.Context, email, password string) (*User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	u := &User{
		ID:           uuid.NewString(),
		Email:        email,
		PasswordHash: string(hash),
		CreatedAt:    time.Now(),
	}
	if err := s.repo.CreateUser(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

func (s *Service) Login(ctx context.Context, email, password string) (string, *User, error) {
	u, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return "", nil, ErrInvalidCredentials
		}
		return "", nil, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return "", nil, ErrInvalidCredentials
	}
	token, err := IssueToken(s.secret, u.ID, s.ttl)
	if err != nil {
		return "", nil, err
	}
	return token, u, nil
}
```

- [ ] **Step 7: Run tests to verify they pass**

Run: `cd api && go test ./internal/auth/... -v`
Expected: PASS for all five tests.

- [ ] **Step 8: Commit**

```bash
git add api/go.mod api/go.sum api/internal/auth
git commit -m "feat: add auth domain with bcrypt hashing, JWT issuing, and register/login service"
```

---

## Task 7: Game domain — models, repository interface, service (repository-agnostic)

**Files:**
- Create: `api/internal/game/models.go`
- Create: `api/internal/game/repository.go`
- Create: `api/internal/game/errors.go`
- Create: `api/internal/game/service.go`
- Test: `api/internal/game/service_test.go`

**Interfaces:**
- Consumes: `chessrules.ApplyMove`, `chessrules.NewGameFEN`, `chessrules.Status`, `chessrules.ErrIllegalMove` (Task 4); `stockfish.Engine`, `stockfish.StubEngine` (Task 5).
- Produces:
  - `type Status string` with constants mirroring `chessrules.Status` plus `StatusResigned`.
  - `type Color string` with `ColorWhite`, `ColorBlack`.
  - `type Game struct { ID, UserID string; Status Status; PlayerColor Color; Difficulty int; CurrentFEN string; Result *string; CreatedAt, UpdatedAt time.Time }`
  - `type Move struct { ID, GameID string; PlyNumber int; MoveSAN, MoveUCI, FENAfter string; CreatedAt time.Time }`
  - `type Repository interface { CreateGame, GetGame, ListGamesByUser, ListMoves, AppendMove, UpdateGameState }` (exact signatures below)
  - `var ErrNotFound, ErrForbidden, ErrNotYourTurn, ErrGameFinished error`
  - `type MoveRequest struct { From, To, Promotion string }`
  - `type Service struct{...}`, `func NewService(repo Repository, engine stockfish.Engine) *Service`
  - `func (s *Service) CreateGame(ctx, userID string, color Color, difficulty int) (*Game, error)`
  - `func (s *Service) GetGame(ctx, userID, gameID string) (*Game, []*Move, error)`
  - `func (s *Service) ListGames(ctx, userID string, limit, offset int) ([]*Game, error)`
  - `func (s *Service) SubmitMove(ctx, userID, gameID string, req MoveRequest) (*Game, []*Move, error)`
  - `func (s *Service) Resign(ctx, userID, gameID string) (*Game, error)`

- [ ] **Step 1: Write `api/internal/game/models.go`**

```go
package game

import "time"

type Status string

const (
	StatusInProgress               Status = "in_progress"
	StatusCheckmate                Status = "checkmate"
	StatusStalemate                Status = "stalemate"
	StatusDrawRepetition           Status = "draw_repetition"
	StatusDrawFiftyMove            Status = "draw_fifty_move"
	StatusDrawInsufficientMaterial Status = "draw_insufficient_material"
	StatusResigned                 Status = "resigned"
)

type Color string

const (
	ColorWhite Color = "white"
	ColorBlack Color = "black"
)

type Game struct {
	ID          string
	UserID      string
	Status      Status
	PlayerColor Color
	Difficulty  int
	CurrentFEN  string
	Result      *string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Move struct {
	ID        string
	GameID    string
	PlyNumber int
	MoveSAN   string
	MoveUCI   string
	FENAfter  string
	CreatedAt time.Time
}
```

- [ ] **Step 2: Write `api/internal/game/repository.go`**

```go
package game

import "context"

type Repository interface {
	CreateGame(ctx context.Context, g *Game) error
	GetGame(ctx context.Context, id string) (*Game, error)
	ListGamesByUser(ctx context.Context, userID string, limit, offset int) ([]*Game, error)
	ListMoves(ctx context.Context, gameID string) ([]*Move, error)
	AppendMove(ctx context.Context, m *Move) error
	UpdateGameState(ctx context.Context, id string, fen string, status Status, result *string) error
}
```

- [ ] **Step 3: Write `api/internal/game/errors.go`**

```go
package game

import "errors"

var (
	ErrNotFound    = errors.New("game not found")
	ErrForbidden   = errors.New("not your game")
	ErrNotYourTurn = errors.New("not your turn")
	ErrGameFinished = errors.New("game is already finished")
)
```

- [ ] **Step 4: Write failing tests `api/internal/game/service_test.go`**

```go
package game

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Shyam576/chess-mind/api/internal/chessrules"
	"github.com/Shyam576/chess-mind/api/internal/stockfish"
)

type memRepo struct {
	games map[string]*Game
	moves map[string][]*Move
}

func newMemRepo() *memRepo {
	return &memRepo{games: map[string]*Game{}, moves: map[string][]*Move{}}
}

func (r *memRepo) CreateGame(ctx context.Context, g *Game) error {
	r.games[g.ID] = g
	return nil
}

func (r *memRepo) GetGame(ctx context.Context, id string) (*Game, error) {
	g, ok := r.games[id]
	if !ok {
		return nil, ErrNotFound
	}
	return g, nil
}

func (r *memRepo) ListGamesByUser(ctx context.Context, userID string, limit, offset int) ([]*Game, error) {
	var out []*Game
	for _, g := range r.games {
		if g.UserID == userID {
			out = append(out, g)
		}
	}
	return out, nil
}

func (r *memRepo) ListMoves(ctx context.Context, gameID string) ([]*Move, error) {
	return r.moves[gameID], nil
}

func (r *memRepo) AppendMove(ctx context.Context, m *Move) error {
	r.moves[m.GameID] = append(r.moves[m.GameID], m)
	return nil
}

func (r *memRepo) UpdateGameState(ctx context.Context, id string, fen string, status Status, result *string) error {
	g := r.games[id]
	g.CurrentFEN = fen
	g.Status = status
	g.Result = result
	return nil
}

func TestCreateGame(t *testing.T) {
	repo := newMemRepo()
	svc := NewService(repo, &stockfish.StubEngine{Move: "e7e5"})
	g, err := svc.CreateGame(context.Background(), "user-1", ColorWhite, 5)
	require.NoError(t, err)
	assert.Equal(t, chessrules.NewGameFEN(), g.CurrentFEN)
	assert.Equal(t, StatusInProgress, g.Status)
}

func TestSubmitMove_PlayerThenBotReply(t *testing.T) {
	repo := newMemRepo()
	svc := NewService(repo, &stockfish.StubEngine{Move: "e7e5"})
	ctx := context.Background()
	g, err := svc.CreateGame(ctx, "user-1", ColorWhite, 5)
	require.NoError(t, err)

	updated, moves, err := svc.SubmitMove(ctx, "user-1", g.ID, MoveRequest{From: "e2", To: "e4"})
	require.NoError(t, err)
	require.Len(t, moves, 2)
	assert.Equal(t, "e4", moves[0].MoveSAN)
	assert.Equal(t, "e5", moves[1].MoveSAN)
	assert.Equal(t, StatusInProgress, updated.Status)
}

func TestSubmitMove_WrongTurn(t *testing.T) {
	repo := newMemRepo()
	svc := NewService(repo, &stockfish.StubEngine{Move: "e7e5"})
	ctx := context.Background()
	g, err := svc.CreateGame(ctx, "user-1", ColorBlack, 5)
	require.NoError(t, err)

	_, _, err = svc.SubmitMove(ctx, "user-1", g.ID, MoveRequest{From: "e2", To: "e4"})
	assert.ErrorIs(t, err, ErrNotYourTurn)
}

func TestSubmitMove_NotOwner(t *testing.T) {
	repo := newMemRepo()
	svc := NewService(repo, &stockfish.StubEngine{Move: "e7e5"})
	ctx := context.Background()
	g, err := svc.CreateGame(ctx, "user-1", ColorWhite, 5)
	require.NoError(t, err)

	_, _, err = svc.SubmitMove(ctx, "someone-else", g.ID, MoveRequest{From: "e2", To: "e4"})
	assert.ErrorIs(t, err, ErrForbidden)
}

func TestSubmitMove_IllegalMove(t *testing.T) {
	repo := newMemRepo()
	svc := NewService(repo, &stockfish.StubEngine{Move: "e7e5"})
	ctx := context.Background()
	g, err := svc.CreateGame(ctx, "user-1", ColorWhite, 5)
	require.NoError(t, err)

	_, _, err = svc.SubmitMove(ctx, "user-1", g.ID, MoveRequest{From: "e2", To: "e5"})
	assert.ErrorIs(t, err, chessrules.ErrIllegalMove)
}

func TestResign(t *testing.T) {
	repo := newMemRepo()
	svc := NewService(repo, &stockfish.StubEngine{Move: "e7e5"})
	ctx := context.Background()
	g, err := svc.CreateGame(ctx, "user-1", ColorWhite, 5)
	require.NoError(t, err)

	updated, err := svc.Resign(ctx, "user-1", g.ID)
	require.NoError(t, err)
	assert.Equal(t, StatusResigned, updated.Status)

	_, _, err = svc.SubmitMove(ctx, "user-1", g.ID, MoveRequest{From: "e2", To: "e4"})
	assert.ErrorIs(t, err, ErrGameFinished)
}
```

- [ ] **Step 5: Run tests to verify they fail**

Run: `cd api && go test ./internal/game/... -v`
Expected: FAIL — `NewService` undefined.

- [ ] **Step 6: Implement `api/internal/game/service.go`**

```go
package game

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/Shyam576/chess-mind/api/internal/chessrules"
	"github.com/Shyam576/chess-mind/api/internal/stockfish"
)

type Service struct {
	repo   Repository
	engine stockfish.Engine
}

func NewService(repo Repository, engine stockfish.Engine) *Service {
	return &Service{repo: repo, engine: engine}
}

func (s *Service) CreateGame(ctx context.Context, userID string, color Color, difficulty int) (*Game, error) {
	g := &Game{
		ID:          uuid.NewString(),
		UserID:      userID,
		Status:      StatusInProgress,
		PlayerColor: color,
		Difficulty:  difficulty,
		CurrentFEN:  chessrules.NewGameFEN(),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := s.repo.CreateGame(ctx, g); err != nil {
		return nil, err
	}
	if color == ColorBlack {
		if _, _, err := s.playBotMove(ctx, g, nil); err != nil {
			return nil, err
		}
	}
	return g, nil
}

func (s *Service) GetGame(ctx context.Context, userID, gameID string) (*Game, []*Move, error) {
	g, err := s.repo.GetGame(ctx, gameID)
	if err != nil {
		return nil, nil, err
	}
	if g.UserID != userID {
		return nil, nil, ErrForbidden
	}
	moves, err := s.repo.ListMoves(ctx, gameID)
	if err != nil {
		return nil, nil, err
	}
	return g, moves, nil
}

func (s *Service) ListGames(ctx context.Context, userID string, limit, offset int) ([]*Game, error) {
	return s.repo.ListGamesByUser(ctx, userID, limit, offset)
}

type MoveRequest struct {
	From      string
	To        string
	Promotion string
}

func uciList(moves []*Move) []string {
	out := make([]string, len(moves))
	for i, m := range moves {
		out[i] = m.MoveUCI
	}
	return out
}

func plyColor(ply int) Color {
	if ply%2 == 0 {
		return ColorWhite
	}
	return ColorBlack
}

func (s *Service) SubmitMove(ctx context.Context, userID, gameID string, req MoveRequest) (*Game, []*Move, error) {
	g, err := s.repo.GetGame(ctx, gameID)
	if err != nil {
		return nil, nil, err
	}
	if g.UserID != userID {
		return nil, nil, ErrForbidden
	}
	if g.Status != StatusInProgress {
		return nil, nil, ErrGameFinished
	}

	existing, err := s.repo.ListMoves(ctx, gameID)
	if err != nil {
		return nil, nil, err
	}
	if plyColor(len(existing)) != g.PlayerColor {
		return nil, nil, ErrNotYourTurn
	}

	prior := uciList(existing)
	result, err := chessrules.ApplyMove(prior, req.From, req.To, req.Promotion)
	if err != nil {
		return nil, nil, err
	}

	playerMove := &Move{
		ID:        uuid.NewString(),
		GameID:    gameID,
		PlyNumber: len(existing),
		MoveSAN:   result.SAN,
		MoveUCI:   result.UCI,
		FENAfter:  result.FEN,
		CreatedAt: time.Now(),
	}
	if err := s.repo.AppendMove(ctx, playerMove); err != nil {
		return nil, nil, err
	}

	newStatus := Status(result.Status)
	if err := s.repo.UpdateGameState(ctx, gameID, result.FEN, newStatus, result.Result); err != nil {
		return nil, nil, err
	}
	g.CurrentFEN = result.FEN
	g.Status = newStatus
	g.Result = result.Result

	newMoves := []*Move{playerMove}
	if g.Status == StatusInProgress {
		botMove, moves, err := s.playBotMove(ctx, g, append(prior, result.UCI))
		if err != nil {
			return nil, nil, err
		}
		if botMove != nil {
			newMoves = append(newMoves, botMove)
		}
		_ = moves
	}

	return g, newMoves, nil
}

// playBotMove asks the engine for a move in the current position (priorUCI) and
// applies it, mutating g in place and persisting the move and updated game state.
// priorUCI is nil when the bot must play first (player chose black).
func (s *Service) playBotMove(ctx context.Context, g *Game, priorUCI []string) (*Move, []string, error) {
	skillLevel := g.Difficulty
	moveTime := 500 * time.Millisecond
	best, err := s.engine.BestMove(ctx, priorUCI, skillLevel, moveTime)
	if err != nil {
		return nil, nil, err
	}

	from, to, promotion := best[0:2], best[2:4], ""
	if len(best) > 4 {
		promotion = best[4:5]
	}

	result, err := chessrules.ApplyMove(priorUCI, from, to, promotion)
	if err != nil {
		return nil, nil, err
	}

	existing, err := s.repo.ListMoves(ctx, g.ID)
	if err != nil {
		return nil, nil, err
	}

	botMove := &Move{
		ID:        uuid.NewString(),
		GameID:    g.ID,
		PlyNumber: len(existing),
		MoveSAN:   result.SAN,
		MoveUCI:   result.UCI,
		FENAfter:  result.FEN,
		CreatedAt: time.Now(),
	}
	if err := s.repo.AppendMove(ctx, botMove); err != nil {
		return nil, nil, err
	}

	newStatus := Status(result.Status)
	if err := s.repo.UpdateGameState(ctx, g.ID, result.FEN, newStatus, result.Result); err != nil {
		return nil, nil, err
	}
	g.CurrentFEN = result.FEN
	g.Status = newStatus
	g.Result = result.Result

	return botMove, append(priorUCI, result.UCI), nil
}

func (s *Service) Resign(ctx context.Context, userID, gameID string) (*Game, error) {
	g, err := s.repo.GetGame(ctx, gameID)
	if err != nil {
		return nil, err
	}
	if g.UserID != userID {
		return nil, ErrForbidden
	}
	if g.Status != StatusInProgress {
		return nil, ErrGameFinished
	}
	result := "resigned"
	if err := s.repo.UpdateGameState(ctx, gameID, g.CurrentFEN, StatusResigned, &result); err != nil {
		return nil, err
	}
	g.Status = StatusResigned
	g.Result = &result
	return g, nil
}
```

Note: `chessrules.Status` values are defined as the identical string constants as `game.Status` (e.g. `"in_progress"`, `"checkmate"`) so the direct conversion `Status(result.Status)` is a safe same-underlying-string-value cast.

- [ ] **Step 7: Run tests to verify they pass**

Run: `cd api && go test ./internal/game/... -v`
Expected: PASS for all six tests.

- [ ] **Step 8: Commit**

```bash
git add api/go.mod api/go.sum api/internal/game
git commit -m "feat: add game domain service orchestrating chess rules, Stockfish, and persistence"
```

---

## Task 8: Postgres-backed repositories (`internal/db`)

**Files:**
- Create: `api/internal/db/user_repo.go`
- Create: `api/internal/db/game_repo.go`
- Test: `api/internal/db/repo_test.go`

**Interfaces:**
- Consumes: `sqlcgen.Queries` (Task 3), `auth.UserRepository`, `game.Repository` (Tasks 6, 7).
- Produces: `func NewUserRepository(q *sqlcgen.Queries) auth.UserRepository`, `func NewGameRepository(q *sqlcgen.Queries) game.Repository`.

- [ ] **Step 1: Add pgx dependency**

Run: `cd api && go get github.com/jackc/pgx/v5@v5.7.1`

- [ ] **Step 2: Implement `api/internal/db/user_repo.go`**

```go
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
	u.ID = row.ID.String()
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
		ID:           row.ID.String(),
		Email:        row.Email,
		PasswordHash: row.PasswordHash,
		CreatedAt:    row.CreatedAt.Time,
	}, nil
}
```

- [ ] **Step 3: Implement `api/internal/db/game_repo.go`**

```go
package db

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/Shyam576/chess-mind/api/internal/db/sqlcgen"
	"github.com/Shyam576/chess-mind/api/internal/game"
)

type gameRepository struct {
	q *sqlcgen.Queries
}

func NewGameRepository(q *sqlcgen.Queries) game.Repository {
	return &gameRepository{q: q}
}

func parseUUID(s string) (pgtype.UUID, error) {
	var u pgtype.UUID
	err := u.Scan(s)
	return u, err
}

func toDomainGame(row sqlcgen.Game) *game.Game {
	var result *string
	if row.Result.Valid {
		r := row.Result.String
		result = &r
	}
	return &game.Game{
		ID:          row.ID.String(),
		UserID:      row.UserID.String(),
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
	g.ID = row.ID.String()
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
			ID:        row.ID.String(),
			GameID:    row.GameID.String(),
			PlyNumber: int(row.PlyNumber),
			MoveSAN:   row.MoveSan,
			MoveUCI:   row.MoveUci,
			FENAfter:  row.FenAfter,
			CreatedAt: row.CreatedAt.Time,
		}
	}
	return out, nil
}

func (r *gameRepository) AppendMove(ctx context.Context, m *game.Move) error {
	row, err := r.q.AppendMove(ctx, sqlcgen.AppendMoveParams{
		GameID:    mustUUID(m.GameID),
		PlyNumber: int32(m.PlyNumber),
		MoveSan:   m.MoveSAN,
		MoveUci:   m.MoveUCI,
		FenAfter:  m.FENAfter,
	})
	if err != nil {
		return err
	}
	m.ID = row.ID.String()
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
```

Note: exact generated field/param names (`sqlcgen.CreateUserParams`, `sqlcgen.CreateGameParams`, column name casing like `CurrentFen`/`FenAfter`/`MoveSan`/`MoveUci`) depend on sqlc's default Go identifier generation from the snake_case SQL column names in Task 3 — after running `make sqlc-generate`, open the generated `api/internal/db/sqlcgen/models.go` and `*.sql.go` files and adjust these field names in `user_repo.go`/`game_repo.go` to match exactly what sqlc generated if they differ from the names used above.

- [ ] **Step 4: Write a repository integration test gated on an explicit test database URL (never run against the dev DB by accident)**

```go
package db

import (
	"context"
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

func TestGameRepository_CreateMoveAndUpdate(t *testing.T) {
	pool := testPool(t)
	q := sqlcgen.New(pool)
	userRepo := NewUserRepository(q)
	gameRepo := NewGameRepository(q)
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
	require.NoError(t, gameRepo.AppendMove(ctx, m))

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
```

- [ ] **Step 5: Run the integration test against local Postgres**

Run:
```bash
cd api
export TEST_DATABASE_URL=postgres://chessmind:chessmind@127.0.0.1:5434/chessmind?sslmode=disable
go build ./...
go test ./internal/db/... -v
```
Expected: `go build` succeeds (fix any field-name mismatches noted in Step 3 first); both repository tests PASS. Without `TEST_DATABASE_URL` set, `go test ./...` across the whole module still passes with these two tests reported as `SKIP`.

- [ ] **Step 6: Commit**

```bash
git add api/go.mod api/go.sum api/internal/db/user_repo.go api/internal/db/game_repo.go api/internal/db/repo_test.go
git commit -m "feat: implement Postgres-backed user and game repositories"
```

---

## Task 9: HTTP layer — router, middleware, response envelope, auth handlers

**Files:**
- Create: `api/internal/httpapi/response.go`
- Create: `api/internal/httpapi/middleware.go`
- Create: `api/internal/httpapi/router.go`
- Create: `api/internal/httpapi/auth_handlers.go`
- Test: `api/internal/httpapi/auth_handlers_test.go`

**Interfaces:**
- Consumes: `auth.Service` (Task 6), `game.Service` (Task 7).
- Produces: `func NewRouter(authSvc *auth.Service, gameSvc *game.Service, jwtSecret []byte) http.Handler`; context helper `func UserIDFromContext(ctx context.Context) (string, bool)`.

- [ ] **Step 1: Add chi dependency**

Run: `cd api && go get github.com/go-chi/chi/v5@v5.1.0`

- [ ] **Step 2: Write `api/internal/httpapi/response.go`**

```go
package httpapi

import (
	"encoding/json"
	"log"
	"net/http"
)

type errorBody struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("writeJSON encode error: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	var body errorBody
	body.Error.Code = code
	body.Error.Message = message
	writeJSON(w, status, body)
}
```

- [ ] **Step 3: Write `api/internal/httpapi/middleware.go`**

```go
package httpapi

import (
	"context"
	"net/http"
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

func recoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				writeError(w, http.StatusInternalServerError, "internal_error", "an unexpected error occurred")
			}
		}()
		next.ServeHTTP(w, r)
	})
}
```

- [ ] **Step 4: Write `api/internal/httpapi/auth_handlers.go`**

```go
package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/Shyam576/chess-mind/api/internal/auth"
)

type authHandler struct {
	svc *auth.Service
}

type registerRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *authHandler) register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "request body must be valid JSON")
		return
	}
	if req.Email == "" || len(req.Password) < 8 {
		writeError(w, http.StatusBadRequest, "invalid_input", "email is required and password must be at least 8 characters")
		return
	}
	u, err := h.svc.Register(r.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, auth.ErrUserExists) {
			writeError(w, http.StatusBadRequest, "user_exists", "an account with that email already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "could not register user")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"id": u.ID, "email": u.Email})
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *authHandler) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "request body must be valid JSON")
		return
	}
	token, u, err := h.svc.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "incorrect email or password")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    token,
		HttpOnly: true,
		Path:     "/",
		Expires:  time.Now().Add(24 * time.Hour),
		SameSite: http.SameSiteLaxMode,
	})
	writeJSON(w, http.StatusOK, map[string]string{"id": u.ID, "email": u.Email})
}

func (h *authHandler) logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		HttpOnly: true,
		Path:     "/",
		Expires:  time.Unix(0, 0),
	})
	writeJSON(w, http.StatusOK, map[string]string{"status": "logged_out"})
}
```

- [ ] **Step 5: Write `api/internal/httpapi/router.go` (game routes registered as a no-op placeholder; filled in Task 10)**

```go
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

	gh := &gameHandler{svc: gameSvc}
	r.Route("/api/games", func(r chi.Router) {
		r.Use(requireAuth(jwtSecret))
		r.Post("/", gh.create)
		r.Get("/", gh.list)
		r.Get("/{id}", gh.get)
		r.Post("/{id}/moves", gh.submitMove)
		r.Post("/{id}/resign", gh.resign)
	})

	return r
}
```

- [ ] **Step 6: Add a minimal `gameHandler` stub so the router compiles (full implementation in Task 10)**

```go
package httpapi

import "net/http"

type gameHandler struct {
	svc interface {
	}
}
```

This stub file is `api/internal/httpapi/game_handlers_stub.go` and will be **deleted** in Task 10 when the real `gameHandler` (holding `*game.Service`) is implemented — do not test against it.

- [ ] **Step 7: Write failing tests `api/internal/httpapi/auth_handlers_test.go`**

```go
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
```

- [ ] **Step 8: Run tests**

Run: `cd api && go test ./internal/httpapi/... -v`
Expected: PASS for both tests. (`gameSvc` is passed as untyped `nil` here only because Task 10 replaces the stub handler with the real one accepting `*game.Service`; these two tests never exercise game routes.)

- [ ] **Step 9: Commit**

```bash
git add api/go.mod api/go.sum api/internal/httpapi
git commit -m "feat: add HTTP router, auth middleware, and auth handlers"
```

---

## Task 10: HTTP layer — game handlers

**Files:**
- Delete: `api/internal/httpapi/game_handlers_stub.go`
- Create: `api/internal/httpapi/game_handlers.go`
- Modify: `api/internal/httpapi/router.go` (no change needed if the stub's field shape matches; see Step 1)
- Test: `api/internal/httpapi/game_handlers_test.go`

**Interfaces:**
- Consumes: `game.Service` (Task 7) — `CreateGame`, `GetGame`, `ListGames`, `SubmitMove`, `Resign`, and sentinel errors `ErrNotFound`, `ErrForbidden`, `ErrNotYourTurn`, `ErrGameFinished`; `chessrules.ErrIllegalMove` (Task 4).
- Produces: fully wired `/api/games...` routes.

- [ ] **Step 1: Delete the stub and write the real `api/internal/httpapi/game_handlers.go`**

```go
package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/Shyam576/chess-mind/api/internal/chessrules"
	"github.com/Shyam576/chess-mind/api/internal/game"
)

type gameHandler struct {
	svc *game.Service
}

type createGameRequest struct {
	Color      string `json:"color"`
	Difficulty int    `json:"difficulty"`
}

type gameResponse struct {
	ID          string  `json:"id"`
	Status      string  `json:"status"`
	PlayerColor string  `json:"player_color"`
	Difficulty  int     `json:"difficulty"`
	CurrentFEN  string  `json:"current_fen"`
	Result      *string `json:"result"`
}

type moveResponse struct {
	PlyNumber int    `json:"ply_number"`
	San       string `json:"san"`
	Uci       string `json:"uci"`
	FenAfter  string `json:"fen_after"`
}

func toGameResponse(g *game.Game) gameResponse {
	return gameResponse{
		ID:          g.ID,
		Status:      string(g.Status),
		PlayerColor: string(g.PlayerColor),
		Difficulty:  g.Difficulty,
		CurrentFEN:  g.CurrentFEN,
		Result:      g.Result,
	}
}

func toMoveResponses(moves []*game.Move) []moveResponse {
	out := make([]moveResponse, len(moves))
	for i, m := range moves {
		out[i] = moveResponse{PlyNumber: m.PlyNumber, San: m.MoveSAN, Uci: m.MoveUCI, FenAfter: m.FENAfter}
	}
	return out
}

func handleServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, game.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "game not found")
	case errors.Is(err, game.ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden", "you do not have access to this game")
	case errors.Is(err, game.ErrNotYourTurn):
		writeError(w, http.StatusConflict, "not_your_turn", "it is not your turn")
	case errors.Is(err, game.ErrGameFinished):
		writeError(w, http.StatusConflict, "game_finished", "this game has already finished")
	case errors.Is(err, chessrules.ErrIllegalMove):
		writeError(w, http.StatusBadRequest, "illegal_move", "that move is not legal in the current position")
	default:
		writeError(w, http.StatusInternalServerError, "internal_error", "an unexpected error occurred")
	}
}

func (h *gameHandler) create(w http.ResponseWriter, r *http.Request) {
	userID, _ := UserIDFromContext(r.Context())
	var req createGameRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "request body must be valid JSON")
		return
	}
	color := game.Color(req.Color)
	if color != game.ColorWhite && color != game.ColorBlack {
		writeError(w, http.StatusBadRequest, "invalid_input", "color must be \"white\" or \"black\"")
		return
	}
	if req.Difficulty < 0 || req.Difficulty > 20 {
		writeError(w, http.StatusBadRequest, "invalid_input", "difficulty must be between 0 and 20")
		return
	}
	g, err := h.svc.CreateGame(r.Context(), userID, color, req.Difficulty)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toGameResponse(g))
}

func (h *gameHandler) list(w http.ResponseWriter, r *http.Request) {
	userID, _ := UserIDFromContext(r.Context())
	limit, offset := 20, 0
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}
	games, err := h.svc.ListGames(r.Context(), userID, limit, offset)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	out := make([]gameResponse, len(games))
	for i, g := range games {
		out[i] = toGameResponse(g)
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *gameHandler) get(w http.ResponseWriter, r *http.Request) {
	userID, _ := UserIDFromContext(r.Context())
	id := chi.URLParam(r, "id")
	g, moves, err := h.svc.GetGame(r.Context(), userID, id)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"game":  toGameResponse(g),
		"moves": toMoveResponses(moves),
	})
}

type submitMoveRequest struct {
	From      string `json:"from"`
	To        string `json:"to"`
	Promotion string `json:"promotion"`
}

func (h *gameHandler) submitMove(w http.ResponseWriter, r *http.Request) {
	userID, _ := UserIDFromContext(r.Context())
	id := chi.URLParam(r, "id")
	var req submitMoveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "request body must be valid JSON")
		return
	}
	if req.From == "" || req.To == "" {
		writeError(w, http.StatusBadRequest, "invalid_input", "from and to are required")
		return
	}
	g, moves, err := h.svc.SubmitMove(r.Context(), userID, id, game.MoveRequest{From: req.From, To: req.To, Promotion: req.Promotion})
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"game":  toGameResponse(g),
		"moves": toMoveResponses(moves),
	})
}

func (h *gameHandler) resign(w http.ResponseWriter, r *http.Request) {
	userID, _ := UserIDFromContext(r.Context())
	id := chi.URLParam(r, "id")
	g, err := h.svc.Resign(r.Context(), userID, id)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toGameResponse(g))
}
```

- [ ] **Step 2: Delete the stub file**

Run: `rm api/internal/httpapi/game_handlers_stub.go`

- [ ] **Step 3: Write failing tests `api/internal/httpapi/game_handlers_test.go`**

```go
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
func (r *memGameRepo) AppendMove(ctx context.Context, m *game.Move) error {
	r.moves[m.GameID] = append(r.moves[m.GameID], m)
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
```

- [ ] **Step 4: Run tests**

Run: `cd api && go build ./... && go test ./internal/httpapi/... -v`
Expected: PASS for all tests in the package.

- [ ] **Step 5: Commit**

```bash
git add api/internal/httpapi
git commit -m "feat: implement game HTTP handlers for create/list/get/move/resign"
```

---

## Task 11: `cmd/server` wiring, config, Dockerfile, full Compose

**Files:**
- Create: `api/internal/config/config.go`
- Create: `api/cmd/server/main.go`
- Create: `api/Dockerfile`
- Modify: `docker-compose.yml` (add `api` service)

**Interfaces:**
- Consumes: everything from Tasks 2–10.
- Produces: a runnable `api` binary serving `/api/...` and `/healthz`.

- [ ] **Step 1: Write `api/internal/config/config.go`**

```go
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
```

- [ ] **Step 2: Add migration-runner and server dependencies**

Run:
```bash
cd api
go get github.com/pressly/goose/v3@v3.21.1
go get github.com/jackc/pgx/v5/stdlib@v5.7.1
```

- [ ] **Step 3: Write `api/cmd/server/main.go`**

```go
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
	gameRepo := db.NewGameRepository(queries)

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
```

- [ ] **Step 4: Write `api/Dockerfile`**

```dockerfile
FROM golang:1.23-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/server ./cmd/server

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends stockfish ca-certificates \
    && rm -rf /var/lib/apt/lists/*
COPY --from=build /out/server /usr/local/bin/server
COPY --from=build /src/internal/db/migrations /app/internal/db/migrations
WORKDIR /app
ENV STOCKFISH_PATH=/usr/games/stockfish
EXPOSE 8080
CMD ["server"]
```

- [ ] **Step 5: Add the `api` service to `docker-compose.yml`**

```yaml
version: "3.9"
services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: chessmind
      POSTGRES_PASSWORD: chessmind
      POSTGRES_DB: chessmind
    ports:
      - "127.0.0.1:5434:5432"
    volumes:
      - pgdata:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U chessmind"]
      interval: 5s
      timeout: 5s
      retries: 10

  api:
    build: ./api
    environment:
      DATABASE_URL: postgres://chessmind:chessmind@postgres:5432/chessmind?sslmode=disable
      JWT_SECRET: dev-secret-change-me
      PORT: "8080"
      STOCKFISH_POOL_SIZE: "4"
    ports:
      - "8080:8080"
    depends_on:
      postgres:
        condition: service_healthy

volumes:
  pgdata:
```

- [ ] **Step 6: Build and smoke-test**

Run:
```bash
docker compose up -d --build postgres api
sleep 3
curl -s http://localhost:8080/healthz
```
Expected: `{"status":"ok"}`.

Run a full manual smoke test of the chess loop:
```bash
curl -s -c /tmp/cookies.txt -X POST http://localhost:8080/api/auth/register \
  -H 'Content-Type: application/json' -d '{"email":"smoke@example.com","password":"hunter2hunter2"}'
curl -s -b /tmp/cookies.txt -c /tmp/cookies.txt -X POST http://localhost:8080/api/auth/login \
  -H 'Content-Type: application/json' -d '{"email":"smoke@example.com","password":"hunter2hunter2"}'
GAME_ID=$(curl -s -b /tmp/cookies.txt -X POST http://localhost:8080/api/games \
  -H 'Content-Type: application/json' -d '{"color":"white","difficulty":1}' | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")
curl -s -b /tmp/cookies.txt -X POST http://localhost:8080/api/games/$GAME_ID/moves \
  -H 'Content-Type: application/json' -d '{"from":"e2","to":"e4"}'
```
Expected: the final response's `"moves"` array has two entries (the player's `e4` and Stockfish's reply).

- [ ] **Step 7: Commit**

```bash
git add api/internal/config api/cmd api/Dockerfile docker-compose.yml api/go.mod api/go.sum
git commit -m "feat: wire server entrypoint, config, Dockerfile, and full backend compose stack"
```

---

## Task 12: Frontend scaffold — Next.js, Tailwind, API client, base layout

**Files:**
- Create: `web/package.json`, `web/tsconfig.json`, `web/next.config.mjs`, `web/tailwind.config.ts`, `web/postcss.config.js`
- Create: `web/app/layout.tsx`, `web/app/globals.css`, `web/app/page.tsx`
- Create: `web/lib/api-client.ts`
- Create: `web/components/theme-toggle.tsx`

**Interfaces:**
- Produces: `apiFetch<T>(path: string, options?: RequestInit) => Promise<T>` in `lib/api-client.ts`, throwing `ApiError` (with `status` and `code`) on non-2xx responses, used by every later frontend task.

- [ ] **Step 1: Scaffold the Next.js app**

The `web/` directory does not exist yet — run the scaffolding tool from the repo root so it creates `web/` itself; do not `cd web` first.

Run:
```bash
npx --yes create-next-app@14 web --typescript --tailwind --eslint --app --no-src-dir --import-alias "@/*" --use-npm
cd web
npm install @tanstack/react-query chess.js react-chessboard
npm install -D vitest @testing-library/react @testing-library/jest-dom jsdom @vitejs/plugin-react
```

- [ ] **Step 2: Write `web/lib/api-client.ts`**

```typescript
export class ApiError extends Error {
  status: number;
  code: string;

  constructor(status: number, code: string, message: string) {
    super(message);
    this.status = status;
    this.code = code;
  }
}

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

export async function apiFetch<T>(path: string, options: RequestInit = {}): Promise<T> {
  const res = await fetch(`${API_URL}${path}`, {
    ...options,
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
      ...options.headers,
    },
  });

  if (!res.ok) {
    let code = "unknown_error";
    let message = "Something went wrong. Please try again.";
    try {
      const body = await res.json();
      code = body.error?.code ?? code;
      message = body.error?.message ?? message;
    } catch {
      // response body was not JSON; fall back to defaults above
    }
    throw new ApiError(res.status, code, message);
  }

  if (res.status === 204) {
    return undefined as T;
  }
  return res.json() as Promise<T>;
}
```

- [ ] **Step 3: Write `web/app/layout.tsx` with a React Query provider and dark/light theme support**

```tsx
import "./globals.css";
import type { Metadata } from "next";
import { Providers } from "./providers";

export const metadata: Metadata = {
  title: "ChessMind",
  description: "Play chess against an adjustable Stockfish opponent.",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" suppressHydrationWarning>
      <body className="bg-white text-neutral-900 dark:bg-neutral-950 dark:text-neutral-100">
        <Providers>{children}</Providers>
      </body>
    </html>
  );
}
```

- [ ] **Step 4: Write `web/app/providers.tsx`**

```tsx
"use client";

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useState } from "react";

export function Providers({ children }: { children: React.ReactNode }) {
  const [client] = useState(() => new QueryClient());
  return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
}
```

- [ ] **Step 5: Write `web/components/theme-toggle.tsx`**

```tsx
"use client";

import { useEffect, useState } from "react";

export function ThemeToggle() {
  const [dark, setDark] = useState(false);

  useEffect(() => {
    const stored = localStorage.getItem("theme");
    const prefersDark = window.matchMedia("(prefers-color-scheme: dark)").matches;
    const isDark = stored ? stored === "dark" : prefersDark;
    setDark(isDark);
    document.documentElement.classList.toggle("dark", isDark);
  }, []);

  function toggle() {
    const next = !dark;
    setDark(next);
    document.documentElement.classList.toggle("dark", next);
    localStorage.setItem("theme", next ? "dark" : "light");
  }

  return (
    <button
      onClick={toggle}
      className="rounded border border-neutral-300 px-3 py-1 text-sm dark:border-neutral-700"
      aria-label="Toggle dark mode"
    >
      {dark ? "Light mode" : "Dark mode"}
    </button>
  );
}
```

- [ ] **Step 6: Write a minimal `web/app/page.tsx` that links to play/login**

```tsx
import Link from "next/link";
import { ThemeToggle } from "@/components/theme-toggle";

export default function HomePage() {
  return (
    <main className="mx-auto flex max-w-xl flex-col gap-6 p-8">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-semibold">ChessMind</h1>
        <ThemeToggle />
      </div>
      <p className="text-neutral-600 dark:text-neutral-400">
        Play chess against an adjustable Stockfish opponent.
      </p>
      <div className="flex gap-4">
        <Link href="/login" className="rounded bg-neutral-900 px-4 py-2 text-white dark:bg-neutral-100 dark:text-neutral-900">
          Log in
        </Link>
        <Link href="/register" className="rounded border border-neutral-300 px-4 py-2 dark:border-neutral-700">
          Register
        </Link>
      </div>
    </main>
  );
}
```

- [ ] **Step 7: Ensure Tailwind's `darkMode` is class-based in `web/tailwind.config.ts`**

Edit the generated file so it includes:

```typescript
import type { Config } from "tailwindcss";

const config: Config = {
  darkMode: "class",
  content: ["./app/**/*.{ts,tsx}", "./components/**/*.{ts,tsx}"],
  theme: { extend: {} },
  plugins: [],
};

export default config;
```

- [ ] **Step 8: Verify the app builds and runs**

Run: `cd web && npm run build`
Expected: build succeeds with no type errors.

- [ ] **Step 9: Commit**

```bash
git add web
git commit -m "feat: scaffold Next.js frontend with Tailwind, React Query, and API client"
```

---

## Task 13: Auth pages (register/login)

**Files:**
- Create: `web/app/register/page.tsx`
- Create: `web/app/login/page.tsx`
- Test: `web/app/login/page.test.tsx`
- Create: `web/vitest.config.ts`
- Create: `web/vitest.setup.ts`

**Interfaces:**
- Consumes: `apiFetch`, `ApiError` (Task 12).
- Produces: successful login/register redirects to `/play`.

- [ ] **Step 1: Write `web/vitest.config.ts`**

```typescript
import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";
import path from "path";

export default defineConfig({
  plugins: [react()],
  resolve: { alias: { "@": path.resolve(__dirname, ".") } },
  test: {
    environment: "jsdom",
    setupFiles: ["./vitest.setup.ts"],
    globals: true,
  },
});
```

- [ ] **Step 2: Write `web/vitest.setup.ts`**

```typescript
import "@testing-library/jest-dom/vitest";
```

- [ ] **Step 3: Add a test script to `web/package.json`**

Add under `"scripts"`: `"test": "vitest run"`.

- [ ] **Step 4: Write `web/app/login/page.tsx`**

```tsx
"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { apiFetch, ApiError } from "@/lib/api-client";

export default function LoginPage() {
  const router = useRouter();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      await apiFetch("/api/auth/login", {
        method: "POST",
        body: JSON.stringify({ email, password }),
      });
      router.push("/play");
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Something went wrong.");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <main className="mx-auto flex max-w-sm flex-col gap-4 p-8">
      <h1 className="text-xl font-semibold">Log in</h1>
      <form onSubmit={handleSubmit} className="flex flex-col gap-3">
        <label className="flex flex-col gap-1 text-sm">
          Email
          <input
            type="email"
            required
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            className="rounded border border-neutral-300 p-2 dark:border-neutral-700 dark:bg-neutral-900"
          />
        </label>
        <label className="flex flex-col gap-1 text-sm">
          Password
          <input
            type="password"
            required
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            className="rounded border border-neutral-300 p-2 dark:border-neutral-700 dark:bg-neutral-900"
          />
        </label>
        {error && (
          <p role="alert" className="text-sm text-red-600">
            {error}
          </p>
        )}
        <button
          type="submit"
          disabled={submitting}
          className="rounded bg-neutral-900 px-4 py-2 text-white disabled:opacity-50 dark:bg-neutral-100 dark:text-neutral-900"
        >
          {submitting ? "Logging in…" : "Log in"}
        </button>
      </form>
    </main>
  );
}
```

- [ ] **Step 5: Write `web/app/register/page.tsx`** (same shape as login, posting to `/api/auth/register` then redirecting to `/login`)

```tsx
"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { apiFetch, ApiError } from "@/lib/api-client";

export default function RegisterPage() {
  const router = useRouter();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      await apiFetch("/api/auth/register", {
        method: "POST",
        body: JSON.stringify({ email, password }),
      });
      router.push("/login");
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Something went wrong.");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <main className="mx-auto flex max-w-sm flex-col gap-4 p-8">
      <h1 className="text-xl font-semibold">Create an account</h1>
      <form onSubmit={handleSubmit} className="flex flex-col gap-3">
        <label className="flex flex-col gap-1 text-sm">
          Email
          <input
            type="email"
            required
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            className="rounded border border-neutral-300 p-2 dark:border-neutral-700 dark:bg-neutral-900"
          />
        </label>
        <label className="flex flex-col gap-1 text-sm">
          Password (min 8 characters)
          <input
            type="password"
            required
            minLength={8}
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            className="rounded border border-neutral-300 p-2 dark:border-neutral-700 dark:bg-neutral-900"
          />
        </label>
        {error && (
          <p role="alert" className="text-sm text-red-600">
            {error}
          </p>
        )}
        <button
          type="submit"
          disabled={submitting}
          className="rounded bg-neutral-900 px-4 py-2 text-white disabled:opacity-50 dark:bg-neutral-100 dark:text-neutral-900"
        >
          {submitting ? "Creating account…" : "Create account"}
        </button>
      </form>
    </main>
  );
}
```

- [ ] **Step 6: Write failing test `web/app/login/page.test.tsx`**

```tsx
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import LoginPage from "./page";

const pushMock = vi.fn();
vi.mock("next/navigation", () => ({ useRouter: () => ({ push: pushMock }) }));

describe("LoginPage", () => {
  beforeEach(() => {
    pushMock.mockClear();
    global.fetch = vi.fn();
  });

  it("redirects to /play on successful login", async () => {
    (global.fetch as any).mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({ id: "1", email: "a@example.com" }),
    });

    render(<LoginPage />);
    fireEvent.change(screen.getByLabelText(/email/i), { target: { value: "a@example.com" } });
    fireEvent.change(screen.getByLabelText(/password/i), { target: { value: "hunter2hunter2" } });
    fireEvent.click(screen.getByRole("button", { name: /log in/i }));

    await waitFor(() => expect(pushMock).toHaveBeenCalledWith("/play"));
  });

  it("shows an error message on invalid credentials", async () => {
    (global.fetch as any).mockResolvedValue({
      ok: false,
      status: 401,
      json: async () => ({ error: { code: "invalid_credentials", message: "Incorrect email or password" } }),
    });

    render(<LoginPage />);
    fireEvent.change(screen.getByLabelText(/email/i), { target: { value: "a@example.com" } });
    fireEvent.change(screen.getByLabelText(/password/i), { target: { value: "wrongpassword" } });
    fireEvent.click(screen.getByRole("button", { name: /log in/i }));

    expect(await screen.findByRole("alert")).toHaveTextContent("Incorrect email or password");
  });
});
```

- [ ] **Step 7: Run tests**

Run: `cd web && npm test`
Expected: both `LoginPage` tests PASS.

- [ ] **Step 8: Commit**

```bash
git add web
git commit -m "feat: add login/register pages with Vitest coverage"
```

---

## Task 14: Play setup page + game creation

**Files:**
- Create: `web/app/play/page.tsx`
- Create: `web/lib/types.ts`

**Interfaces:**
- Consumes: `apiFetch` (Task 12).
- Produces: `GameResponse` type (mirrors backend `gameResponse` JSON) reused by Tasks 15–16; creating a game navigates to `/game/[id]`.

- [ ] **Step 1: Write `web/lib/types.ts`**

```typescript
export type Color = "white" | "black";

export type GameStatus =
  | "in_progress"
  | "checkmate"
  | "stalemate"
  | "draw_repetition"
  | "draw_fifty_move"
  | "draw_insufficient_material"
  | "resigned";

export interface GameResponse {
  id: string;
  status: GameStatus;
  player_color: Color;
  difficulty: number;
  current_fen: string;
  result: string | null;
}

export interface MoveResponse {
  ply_number: number;
  san: string;
  uci: string;
  fen_after: string;
}

export interface GameWithMoves {
  game: GameResponse;
  moves: MoveResponse[];
}
```

- [ ] **Step 2: Write `web/app/play/page.tsx`**

```tsx
"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { apiFetch, ApiError } from "@/lib/api-client";
import type { Color, GameResponse } from "@/lib/types";

export default function PlaySetupPage() {
  const router = useRouter();
  const [color, setColor] = useState<Color>("white");
  const [difficulty, setDifficulty] = useState(5);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  async function startGame() {
    setError(null);
    setSubmitting(true);
    try {
      const game = await apiFetch<GameResponse>("/api/games", {
        method: "POST",
        body: JSON.stringify({ color, difficulty }),
      });
      router.push(`/game/${game.id}`);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Could not start a new game.");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <main className="mx-auto flex max-w-sm flex-col gap-6 p-8">
      <h1 className="text-xl font-semibold">New game</h1>

      <fieldset className="flex flex-col gap-2">
        <legend className="text-sm font-medium">Play as</legend>
        <div className="flex gap-3">
          {(["white", "black"] as const).map((c) => (
            <label key={c} className="flex items-center gap-1 text-sm">
              <input
                type="radio"
                name="color"
                checked={color === c}
                onChange={() => setColor(c)}
              />
              {c}
            </label>
          ))}
        </div>
      </fieldset>

      <label className="flex flex-col gap-1 text-sm">
        Difficulty: {difficulty}
        <input
          type="range"
          min={0}
          max={20}
          value={difficulty}
          onChange={(e) => setDifficulty(Number(e.target.value))}
        />
      </label>

      {error && (
        <p role="alert" className="text-sm text-red-600">
          {error}
        </p>
      )}

      <button
        onClick={startGame}
        disabled={submitting}
        className="rounded bg-neutral-900 px-4 py-2 text-white disabled:opacity-50 dark:bg-neutral-100 dark:text-neutral-900"
      >
        {submitting ? "Starting…" : "Start game"}
      </button>
    </main>
  );
}
```

- [ ] **Step 3: Manual verification**

Run `cd web && npm run dev`, with the backend running (`docker compose up -d`), register/login, navigate to `/play`, start a game, confirm redirect to `/game/<id>` (the page itself is built in Task 15, so a 404 there is expected for now).

- [ ] **Step 4: Commit**

```bash
git add web
git commit -m "feat: add new game setup page with color and difficulty selection"
```

---

## Task 15: Chessboard game page

**Files:**
- Create: `web/app/game/[id]/page.tsx`
- Create: `web/components/chessboard/GameBoard.tsx`
- Create: `web/components/chessboard/MoveHistory.tsx`
- Create: `web/components/chessboard/PromotionDialog.tsx`
- Test: `web/components/chessboard/GameBoard.test.tsx`

**Interfaces:**
- Consumes: `apiFetch`, `GameResponse`, `MoveResponse`, `GameWithMoves` (Tasks 12, 14); `chess.js`, `react-chessboard`.
- Produces: the primary gameplay surface — this is the component exercised by the Task 17 E2E test.

**Note on promotion:** `react-chessboard`'s own promotion-piece-code format varies across versions/forks and is not safe to guess. Promotion is instead detected locally with `chess.js` (`probe.moves({ square, verbose: true })`, checking the matching move's `promotion` field) and resolved with our own `PromotionDialog` component — never by parsing `react-chessboard`'s `onPieceDrop` piece argument.

- [ ] **Step 1: Write `web/components/chessboard/MoveHistory.tsx`**

```tsx
import type { MoveResponse } from "@/lib/types";

export function MoveHistory({ moves }: { moves: MoveResponse[] }) {
  const rows: { num: number; white?: string; black?: string }[] = [];
  moves.forEach((m, i) => {
    const num = Math.floor(i / 2) + 1;
    if (i % 2 === 0) {
      rows.push({ num, white: m.san });
    } else {
      rows[rows.length - 1].black = m.san;
    }
  });

  return (
    <ol className="flex flex-col gap-1 text-sm" aria-label="Move history">
      {rows.map((row) => (
        <li key={row.num} className="flex gap-2">
          <span className="w-6 text-neutral-500">{row.num}.</span>
          <span className="w-16">{row.white}</span>
          <span className="w-16">{row.black}</span>
        </li>
      ))}
    </ol>
  );
}
```

- [ ] **Step 2: Write `web/components/chessboard/PromotionDialog.tsx`**

```tsx
type PromotionPiece = "q" | "r" | "b" | "n";

const PIECES: { value: PromotionPiece; label: string }[] = [
  { value: "q", label: "Queen" },
  { value: "r", label: "Rook" },
  { value: "b", label: "Bishop" },
  { value: "n", label: "Knight" },
];

export function PromotionDialog({
  onSelect,
  onCancel,
}: {
  onSelect: (piece: PromotionPiece) => void;
  onCancel: () => void;
}) {
  return (
    <div
      className="fixed inset-0 z-10 flex items-center justify-center bg-black/40"
      role="dialog"
      aria-label="Choose promotion piece"
    >
      <div className="flex gap-2 rounded bg-white p-4 shadow-lg dark:bg-neutral-900">
        {PIECES.map((p) => (
          <button
            key={p.value}
            onClick={() => onSelect(p.value)}
            className="rounded border border-neutral-300 px-3 py-2 text-sm dark:border-neutral-700"
          >
            {p.label}
          </button>
        ))}
        <button
          onClick={onCancel}
          className="rounded border border-neutral-300 px-3 py-2 text-sm dark:border-neutral-700"
        >
          Cancel
        </button>
      </div>
    </div>
  );
}
```

- [ ] **Step 3: Write `web/components/chessboard/GameBoard.tsx`**

```tsx
"use client";

import { useMemo, useState } from "react";
import { Chess } from "chess.js";
import { Chessboard } from "react-chessboard";
import { apiFetch, ApiError } from "@/lib/api-client";
import type { GameResponse, GameWithMoves, MoveResponse } from "@/lib/types";
import { MoveHistory } from "./MoveHistory";
import { PromotionDialog } from "./PromotionDialog";

type PendingPromotion = { from: string; to: string };

export function GameBoard({ initial }: { initial: GameWithMoves }) {
  const [game, setGame] = useState<GameResponse>(initial.game);
  const [moves, setMoves] = useState<MoveResponse[]>(initial.moves);
  const [error, setError] = useState<string | null>(null);
  const [thinking, setThinking] = useState(false);
  const [pendingPromotion, setPendingPromotion] = useState<PendingPromotion | null>(null);

  const chessInstance = useMemo(() => new Chess(game.current_fen), [game.current_fen]);
  const isGameOver = game.status !== "in_progress";

  async function submitMove(from: string, to: string, promotion: string) {
    setError(null);
    setThinking(true);
    try {
      const resp = await apiFetch<{ game: GameResponse; moves: MoveResponse[] }>(
        `/api/games/${game.id}/moves`,
        {
          method: "POST",
          body: JSON.stringify({ from, to, promotion }),
        }
      );
      setGame(resp.game);
      setMoves((prev) => [...prev, ...resp.moves]);
      return true;
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Could not submit that move.");
      return false;
    } finally {
      setThinking(false);
    }
  }

  function onDrop(sourceSquare: string, targetSquare: string) {
    // chess.js is used here only to detect whether this is a promotion move
    // (it has its own verbose move list); the backend independently
    // re-validates and is authoritative for legality.
    const probe = new Chess(game.current_fen);
    const candidates = probe.moves({ square: sourceSquare, verbose: true }) as {
      to: string;
      promotion?: string;
    }[];
    const match = candidates.find((m) => m.to === targetSquare);
    if (!match) {
      return false;
    }
    if (match.promotion) {
      setPendingPromotion({ from: sourceSquare, to: targetSquare });
      return false;
    }
    void submitMove(sourceSquare, targetSquare, "");
    return true;
  }

  function choosePromotion(piece: "q" | "r" | "b" | "n") {
    if (!pendingPromotion) return;
    const { from, to } = pendingPromotion;
    setPendingPromotion(null);
    void submitMove(from, to, piece);
  }

  async function resign() {
    try {
      const updated = await apiFetch<GameResponse>(`/api/games/${game.id}/resign`, { method: "POST" });
      setGame(updated);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Could not resign.");
    }
  }

  return (
    <div className="flex flex-col gap-4 md:flex-row">
      <div className="w-full max-w-md">
        <Chessboard
          position={chessInstance.fen()}
          onPieceDrop={onDrop}
          boardOrientation={game.player_color}
          arePiecesDraggable={!isGameOver}
        />
        {pendingPromotion && (
          <PromotionDialog onSelect={choosePromotion} onCancel={() => setPendingPromotion(null)} />
        )}
        {thinking && <p className="mt-2 text-sm text-neutral-500">Stockfish is thinking…</p>}
        {error && (
          <p role="alert" className="mt-2 text-sm text-red-600">
            {error}
          </p>
        )}
        {isGameOver && (
          <p className="mt-2 font-medium">
            Game over: {game.status.replace(/_/g, " ")} {game.result ? `(${game.result})` : ""}
          </p>
        )}
        {!isGameOver && (
          <button
            onClick={resign}
            className="mt-2 rounded border border-red-300 px-3 py-1 text-sm text-red-600"
          >
            Resign
          </button>
        )}
      </div>
      <div className="w-full max-w-xs">
        <h2 className="mb-2 text-sm font-semibold">Moves</h2>
        <MoveHistory moves={moves} />
      </div>
    </div>
  );
}
```

- [ ] **Step 4: Write `web/app/game/[id]/page.tsx`** (server component fetching initial state; note: the backend session cookie must be forwarded for this server-side fetch, so this page fetches client-side instead to reuse the browser's cookie jar)

```tsx
"use client";

import { useEffect, useState } from "react";
import { useParams } from "next/navigation";
import { apiFetch, ApiError } from "@/lib/api-client";
import type { GameWithMoves } from "@/lib/types";
import { GameBoard } from "@/components/chessboard/GameBoard";

export default function GamePage() {
  const params = useParams<{ id: string }>();
  const [data, setData] = useState<GameWithMoves | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    apiFetch<GameWithMoves>(`/api/games/${params.id}`)
      .then(setData)
      .catch((err) => setError(err instanceof ApiError ? err.message : "Could not load this game."));
  }, [params.id]);

  if (error) {
    return <p role="alert" className="p-8 text-red-600">{error}</p>;
  }
  if (!data) {
    return <p className="p-8 text-neutral-500">Loading…</p>;
  }
  return (
    <main className="p-8">
      <GameBoard initial={data} />
    </main>
  );
}
```

- [ ] **Step 5: Write failing test `web/components/chessboard/GameBoard.test.tsx`**

```tsx
import { render, screen } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { GameBoard } from "./GameBoard";
import type { GameWithMoves } from "@/lib/types";

vi.mock("react-chessboard", () => ({
  Chessboard: () => <div data-testid="board" />,
}));

const initial: GameWithMoves = {
  game: {
    id: "game-1",
    status: "in_progress",
    player_color: "white",
    difficulty: 5,
    current_fen: "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1",
    result: null,
  },
  moves: [],
};

describe("GameBoard", () => {
  beforeEach(() => {
    global.fetch = vi.fn();
  });

  it("renders the board and an empty move history", () => {
    render(<GameBoard initial={initial} />);
    expect(screen.getByTestId("board")).toBeInTheDocument();
    expect(screen.getByLabelText(/move history/i)).toBeInTheDocument();
  });

  it("shows the game-over banner once the game has finished", () => {
    render(
      <GameBoard
        initial={{ ...initial, game: { ...initial.game, status: "checkmate", result: "1-0" } }}
      />
    );
    expect(screen.getByText(/game over: checkmate/i)).toBeInTheDocument();
  });
});
```

- [ ] **Step 6: Run tests**

Run: `cd web && npm test`
Expected: both `GameBoard` tests PASS.

- [ ] **Step 7: Commit**

```bash
git add web
git commit -m "feat: add chessboard game page with move submission and resign"
```

---

## Task 16: Game history page

**Files:**
- Create: `web/app/games/page.tsx`

**Interfaces:**
- Consumes: `apiFetch`, `GameResponse` (Tasks 12, 14).

- [ ] **Step 1: Write `web/app/games/page.tsx`**

```tsx
"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { apiFetch, ApiError } from "@/lib/api-client";
import type { GameResponse } from "@/lib/types";

const PAGE_SIZE = 20;

export default function GamesHistoryPage() {
  const [games, setGames] = useState<GameResponse[]>([]);
  const [page, setPage] = useState(0);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    apiFetch<GameResponse[]>(`/api/games?limit=${PAGE_SIZE}&offset=${page * PAGE_SIZE}`)
      .then(setGames)
      .catch((err) => setError(err instanceof ApiError ? err.message : "Could not load game history."));
  }, [page]);

  return (
    <main className="mx-auto max-w-2xl p-8">
      <h1 className="mb-4 text-xl font-semibold">Your games</h1>
      {error && (
        <p role="alert" className="text-sm text-red-600">
          {error}
        </p>
      )}
      {!error && games.length === 0 && (
        <p className="text-neutral-500">No games yet. <Link href="/play" className="underline">Start one</Link>.</p>
      )}
      <ul className="flex flex-col gap-2">
        {games.map((g) => (
          <li key={g.id}>
            <Link href={`/game/${g.id}`} className="flex justify-between rounded border border-neutral-200 p-3 hover:bg-neutral-50 dark:border-neutral-800 dark:hover:bg-neutral-900">
              <span>Playing {g.player_color}, difficulty {g.difficulty}</span>
              <span className="text-neutral-500">{g.status.replace(/_/g, " ")}</span>
            </Link>
          </li>
        ))}
      </ul>
      <div className="mt-4 flex gap-2">
        <button
          onClick={() => setPage((p) => Math.max(0, p - 1))}
          disabled={page === 0}
          className="rounded border border-neutral-300 px-3 py-1 text-sm disabled:opacity-50 dark:border-neutral-700"
        >
          Previous
        </button>
        <button
          onClick={() => setPage((p) => p + 1)}
          disabled={games.length < PAGE_SIZE}
          className="rounded border border-neutral-300 px-3 py-1 text-sm disabled:opacity-50 dark:border-neutral-700"
        >
          Next
        </button>
      </div>
    </main>
  );
}
```

- [ ] **Step 2: Manual verification**

Run `cd web && npm run dev` against the running backend; confirm `/games` lists games created in earlier manual tests and links to each one.

- [ ] **Step 3: Commit**

```bash
git add web
git commit -m "feat: add paginated game history page"
```

---

## Task 17: Frontend Dockerfile, full Compose wiring, Playwright E2E

**Files:**
- Create: `web/Dockerfile`
- Modify: `docker-compose.yml` (add `web` service)
- Create: `web/playwright.config.ts`
- Create: `web/e2e/golden-path.spec.ts`

**Interfaces:**
- Consumes: the full stack from Tasks 1–16.

- [ ] **Step 1: Write `web/Dockerfile`**

```dockerfile
FROM node:20-bookworm-slim AS build
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci
COPY . .
RUN npm run build

FROM node:20-bookworm-slim
WORKDIR /app
COPY --from=build /app/.next ./.next
COPY --from=build /app/public ./public
COPY --from=build /app/package.json ./package.json
COPY --from=build /app/node_modules ./node_modules
EXPOSE 3000
CMD ["npm", "run", "start"]
```

- [ ] **Step 2: Add the `web` service to `docker-compose.yml`**

```yaml
  web:
    build: ./web
    environment:
      NEXT_PUBLIC_API_URL: http://localhost:8080
    ports:
      - "3000:3000"
    depends_on:
      - api
```

(Append under the existing `services:` key alongside `postgres` and `api` from Task 11.)

- [ ] **Step 3: Install Playwright and write config**

Run: `cd web && npm install -D @playwright/test && npx playwright install --with-deps chromium`

```typescript
// web/playwright.config.ts
import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "./e2e",
  use: { baseURL: "http://localhost:3000" },
  webServer: {
    command: "npm run dev",
    url: "http://localhost:3000",
    reuseExistingServer: true,
    timeout: 30_000,
  },
});
```

- [ ] **Step 4: Write `web/e2e/golden-path.spec.ts`**

```typescript
import { test, expect } from "@playwright/test";

test("register, play a game to resignation, see it in history", async ({ page }) => {
  const email = `e2e-${Date.now()}@example.com`;
  const password = "hunter2hunter2";

  await page.goto("/register");
  await page.getByLabel(/email/i).fill(email);
  await page.getByLabel(/password/i).fill(password);
  await page.getByRole("button", { name: /create account/i }).click();
  await expect(page).toHaveURL(/\/login/);

  await page.getByLabel(/email/i).fill(email);
  await page.getByLabel(/password/i).fill(password);
  await page.getByRole("button", { name: /log in/i }).click();
  await expect(page).toHaveURL(/\/play/);

  await page.getByRole("button", { name: /start game/i }).click();
  await expect(page).toHaveURL(/\/game\//);

  await page.getByRole("button", { name: /resign/i }).click();
  await expect(page.getByText(/game over: resigned/i)).toBeVisible();

  await page.goto("/games");
  await expect(page.getByText(/resigned/i).first()).toBeVisible();
});
```

- [ ] **Step 5: Run the full stack and the E2E test**

Run:
```bash
docker compose up -d --build
cd web && npx playwright test
```
Expected: the `golden-path` test PASSES against the live backend started via Docker Compose.

- [ ] **Step 6: Commit**

```bash
git add web/Dockerfile docker-compose.yml web/playwright.config.ts web/e2e web/package.json web/package-lock.json
git commit -m "feat: add frontend Dockerfile, full compose stack, and golden-path E2E test"
```

---

## Task 18: README and final verification pass

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Write `README.md`**

```markdown
# ChessMind — Foundation

A chess application: play against an adjustable Stockfish opponent, with the Go backend authoritative over chess legality. This is the Foundation slice — no coaching features yet (see `docs/superpowers/specs/`).

## Stack

- `/web` — Next.js 14 (App Router), TypeScript, Tailwind, react-chessboard, chess.js, TanStack Query
- `/api` — Go, chi, pgx + sqlc, notnil/chess, a pooled Stockfish UCI integration
- Postgres 16

## Local development

1. Copy `.env.example` to `.env` and adjust if needed.
2. `docker compose up -d --build` — starts Postgres, the Go API (migrations run automatically on boot), and the Next.js frontend.
3. Visit `http://localhost:3000`.

## Running tests

Backend:
```bash
cd api
go test ./...                                   # unit tests only
TEST_DATABASE_URL=postgres://chessmind:chessmind@127.0.0.1:5434/chessmind?sslmode=disable \
  go test ./internal/db/... -v                   # + repository integration tests
```

Frontend:
```bash
cd web
npm test              # Vitest unit/component tests
npx playwright test   # end-to-end golden path (requires the full stack running)
```

## Environment variables

| Variable | Service | Purpose |
|---|---|---|
| `DATABASE_URL` | api | Postgres connection string |
| `JWT_SECRET` | api | HMAC secret for session JWTs |
| `PORT` | api | HTTP port (default 8080) |
| `STOCKFISH_PATH` | api | Path to the Stockfish binary |
| `STOCKFISH_POOL_SIZE` | api | Number of long-lived engine processes |
| `NEXT_PUBLIC_API_URL` | web | Base URL the frontend calls |

## Known limitations (by design, this slice)

- No AI coaching, hints, blunder interception, training modes, or learning metrics — see the full product spec and `docs/superpowers/specs/` for what's next.
- No chess clock.
- No password reset/email verification.
```

- [ ] **Step 2: Full-stack final verification**

Run (sequentially, in one shell so each `cd` carries into the next line):
```bash
cd api && go build ./... && go test ./...
cd ../web && npm run build && npm test
cd .. && docker compose up -d --build
curl -s http://localhost:8080/healthz
cd web && npx playwright test
```
Expected: Go build and tests pass; frontend build and Vitest tests pass; health check returns `{"status":"ok"}`; Playwright golden-path test passes.

- [ ] **Step 3: Commit**

```bash
git add README.md
git commit -m "docs: add README with setup, env vars, and test instructions"
git push
```
