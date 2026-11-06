# ChessMind — Foundation

A chess application: play against an adjustable Stockfish opponent, with the Go backend authoritative over chess legality. This is the Foundation slice — no coaching features yet (see `docs/superpowers/specs/`).

## Stack

- `/web` — Next.js 14 (App Router), TypeScript, Tailwind, react-chessboard, chess.js, TanStack Query
- `/api` — Go, chi, pgx + sqlc, notnil/chess, a pooled Stockfish UCI integration
- Postgres 16

## Local development

1. Copy `.env.example` to `.env` and adjust if needed.
2. `docker compose up -d --build` — starts Postgres, the Go API (migrations run automatically on boot), and the Next.js frontend.
3. Visit `http://localhost:3001`.

Host ports (`5434` for Postgres, `8081` for the API, `3001` for the frontend) are intentionally non-default — `5432`/`8080`/`3000` were already in use by unrelated processes on the machine this was built and verified on. Each service's container-internal port is unchanged (`5432`, `8080`, `3000` respectively); only the host-side mapping differs. Edit the `ports:` entries in `docker-compose.yml` if you'd prefer the conventional defaults and they're free on your machine.

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
| `CORS_ALLOWED_ORIGIN` | api | Frontend origin allowed to make credentialed cross-origin requests (default `http://localhost:3001`) |
| `NEXT_PUBLIC_API_URL` | web | Base URL the frontend calls |

## Known limitations (by design, this slice)

- No AI coaching, hints, blunder interception, training modes, or learning metrics — see the full product spec and `docs/superpowers/specs/` for what's next.
- No chess clock.
- No password reset/email verification.
