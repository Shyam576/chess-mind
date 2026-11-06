import type { MoveResponse } from "@/lib/types";

export function MoveHistory({ moves }: { moves: MoveResponse[] }) {
  const rows: { num: number; white?: string; black?: string }[] = [];
  moves.forEach((m, i) => {
    const num = Math.floor(i / 2) + 1;
    if (i % 2 === 0) {
      rows.push({ num, white: m.san });
    } else {
      rows[rows.length - 1].black = m.san;
    }
  });

  return (
    <ol className="flex flex-col gap-1 text-sm" aria-label="Move history">
      {rows.map((row) => (
        <li key={row.num} className="flex gap-2">
          <span className="w-6 text-neutral-500">{row.num}.</span>
          <span className="w-16">{row.white}</span>
          <span className="w-16">{row.black}</span>
        </li>
      ))}
    </ol>
  );
}
