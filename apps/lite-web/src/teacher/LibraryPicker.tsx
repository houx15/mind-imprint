import { useEffect, useState, type ReactNode } from "react";
import { getLibraryShelf, type LibraryArticle } from "../api/library";
import { errorText, filterArticles, tierLabel } from "./assignmentLogic";

/**
 * LibraryPicker — choose one article from 分级阅读库 and a level for it.
 *
 * Reads the shelf the student end uses (`getLibraryShelf`, lite-edition gated,
 * not role gated). Only the article list and each article's levels are used:
 * the shelf's recommendations and default tier are computed for the caller,
 * who here is the teacher, so they mean nothing for her students.
 *
 * `tier === null` is 按学生当前水平: the payload leaves `tier` out and the
 * server picks each student's level when she starts.
 */
export function LibraryPicker({
  slug,
  tier,
  onChange,
}: {
  slug: string;
  tier: number | null;
  onChange: (next: { slug: string; tier: number | null }) => void;
}) {
  const [articles, setArticles] = useState<LibraryArticle[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [nonce, setNonce] = useState(0);
  const [query, setQuery] = useState("");

  useEffect(() => {
    let cancelled = false;
    setArticles(null);
    setError(null);
    getLibraryShelf()
      .then((shelf) => {
        if (!cancelled) setArticles(Array.isArray(shelf?.articles) ? shelf.articles : []);
      })
      .catch((e: unknown) => {
        if (!cancelled) setError(errorText(e));
      });
    return () => {
      cancelled = true;
    };
  }, [nonce]);

  if (error) {
    return (
      <div className="text-mk-small font-semibold text-mk-danger">
        加载失败：{error}{" "}
        <button type="button" onClick={() => setNonce((n) => n + 1)} className="cursor-pointer underline">
          重试
        </button>
      </div>
    );
  }
  if (articles === null) return <div className="text-mk-small text-mk-muted">加载中…</div>;
  if (articles.length === 0) return <div className="text-mk-small text-mk-muted">暂无文章</div>;

  const shown = filterArticles(articles, query);
  const selected = articles.find((a) => a.slug === slug) ?? null;
  const levels = selected && Array.isArray(selected.levels) ? selected.levels : [];

  return (
    <div className="flex flex-col gap-3">
      <input
        type="search"
        value={query}
        onChange={(e) => setQuery(e.target.value)}
        placeholder="按标题搜索"
        aria-label="按标题搜索"
        className="w-full rounded-mk-md border border-mk-border bg-mk-surface px-3 py-2 text-mk-small text-mk-ink outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
      />
      <div className="max-h-[280px] overflow-y-auto rounded-mk-md border border-mk-border bg-mk-surface" role="listbox" aria-label="文章">
        {shown.length === 0 ? (
          <div className="px-3 py-3 text-mk-small text-mk-muted">无匹配文章</div>
        ) : (
          shown.map((a) => {
            const active = a.slug === slug;
            return (
              <button
                key={a.slug}
                type="button"
                role="option"
                aria-selected={active}
                // A different article may not have the chosen level, so a new
                // choice starts from 按学生当前水平.
                onClick={() => onChange({ slug: a.slug, tier: active ? tier : null })}
                className="block w-full border-b border-mk-border px-3 py-2.5 text-left last:border-b-0 hover:bg-mk-accent-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-mk-accent-200"
                style={active ? { background: "color-mix(in srgb, var(--mk-accent-500) 12%, var(--mk-surface))" } : undefined}
              >
                <span className="block text-mk-small font-bold text-mk-ink">{a.zhTitle || a.title}</span>
                {a.zhTitle && a.title ? <span className="block text-mk-small text-mk-muted">{a.title}</span> : null}
              </button>
            );
          })
        )}
      </div>

      {selected && (
        <div>
          <div className="text-mk-label font-bold text-mk-muted">难度</div>
          <div className="mt-1.5 flex flex-wrap gap-2">
            <LevelButton active={tier === null} onClick={() => onChange({ slug, tier: null })}>
              {tierLabel(null)}
            </LevelButton>
            {levels.map((lv) => (
              <LevelButton key={lv.tier} active={tier === lv.tier} onClick={() => onChange({ slug, tier: lv.tier })}>
                {lv.name || tierLabel(lv.tier)}
              </LevelButton>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}

function LevelButton({ active, onClick, children }: { active: boolean; onClick: () => void; children: ReactNode }) {
  return (
    <button
      type="button"
      aria-pressed={active}
      onClick={onClick}
      className={
        "rounded-mk-full border px-3 py-1.5 text-mk-small transition-colors duration-[120ms] ease-mk focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200 " +
        (active ? "border-mk-accent font-bold text-mk-accent-700" : "border-mk-border text-mk-ink hover:bg-mk-accent-50")
      }
      style={active ? { background: "color-mix(in srgb, var(--mk-accent-500) 12%, var(--mk-surface))" } : undefined}
    >
      {children}
    </button>
  );
}
