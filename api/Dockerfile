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
