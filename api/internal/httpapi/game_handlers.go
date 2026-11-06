package httpapi

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/Shyam576/chess-mind/api/internal/chessrules"
	"github.com/Shyam576/chess-mind/api/internal/game"
)

type gameHandler struct {
	svc *game.Service
}

type createGameRequest struct {
	Color      string `json:"color"`
	Difficulty int    `json:"difficulty"`
}

type gameResponse struct {
	ID          string  `json:"id"`
	Status      string  `json:"status"`
	PlayerColor string  `json:"player_color"`
	Difficulty  int     `json:"difficulty"`
	CurrentFEN  string  `json:"current_fen"`
	Result      *string `json:"result"`
}

type moveResponse struct {
	PlyNumber int    `json:"ply_number"`
	San       string `json:"san"`
	Uci       string `json:"uci"`
	FenAfter  string `json:"fen_after"`
}

func toGameResponse(g *game.Game) gameResponse {
	return gameResponse{
		ID:          g.ID,
		Status:      string(g.Status),
		PlayerColor: string(g.PlayerColor),
		Difficulty:  g.Difficulty,
		CurrentFEN:  g.CurrentFEN,
		Result:      g.Result,
	}
}

func toMoveResponses(moves []*game.Move) []moveResponse {
	out := make([]moveResponse, len(moves))
	for i, m := range moves {
		out[i] = moveResponse{PlyNumber: m.PlyNumber, San: m.MoveSAN, Uci: m.MoveUCI, FenAfter: m.FENAfter}
	}
	return out
}

func handleServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, game.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "game not found")
	case errors.Is(err, game.ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden", "you do not have access to this game")
	case errors.Is(err, game.ErrNotYourTurn):
		writeError(w, http.StatusConflict, "not_your_turn", "it is not your turn")
	case errors.Is(err, game.ErrGameFinished):
		writeError(w, http.StatusConflict, "game_finished", "this game has already finished")
	case errors.Is(err, chessrules.ErrIllegalMove):
		writeError(w, http.StatusBadRequest, "illegal_move", "that move is not legal in the current position")
	default:
		log.Printf("game handler: %v", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "an unexpected error occurred")
	}
}

func (h *gameHandler) create(w http.ResponseWriter, r *http.Request) {
	userID, _ := UserIDFromContext(r.Context())
	var req createGameRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "request body must be valid JSON")
		return
	}
	color := game.Color(req.Color)
	if color != game.ColorWhite && color != game.ColorBlack {
		writeError(w, http.StatusBadRequest, "invalid_input", "color must be \"white\" or \"black\"")
		return
	}
	if req.Difficulty < 0 || req.Difficulty > 20 {
		writeError(w, http.StatusBadRequest, "invalid_input", "difficulty must be between 0 and 20")
		return
	}
	g, err := h.svc.CreateGame(r.Context(), userID, color, req.Difficulty)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toGameResponse(g))
}

func (h *gameHandler) list(w http.ResponseWriter, r *http.Request) {
	userID, _ := UserIDFromContext(r.Context())
	limit, offset := 20, 0
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}
	games, err := h.svc.ListGames(r.Context(), userID, limit, offset)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	out := make([]gameResponse, len(games))
	for i, g := range games {
		out[i] = toGameResponse(g)
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *gameHandler) get(w http.ResponseWriter, r *http.Request) {
	userID, _ := UserIDFromContext(r.Context())
	id := chi.URLParam(r, "id")
	g, moves, err := h.svc.GetGame(r.Context(), userID, id)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"game":  toGameResponse(g),
		"moves": toMoveResponses(moves),
	})
}

type submitMoveRequest struct {
	From      string `json:"from"`
	To        string `json:"to"`
	Promotion string `json:"promotion"`
}

func (h *gameHandler) submitMove(w http.ResponseWriter, r *http.Request) {
	userID, _ := UserIDFromContext(r.Context())
	id := chi.URLParam(r, "id")
	var req submitMoveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "request body must be valid JSON")
		return
	}
	if req.From == "" || req.To == "" {
		writeError(w, http.StatusBadRequest, "invalid_input", "from and to are required")
		return
	}
	g, moves, err := h.svc.SubmitMove(r.Context(), userID, id, game.MoveRequest{From: req.From, To: req.To, Promotion: req.Promotion})
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"game":  toGameResponse(g),
		"moves": toMoveResponses(moves),
	})
}

func (h *gameHandler) resign(w http.ResponseWriter, r *http.Request) {
	userID, _ := UserIDFromContext(r.Context())
	id := chi.URLParam(r, "id")
	g, err := h.svc.Resign(r.Context(), userID, id)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toGameResponse(g))
}
