# ChessMind Foundation — Design Spec

Status: Approved
Date: 2026-11-06
Scope: First of several sub-projects decomposed from the full ChessMind product spec. This slice delivers a complete, correct, playable chess application (auth, gameplay vs. Stockfish, persistence). It intentionally excludes all coaching/learning features — those are separate future sub-projects (Socratic coach, blunder interceptor, training modes, adaptive learning/metrics, game review).

## 1. Goals

- A user can register, log in, start a game against Stockfish at an adjustable difficulty, play a fully rules-correct game (castling, en passant, promotion, check/checkmate/stalemate, draw conditions), resign, and see the game in their history.
- The backend is the authoritative source of truth for game state and chess legality — never the client alone.
- Establish the repo/dev-environment foundation (monorepo, Docker Compose, migrations, test tooling) that every later sub-project builds on.

## 2. Non-goals (explicitly deferred to later sub-projects)

- AI coaching, Socratic questioning, hint ladder.
- Blunder interceptor.
- Training modes (Guided / Assisted / Independent).
- Learning profiles, adaptive coaching, thinking-quality metrics dashboard.
- Post-game learning review (a "my games" history list is in scope; a coaching-linked review is not).
- Chess clock (nice-to-have; add only if it doesn't block completion of the above).
- Multiplayer, social features.

## 3. Repository layout

Monorepo at repo root:

```
/web                  Next.js 14 (App Router), TypeScript, Tailwind, shadcn/ui
/api                  Go backend
/api/cmd/server        main entrypoint
/api/internal/...      handlers, services, repo, chess, stockfish packages (see §6)
/api/migrations        goose SQL migrations
docker-compose.yml     web + api + postgres for local dev
docs/superpowers/specs Design specs (this file and future ones)
```

## 4. Authority model

chess.js (frontend) validates moves locally for instant UI feedback and prevents obviously illegal drag/drop interactions. It is **not** trusted as the system of record.

The Go backend independently re-validates every submitted move using a Go chess rules library (`notnil/chess`) against its own persisted game state before accepting it. This follows the project's security rule of never trusting frontend-only checks: a modified/compromised client cannot write illegal positions into saved game history.

Game flow:

1. Player makes a move on the board; chess.js gives immediate local visual feedback (does not persist anything).
2. Frontend calls `POST /api/games/:id/moves` with `{from, to, promotion?}`.
3. Backend loads the game's current state, confirms: game is `in_progress`, it is this player's turn, the move is legal per `notnil/chess`.
4. If invalid → 400/409 with a clear error; frontend reverts the optimistic UI move.
5. If valid → move persisted (append-only), game status re-evaluated (check/checkmate/stalemate/draw).
6. If the game continues and it is now the bot's turn, backend requests a move from its managed Stockfish process at the game's configured difficulty.
7. Response returns the updated authoritative state (FEN, full move list delta, status) for both the player's move and (if applicable) the bot's reply in one round trip.
8. Frontend re-renders from the server's returned state — it does not trust its own prior optimistic state as final.

A "thinking" indicator covers the round-trip. Stockfish calls are bounded by movetime/depth so the UI is never blocked indefinitely; a Stockfish failure/timeout returns a clear error rather than hanging the request.

## 5. Data model (Postgres)

Migrations managed with `goose`. Access via `sqlc`-generated code over `pgx`.

**users**
- `id` (uuid, pk)
- `email` (citext/text, unique, not null)
- `password_hash` (text, not null) — bcrypt
- `created_at` (timestamptz, default now())

**games**
- `id` (uuid, pk)
- `user_id` (uuid, fk → users, not null)
- `status` (text enum: `in_progress`, `checkmate`, `stalemate`, `draw_repetition`, `draw_fifty_move`, `draw_insufficient_material`, `draw_agreement`, `resigned`)
- `player_color` (text enum: `white`, `black`)
- `difficulty` (int — Stockfish skill level, 0-20) 
- `current_fen` (text, not null)
- `result` (text, nullable — `1-0`, `0-1`, `1/2-1/2`, null while in progress)
- `created_at`, `updated_at` (timestamptz)

**moves** (append-only — never updated or deleted)
- `id` (uuid, pk)
- `game_id` (uuid, fk → games, not null)
- `ply_number` (int, not null)
- `move_san` (text, not null)
- `move_uci` (text, not null)
- `fen_after` (text, not null)
- `created_at` (timestamptz, default now())

Indexes: `games(user_id, created_at desc)` for history listing; `moves(game_id, ply_number)` for ordered retrieval.

## 6. Backend structure (Go)

Layered per project convention (thin handlers → services hold business logic → repositories handle DB access):

- `internal/http` — chi router, handlers (thin: parse/validate request, call service, write response)
- `internal/auth` — password hashing, JWT issuing/verification, auth middleware
- `internal/game` — service layer: move submission logic, status/turn-transition rules, orchestrates chess validation + Stockfish + persistence
- `internal/chess` — thin wrapper around `notnil/chess` exposing exactly what the game service needs (legal-move check, SAN/UCI conversion, status detection)
- `internal/stockfish` — UCI process management behind an interface (`Engine`), pooled long-lived processes, configurable skill level / movetime; a fake/mock `Engine` implementation is used in tests so CI doesn't depend on a real Stockfish binary
- `internal/db` — sqlc-generated queries + repository interfaces
- `cmd/server` — wiring/composition root, config from env vars

## 7. API surface

All responses JSON. Errors use `{"error": {"code": "...", "message": "..."}}`.

| Method | Path | Purpose |
|---|---|---|
| POST | `/api/auth/register` | Create account (email, password) |
| POST | `/api/auth/login` | Issue session (JWT in httpOnly cookie) |
| POST | `/api/auth/logout` | Clear session |
| POST | `/api/games` | Create game `{color, difficulty}` |
| GET | `/api/games` | Paginated list of the caller's games |
| GET | `/api/games/:id` | Game state + full move history |
| POST | `/api/games/:id/moves` | Submit a move `{from, to, promotion?}` |
| POST | `/api/games/:id/resign` | Resign the game |

Status codes: 400 invalid input/illegal move, 401 unauthenticated, 403 not the game's owner, 404 unknown game, 409 invalid state transition (e.g. move submitted to a finished game or out of turn).

FEN/PGN export is derived client-side from data already returned by `GET /api/games/:id` — no dedicated export endpoint needed for this slice.

## 8. Frontend

Next.js App Router pages:
- `/login`, `/register`
- `/play` — new game setup (choose color, difficulty)
- `/game/[id]` — the board, move history panel, resign control
- `/games` — history list (paginated)

Components: `react-chessboard` + `chess.js` for interaction/local validation, move/last-move highlighting, promotion dialog, board-orientation flip. TanStack Query for server-state fetching/caching and mutations against the Go API. Tailwind + shadcn/ui for styling; dark/light mode.

## 9. Error handling & validation

- All external input (auth payloads, move submissions, pagination params) validated at the handler boundary before reaching services.
- Specific, actionable errors (e.g. "It is not your turn", "That move is not legal in the current position") rather than generic failures.
- Unexpected errors logged server-side with context (request id, user id, game id); stack traces never exposed to the client.
- No secrets in logs (passwords, password hashes, JWTs never logged).

## 10. Testing plan

- **Go unit tests** (testify): move legality via `internal/chess` wrapper, turn/status transition rules in `internal/game`, Stockfish integration against the mock `Engine`, auth (hashing, JWT issuing/verification).
- **Frontend unit tests** (Vitest + React Testing Library): board interaction, promotion flow, move list rendering.
- **E2E** (Playwright): one golden-path test — register → create game → play to checkmate or resign → game appears in `/games` history.

## 11. Assumptions made

- Single bundled JWT access token in an httpOnly cookie is sufficient session handling for this slice (no refresh-token rotation yet — can be added later without breaking the API contract).
- Stockfish binary is available in the `api` Docker image (installed via the container's package manager); difficulty is mapped to Stockfish's `Skill Level` UCI option (0-20) plus a bounded movetime.
- No password-reset/email-verification flow in this slice — register/login only.
- Chess clock is out of scope; if trivial to bolt on once moves/persistence exist, it may be added as a stretch item, but its absence does not block calling this slice done.

## 12. Definition of done for this slice

1. `docker-compose up` brings up web + api + postgres locally with documented setup steps.
2. A user can register, log in, start a game at a chosen difficulty and color, and play it to completion (checkmate, stalemate, any draw condition, or resignation) with fully correct rules (castling, en passant, promotion included).
3. Backend independently validates and is authoritative for every move; a tampered/invalid move submission is rejected with a clear error.
4. Completed and in-progress games persist and are visible in the user's game history.
5. Automated tests listed in §10 exist and pass.
6. README documents local dev setup (Docker Compose, env vars, running tests).
