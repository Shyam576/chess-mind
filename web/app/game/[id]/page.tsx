"use client";

import { useEffect, useState } from "react";
import { useParams } from "next/navigation";
import { apiFetch, ApiError } from "@/lib/api-client";
import type { GameWithMoves } from "@/lib/types";
import { GameBoard } from "@/components/chessboard/GameBoard";

export default function GamePage() {
  const params = useParams<{ id: string }>();
  const [data, setData] = useState<GameWithMoves | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    apiFetch<GameWithMoves>(`/api/games/${params.id}`)
      .then(setData)
      .catch((err) => setError(err instanceof ApiError ? err.message : "Could not load this game."));
  }, [params.id]);

  if (error) {
    return <p role="alert" className="p-8 text-red-600">{error}</p>;
  }
  if (!data) {
    return <p className="p-8 text-neutral-500">Loading…</p>;
  }
  return (
    <main className="p-8">
      <GameBoard initial={data} />
    </main>
  );
}
