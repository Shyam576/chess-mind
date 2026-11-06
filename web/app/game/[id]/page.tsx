"use client";

import { useEffect, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import { apiFetch, ApiError } from "@/lib/api-client";
import type { GameWithMoves } from "@/lib/types";
import { GameBoard } from "@/components/chessboard/GameBoard";
import { AppHeader } from "@/components/app-header";

export default function GamePage() {
  const params = useParams<{ id: string }>();
  const router = useRouter();
  const [data, setData] = useState<GameWithMoves | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    apiFetch<GameWithMoves>(`/api/games/${params.id}`)
      .then(setData)
      .catch((err) => {
        if (err instanceof ApiError && err.status === 401) {
          router.push("/login");
          return;
        }
        setError(err instanceof ApiError ? err.message : "Could not load this game.");
      });
  }, [params.id, router]);

  if (error) {
    return (
      <>
        <AppHeader />
        <p role="alert" className="p-8 text-red-600">{error}</p>
      </>
    );
  }
  if (!data) {
    return (
      <>
        <AppHeader />
        <p className="p-8 text-neutral-500">Loading…</p>
      </>
    );
  }
  return (
    <>
      <AppHeader />
      <main className="p-8">
        <GameBoard initial={data} />
      </main>
    </>
  );
}
