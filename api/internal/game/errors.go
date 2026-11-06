package game

import "errors"

var (
	ErrNotFound     = errors.New("game not found")
	ErrForbidden    = errors.New("not your game")
	ErrNotYourTurn  = errors.New("not your turn")
	ErrGameFinished = errors.New("game is already finished")
)
