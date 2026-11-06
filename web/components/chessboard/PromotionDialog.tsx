type PromotionPiece = "q" | "r" | "b" | "n";

const PIECES: { value: PromotionPiece; label: string }[] = [
  { value: "q", label: "Queen" },
  { value: "r", label: "Rook" },
  { value: "b", label: "Bishop" },
  { value: "n", label: "Knight" },
];

export function PromotionDialog({
  onSelect,
  onCancel,
}: {
  onSelect: (piece: PromotionPiece) => void;
  onCancel: () => void;
}) {
  return (
    <div
      className="fixed inset-0 z-10 flex items-center justify-center bg-black/40"
      role="dialog"
      aria-label="Choose promotion piece"
    >
      <div className="flex gap-2 rounded bg-white p-4 shadow-lg dark:bg-neutral-900">
        {PIECES.map((p) => (
          <button
            key={p.value}
            onClick={() => onSelect(p.value)}
            className="rounded border border-neutral-300 px-3 py-2 text-sm dark:border-neutral-700"
          >
            {p.label}
          </button>
        ))}
        <button
          onClick={onCancel}
          className="rounded border border-neutral-300 px-3 py-2 text-sm dark:border-neutral-700"
        >
          Cancel
        </button>
      </div>
    </div>
  );
}
