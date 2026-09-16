// FinishedTabs — 完成页上的那几格。
//
// 切的是同一页里的几屏，**不进 URL**：`/readings/:id` 已经是一条会被分享、
// 会被教师端引用的深链接，多一段 `?view=` 会让「同一条链接对不同人打开不同屏」
// 变成新的一类 bug，而这几屏之间的切换没有需要被链接的价值。
//
// 标签是名词（`报告` / `对话` / `原文`），不是句子，也不是印记在跟她说话 ——
// 界面负责给东西命名（AGENTS.md § 界面文案怎么写 · 规则 0 与 1）。
export type FinishedTab = "report" | "transcript" | "source";

export function FinishedTabs({
  tabs,
  active,
  onPick,
}: {
  tabs: { id: FinishedTab; label: string }[];
  active: FinishedTab;
  onPick: (id: FinishedTab) => void;
}) {
  return (
    <div role="tablist" className="inline-flex gap-1 rounded-mk-full border border-mk-border bg-mk-surface p-1">
      {tabs.map((t) => {
        const on = t.id === active;
        return (
          <button
            key={t.id}
            role="tab"
            type="button"
            aria-selected={on}
            onClick={() => onPick(t.id)}
            className="rounded-mk-full px-4 py-1.5 text-mk-small transition-colors duration-[120ms] ease-mk"
            style={
              on
                ? { background: "var(--mk-accent-100)", color: "var(--mk-accent-700)" }
                : { color: "var(--mk-muted)" }
            }
          >
            {t.label}
          </button>
        );
      })}
    </div>
  );
}
