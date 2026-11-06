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

	if err := game.Move(move); err != nil {
		return MoveResult{}, ErrIllegalMove
	}

	// notnil/chess applies fivefold repetition and the 75-move rule
	// automatically, but threefold repetition and the fifty-move rule are
	// claim-only. Auto-claim them for a bot opponent — there's no one to
	// decline a draw offer against. Only do this if the game hasn't already
	// ended on its own (e.g. checkmate), since Draw() does not check that.
	if game.Outcome() == chess.NoOutcome {
		for _, method := range game.EligibleDraws() {
			if method == chess.ThreefoldRepetition || method == chess.FiftyMoveRule {
				_ = game.Draw(method)
				break
			}
		}
	}

	// Encode SAN from the move as recorded in the game's history, not the
	// bare decoded move — only the former carries the check (+) / mate (#)
	// suffix, which notnil/chess computes during game.Move.
	history := game.Moves()
	san := chess.AlgebraicNotation{}.Encode(pos, history[len(history)-1])

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
