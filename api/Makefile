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
