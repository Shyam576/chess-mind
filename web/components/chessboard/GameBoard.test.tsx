import { render, screen } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { GameBoard } from "./GameBoard";
import type { GameWithMoves } from "@/lib/types";

vi.mock("react-chessboard", () => ({
  Chessboard: () => <div data-testid="board" />,
}));

const initial: GameWithMoves = {
  game: {
    id: "game-1",
    status: "in_progress",
    player_color: "white",
    difficulty: 5,
    current_fen: "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1",
    result: null,
  },
  moves: [],
};

describe("GameBoard", () => {
  beforeEach(() => {
    global.fetch = vi.fn();
  });

  it("renders the board and an empty move history", () => {
    render(<GameBoard initial={initial} />);
    expect(screen.getByTestId("board")).toBeInTheDocument();
    expect(screen.getByLabelText(/move history/i)).toBeInTheDocument();
  });

  it("shows the game-over banner once the game has finished", () => {
    render(
      <GameBoard
        initial={{ ...initial, game: { ...initial.game, status: "checkmate", result: "1-0" } }}
      />
    );
    expect(screen.getByText(/game over: checkmate/i)).toBeInTheDocument();
  });
});
