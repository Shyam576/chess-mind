"use client";

import { useMemo, useState } from "react";
import { Chess } from "chess.js";
import type { Square } from "chess.js";
import { Chessboard } from "react-chessboard";
import { apiFetch, ApiError } from "@/lib/api-client";
import type { GameResponse, GameWithMoves, MoveResponse } from "@/lib/types";
import { MoveHistory } from "./MoveHistory";
import { PromotionDialog } from "./PromotionDialog";

type PendingPromotion = { from: string; to: string };

export function GameBoard({ initial }: { initial: GameWithMoves }) {
  const [game, setGame] = useState<GameResponse>(initial.game);
  const [moves, setMoves] = useState<MoveResponse[]>(initial.moves);
  const [error, setError] = useState<string | null>(null);
  const [thinking, setThinking] = useState(false);
  const [pendingPromotion, setPendingPromotion] = useState<PendingPromotion | null>(null);

  const chessInstance = useMemo(() => new Chess(game.current_fen), [game.current_fen]);
  const isGameOver = game.status !== "in_progress";

  async function submitMove(from: string, to: string, promotion: string) {
    setError(null);
    setThinking(true);
    try {
      const resp = await apiFetch<{ game: GameResponse; moves: MoveResponse[] }>(
        `/api/games/${game.id}/moves`,
        {
          method: "POST",
          body: JSON.stringify({ from, to, promotion }),
        }
      );
      setGame(resp.game);
      setMoves((prev) => [...prev, ...resp.moves]);
      return true;
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Could not submit that move.");
      return false;
    } finally {
      setThinking(false);
    }
  }

  function onDrop(sourceSquare: string, targetSquare: string) {
    // chess.js is used here only to detect whether this is a promotion move
    // (it has its own verbose move list); the backend independently
    // re-validates and is authoritative for legality.
    const probe = new Chess(game.current_fen);
    const candidates = probe.moves({ square: sourceSquare as Square, verbose: true });
    const match = candidates.find((m) => m.to === targetSquare);
    if (!match) {
      return false;
    }
    if (match.promotion) {
      setPendingPromotion({ from: sourceSquare, to: targetSquare });
      return false;
    }
    void submitMove(sourceSquare, targetSquare, "");
    return true;
  }

  function choosePromotion(piece: "q" | "r" | "b" | "n") {
    if (!pendingPromotion) return;
    const { from, to } = pendingPromotion;
    setPendingPromotion(null);
    void submitMove(from, to, piece);
  }

  async function resign() {
    try {
      const updated = await apiFetch<GameResponse>(`/api/games/${game.id}/resign`, { method: "POST" });
      setGame(updated);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Could not resign.");
    }
  }

  return (
    <div className="flex flex-col gap-4 md:flex-row">
      <div className="w-full max-w-md">
        <Chessboard
          position={chessInstance.fen()}
          onPieceDrop={onDrop}
          boardOrientation={game.player_color}
          arePiecesDraggable={!isGameOver && !thinking}
          showPromotionDialog={false}
        />
        {pendingPromotion && (
          <PromotionDialog onSelect={choosePromotion} onCancel={() => setPendingPromotion(null)} />
        )}
        {thinking && <p className="mt-2 text-sm text-neutral-500">Stockfish is thinking…</p>}
        {error && (
          <p role="alert" className="mt-2 text-sm text-red-600">
            {error}
          </p>
        )}
        {isGameOver && (
          <p className="mt-2 font-medium">
            Game over: {game.status.replace(/_/g, " ")} {game.result ? `(${game.result})` : ""}
          </p>
        )}
        {!isGameOver && (
          <button
            onClick={resign}
            className="mt-2 rounded border border-red-300 px-3 py-1 text-sm text-red-600"
          >
            Resign
          </button>
        )}
      </div>
      <div className="w-full max-w-xs">
        <h2 className="mb-2 text-sm font-semibold">Moves</h2>
        <MoveHistory moves={moves} />
      </div>
    </div>
  );
}
