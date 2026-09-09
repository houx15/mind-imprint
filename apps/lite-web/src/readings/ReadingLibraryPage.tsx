import { useEffect, useMemo, useState } from "react";
import { ArrowLeft, Search } from "lucide-react";
import { navigate, readingPath, liteRoutePath } from "../routing";
import { getLibraryShelf, startLibraryReading, type LibraryShelf } from "../api/library";
import { apiErrorText } from "../api/errorText";
import { fieldById } from "../tree/geometry";
import type { FieldId } from "../tree/types";
import { LibraryCard } from "./LibraryCard";

/**
 * ReadingLibraryPage —— 分级阅读的全部文章（`/readings/library`）。
 *
 * 落地页只摆四篇推荐；这一页是整座书架，能筛能搜。
 *
 * # 为什么筛选在前端
 *
 * 目录一次全发（二十篇的元数据，不含正文，十几 KB），所以敲一个字不打一次
 * 请求，切一根主枝也没有加载态。等库长到几百篇，这里会换成服务端筛 —— 到那
 * 时候再换，比现在先写一套用不上的分页强。
 *
 * # 三个筛子
 *
 *   搜索   中英标题、推荐语、学科名都算命中。
 *   主枝   兴趣树的七根枝，只列库里真有文章的那几根。
 *   难度   按她能读的档位筛：选「进阶」列出所有有进阶档的文章，并把卡片的默认
 *          档设成进阶。五档每篇都有，所以这个筛子筛的其实是「用哪一档打开」，
 *          它改的是默认选中项，不是文章数量 —— 这一点在标签上写清楚了。
 *
 * 🚨 最外层必须带 `mk-branch-hues`：七根主枝的颜色变量只在那个作用域里有定义
 * （见 tree/branchHues.css），少了它卡片和标签会一声不响地失去全部颜色。
 */

const TIER_NAMES = ["入门", "基础", "进阶", "高阶", "原文"];

export function ReadingLibraryPage() {
  const [shelf, setShelf] = useState<LibraryShelf | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [query, setQuery] = useState("");
  const [field, setField] = useState<string>("");
  const [tier, setTier] = useState(0); // 0 = 用服务端给的默认档
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    let cancelled = false;
    getLibraryShelf()
      .then((s) => {
        if (cancelled) return;
        setShelf(s);
        setTier(s.tier);
      })
      .catch((err) => {
        if (!cancelled) setError(apiErrorText(err));
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const visible = useMemo(() => {
    if (!shelf) return [];
    const q = query.trim().toLowerCase();
    return shelf.articles.filter((a) => {
      if (field && a.field !== field) return false;
      if (!q) return true;
      const hay = [a.zhTitle, a.title.toLowerCase(), a.reason, ...a.tags.map((t) => t.zh)];
      return hay.some((h) => h.includes(q));
    });
  }, [shelf, query, field]);

  async function start(slug: string, pick: number) {
    if (busy) return;
    setBusy(true);
    setError(null);
    try {
      navigate(readingPath(await startLibraryReading(slug, pick)));
    } catch (err) {
      setError(apiErrorText(err));
      setBusy(false);
    }
  }

  return (
    <div className="mk-branch-hues min-h-full">
      <div className="mx-auto w-full max-w-[1120px] px-4 pb-20 pt-5 sm:px-6">
        <button
          type="button"
          onClick={() => navigate(liteRoutePath({ tab: "readings" }))}
          className="flex items-center gap-1.5 text-mk-small text-mk-secondary transition-colors duration-[120ms] ease-mk hover:text-mk-accent-700"
        >
          <ArrowLeft size={15} aria-hidden="true" />
          返回阅读
        </button>

        <header className="mt-4">
          <h1 className="text-[26px] font-semibold leading-tight text-mk-ink">分级阅读</h1>
          <p className="mt-1.5 text-mk-small text-mk-secondary">
            同一篇报道有五个难度版本。挑一个话题，再挑一档你现在读得动的。
          </p>
        </header>

        <div className="mt-5 flex flex-col gap-3">
          <label className="relative block">
            <span className="sr-only">搜索文章</span>
            <Search
              size={16}
              aria-hidden="true"
              className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-mk-muted"
            />
            <input
              type="search"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder="搜索标题、话题或学科"
              className="w-full rounded-mk-full border border-mk-input-border bg-mk-surface py-2 pl-9 pr-4 text-mk-small text-mk-ink outline-none transition-colors duration-[120ms] ease-mk placeholder:text-mk-muted focus:border-mk-accent-300"
            />
          </label>

          <div className="flex flex-wrap items-center gap-1.5">
            <FilterChip label="全部学科" active={field === ""} onPick={() => setField("")} />
            {(shelf?.fields ?? []).map((f) => (
              <FilterChip
                key={f.id}
                label={f.zh}
                active={field === f.id}
                hue={fieldById(f.id as FieldId).hue}
                onPick={() => setField(field === f.id ? "" : f.id)}
              />
            ))}
          </div>

          <div className="flex flex-wrap items-center gap-1.5">
            <span className="pr-1 text-mk-label text-mk-muted">默认难度</span>
            {TIER_NAMES.map((name, i) => (
              <FilterChip key={name} label={name} active={tier === i + 1} onPick={() => setTier(i + 1)} />
            ))}
          </div>
        </div>

        {error && (
          <p role="alert" className="mt-4 text-mk-small text-mk-danger">
            {error}
          </p>
        )}

        {shelf === null && !error && (
          <p className="mt-10 text-center text-mk-small text-mk-muted">处理中</p>
        )}

        {shelf !== null && (
          <>
            <p className="mt-5 text-mk-label text-mk-muted">
              {visible.length} 篇{query.trim() || field ? "（已筛选）" : ""}
            </p>
            {visible.length === 0 ? (
              <p className="mt-10 text-center text-mk-small text-mk-secondary">
                没有匹配的文章。换一个词，或者清掉学科筛选。
              </p>
            ) : (
              <div className="mt-3 grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
                {visible.map((a) => (
                  <LibraryCard
                    key={a.slug}
                    article={a}
                    defaultTier={tier || shelf.tier}
                    busy={busy}
                    onStart={(slug, pick) => void start(slug, pick)}
                    onResume={(id) => navigate(readingPath(id))}
                  />
                ))}
              </div>
            )}
          </>
        )}
      </div>
    </div>
  );
}

function FilterChip({
  label,
  active,
  hue,
  onPick,
}: {
  label: string;
  active: boolean;
  hue?: string;
  onPick: () => void;
}) {
  const tone = hue ?? "var(--mk-accent-500)";
  return (
    <button
      type="button"
      onClick={onPick}
      aria-pressed={active}
      className="rounded-mk-full border px-3 py-1 text-mk-label transition-colors duration-[120ms] ease-mk"
      style={
        active
          ? {
              borderColor: tone,
              background: `color-mix(in srgb, ${tone} 16%, transparent)`,
              color: `color-mix(in srgb, ${tone} 74%, black)`,
            }
          : { borderColor: "var(--mk-border)", color: "var(--mk-secondary)" }
      }
    >
      {label}
    </button>
  );
}
