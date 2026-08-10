// PartsOverview — §4 gap G10 · a consolidated view of ALL guided parts while
// guiding (not just the current card): each part shows its title, whether it has
// content, and its status tag (green=写好了 / yellow=待完善), and clicking it jumps
// to that step. This is the "see everything I've written so far" board the doc
// wants, and doubles as free navigation across the guided cards.

export type OverviewPart = { key: string; title: string; hasText: boolean };

export function PartsOverview({
  parts,
  tags,
  currentKey,
  onJump,
}: {
  parts: OverviewPart[];
  tags: Record<string, string>;
  currentKey: string;
  onJump: (key: string) => void;
}) {
  if (parts.length === 0) return null;
  return (
    <div className="flex flex-wrap items-center gap-1.5">
      <span className="text-[12px] font-bold text-mk-faint">各部分：</span>
      {parts.map((p) => {
        const tag = tags[p.key];
        const active = p.key === currentKey;
        const dot = tag === "green" ? "bg-mk-success" : tag === "yellow" ? "bg-mk-warning" : p.hasText ? "bg-mk-accent" : "bg-mk-border";
        return (
          <button
            key={p.key}
            type="button"
            onClick={() => onJump(p.key)}
            title={tag === "green" ? "写好了" : tag === "yellow" ? "待完善" : p.hasText ? "已写" : "还没写"}
            className={`flex items-center gap-1.5 rounded-full border px-2.5 py-1 text-[12px] font-semibold ${active ? "border-mk-accent bg-mk-accent-50 text-mk-accent" : "border-mk-border bg-mk-surface text-mk-muted hover:border-mk-accent hover:text-mk-accent"}`}
          >
            <span className={`h-2 w-2 flex-none rounded-full ${dot}`} />
            <span className="max-w-[10ch] truncate">{p.title}</span>
          </button>
        );
      })}
    </div>
  );
}
