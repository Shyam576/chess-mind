package stockfish

import (
	"context"
	"os"
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
	if err := os.WriteFile(path, []byte(fakeEngineScript), 0o755); err != nil {
		t.Fatalf("failed to write fake engine: %v", err)
	}
	return path
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
