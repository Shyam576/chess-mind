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
	if last.SAN != "Qh4#" {
		t.Errorf("SAN = %q, want %q", last.SAN, "Qh4#")
	}
}

func TestApplyMove_SANIncludesCheck(t *testing.T) {
	// 1. e4 d5 2. Bb5+ checks the king directly out of the opening.
	moves := [][3]string{
		{"e2", "e4", ""},
		{"d7", "d5", ""},
		{"f1", "b5", ""},
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
	if last.SAN != "Bb5+" {
		t.Errorf("SAN = %q, want %q", last.SAN, "Bb5+")
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
		{"b5", "a6", ""}, {"b8", "c6", ""},
		{"a6", "a7", ""}, {"a8", "b8", ""},
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
