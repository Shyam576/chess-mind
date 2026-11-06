package stockfish

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type process struct {
	cmd       *exec.Cmd
	stdinPipe io.WriteCloser
	stdin     *bufio.Writer
	stdout    *bufio.Scanner
	mu        sync.Mutex
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
		cmd:       cmd,
		stdinPipe: stdin,
		stdin:     bufio.NewWriter(stdin),
		stdout:    bufio.NewScanner(stdout),
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
	// Closing stdin ends the engine's input loop even if it ignores "quit";
	// without this, cmd.Wait would block forever on such an engine.
	_ = p.stdinPipe.Close()
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
		} else {
			err = fmt.Errorf("%w (pool worker respawn also failed: %v)", err, startErr)
		}
		return "", err
	}
	p.procs <- proc
	return move, nil
}

// Close shuts down every pooled engine process. It must not be called
// concurrently with BestMove; callers should ensure in-flight requests
// have completed first (see Task 11's shutdown ordering).
func (p *Pool) Close() error {
	close(p.procs)
	for proc := range p.procs {
		_ = proc.close()
	}
	return nil
}
