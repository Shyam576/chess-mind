"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { apiFetch, ApiError } from "@/lib/api-client";
import type { Color, GameResponse } from "@/lib/types";
import { AppHeader } from "@/components/app-header";

export default function PlaySetupPage() {
  const router = useRouter();
  const [color, setColor] = useState<Color>("white");
  const [difficulty, setDifficulty] = useState(5);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  async function startGame() {
    setError(null);
    setSubmitting(true);
    try {
      const game = await apiFetch<GameResponse>("/api/games", {
        method: "POST",
        body: JSON.stringify({ color, difficulty }),
      });
      router.push(`/game/${game.id}`);
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        router.push("/login");
        return;
      }
      setError(err instanceof ApiError ? err.message : "Could not start a new game.");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <>
      <AppHeader />
      <main className="mx-auto flex max-w-sm flex-col gap-6 p-8">
        <h1 className="text-xl font-semibold">New game</h1>

        <fieldset className="flex flex-col gap-2">
          <legend className="text-sm font-medium">Play as</legend>
          <div className="flex gap-3">
            {(["white", "black"] as const).map((c) => (
              <label key={c} className="flex items-center gap-1 text-sm">
                <input
                  type="radio"
                  name="color"
                  checked={color === c}
                  onChange={() => setColor(c)}
                />
                {c}
              </label>
            ))}
          </div>
        </fieldset>

        <label className="flex flex-col gap-1 text-sm">
          Difficulty: {difficulty}
          <input
            type="range"
            min={0}
            max={20}
            value={difficulty}
            onChange={(e) => setDifficulty(Number(e.target.value))}
          />
        </label>

        {error && (
          <p role="alert" className="text-sm text-red-600">
            {error}
          </p>
        )}

        <button
          onClick={startGame}
          disabled={submitting}
          className="rounded bg-neutral-900 px-4 py-2 text-white disabled:opacity-50 dark:bg-neutral-100 dark:text-neutral-900"
        >
          {submitting ? "Starting…" : "Start game"}
        </button>
      </main>
    </>
  );
}
