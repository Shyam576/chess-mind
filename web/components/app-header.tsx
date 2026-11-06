"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { apiFetch } from "@/lib/api-client";
import { ThemeToggle } from "./theme-toggle";

export function AppHeader() {
  const router = useRouter();

  async function logout() {
    try {
      await apiFetch("/api/auth/logout", { method: "POST" });
    } finally {
      router.push("/login");
    }
  }

  return (
    <header className="mx-auto flex max-w-2xl items-center justify-between p-4 text-sm">
      <nav className="flex gap-4">
        <Link href="/play" className="font-medium hover:underline">
          New game
        </Link>
        <Link href="/games" className="font-medium hover:underline">
          History
        </Link>
      </nav>
      <div className="flex items-center gap-3">
        <ThemeToggle />
        <button
          onClick={logout}
          className="rounded border border-neutral-300 px-3 py-1 dark:border-neutral-700"
        >
          Log out
        </button>
      </div>
    </header>
  );
}
