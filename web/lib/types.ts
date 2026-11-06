export type Color = "white" | "black";

export type GameStatus =
  | "in_progress"
  | "checkmate"
  | "stalemate"
  | "draw_repetition"
  | "draw_fifty_move"
  | "draw_insufficient_material"
  | "resigned";

export interface GameResponse {
  id: string;
  status: GameStatus;
  player_color: Color;
  difficulty: number;
  current_fen: string;
  result: string | null;
}

export interface MoveResponse {
  ply_number: number;
  san: string;
  uci: string;
  fen_after: string;
}

export interface GameWithMoves {
  game: GameResponse;
  moves: MoveResponse[];
}
