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
