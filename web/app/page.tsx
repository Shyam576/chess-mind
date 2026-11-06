import Link from "next/link";
import { ThemeToggle } from "@/components/theme-toggle";

export default function HomePage() {
  return (
    <main className="mx-auto flex max-w-xl flex-col gap-6 p-8">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-semibold">ChessMind</h1>
        <ThemeToggle />
      </div>
      <p className="text-neutral-600 dark:text-neutral-400">
        Play chess against an adjustable Stockfish opponent.
      </p>
      <div className="flex gap-4">
        <Link href="/login" className="rounded bg-neutral-900 px-4 py-2 text-white dark:bg-neutral-100 dark:text-neutral-900">
          Log in
        </Link>
        <Link href="/register" className="rounded border border-neutral-300 px-4 py-2 dark:border-neutral-700">
          Register
        </Link>
      </div>
    </main>
  );
}
