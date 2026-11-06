"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { apiFetch, ApiError } from "@/lib/api-client";
import type { GameResponse } from "@/lib/types";
import { AppHeader } from "@/components/app-header";

const PAGE_SIZE = 20;

export default function GamesHistoryPage() {
  const router = useRouter();
  const [games, setGames] = useState<GameResponse[]>([]);
  const [page, setPage] = useState(0);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    apiFetch<GameResponse[]>(`/api/games?limit=${PAGE_SIZE}&offset=${page * PAGE_SIZE}`)
      .then((data) => {
        setError(null);
        setGames(data);
      })
      .catch((err) => {
        setGames([]);
        if (err instanceof ApiError && err.status === 401) {
          router.push("/login");
          return;
        }
        setError(err instanceof ApiError ? err.message : "Could not load game history.");
      });
  }, [page, router]);

  return (
    <>
      <AppHeader />
      <main className="mx-auto max-w-2xl p-8">
        <h1 className="mb-4 text-xl font-semibold">Your games</h1>
        {error && (
          <p role="alert" className="text-sm text-red-600">
            {error}
          </p>
        )}
        {!error && games.length === 0 && (
          <p className="text-neutral-500">No games yet. <Link href="/play" className="underline">Start one</Link>.</p>
        )}
        <ul className="flex flex-col gap-2">
          {games.map((g) => (
            <li key={g.id}>
              <Link href={`/game/${g.id}`} className="flex justify-between rounded border border-neutral-200 p-3 hover:bg-neutral-50 dark:border-neutral-800 dark:hover:bg-neutral-900">
                <span>Playing {g.player_color}, difficulty {g.difficulty}</span>
                <span className="text-neutral-500">{g.status.replace(/_/g, " ")}</span>
              </Link>
            </li>
          ))}
        </ul>
        <div className="mt-4 flex gap-2">
          <button
            onClick={() => setPage((p) => Math.max(0, p - 1))}
            disabled={page === 0}
            className="rounded border border-neutral-300 px-3 py-1 text-sm disabled:opacity-50 dark:border-neutral-700"
          >
            Previous
          </button>
          <button
            onClick={() => setPage((p) => p + 1)}
            disabled={games.length < PAGE_SIZE}
            className="rounded border border-neutral-300 px-3 py-1 text-sm disabled:opacity-50 dark:border-neutral-700"
          >
            Next
          </button>
        </div>
      </main>
    </>
  );
}
