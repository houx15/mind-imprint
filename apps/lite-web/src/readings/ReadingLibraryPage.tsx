import { LandingHeader } from "../learning/LandingHeader";
import { useEffect, useMemo, useState } from "react";
import { Search } from "lucide-react";
import { navigate, readingPath } from "../routing";
import { getLibraryShelf, startLibraryReading, type LibraryShelf } from "../api/library";
import { apiErrorText } from "../api/errorText";
import { fieldById } from "../tree/geometry";
import type { FieldId } from "../tree/types";
import { Pagination } from "../learning/Pagination";
import { LibraryCard } from "./LibraryCard";
import { ReadingsTabs } from "./ReadingsTabs";

/**
 * ReadingLibraryPage —— 分级阅读的全部文章（`/readings/library`）。
 *
 * 落地页只摆四篇推荐；这一页是整座书架，能筛能搜。
 *
 * # 为什么筛选在前端
 *
 * 目录一次全发（四十多篇的元数据，不含正文，几十 KB），所以敲一个字不打一次
 * 请求，切一根主枝也没有加载态。
 *
 * 🚨 这段注释原来写着「等库长到几百篇，这里会换成服务端筛」。那一天还没到
 * （48 篇），但**翻页那一天到了**：四十多张卡一屏滚不完。所以分页放在前端，
 * 和筛选同一侧 —— 数据本来就全在手上，服务端翻页反而要多打一次请求。
 * 写作题库那边是 705 道，筛搜翻全在服务端，两边的选择不一样是**因为量级不一样**，
 * 不是因为两个人写的。
 *
 * # 三个筛子
 *
 *   搜索   中英标题、推荐语、学科名都算命中。
 *   主枝   兴趣树的七根枝，只列库里真有文章的那几根。
 *   难度   每篇都有全部五档，所以这一排改的不是有几篇，而是「点开就用哪一档」。
 *          标签因此叫「默认难度」而不是「难度」。她手上还开着的那几篇不跟着
 *          动 —— 卡片停在她自己打开的那一档（见 LibraryCard）。
 *
 * 🚨 最外层必须带 `mk-branch-hues`：七根主枝的颜色变量只在那个作用域里有定义
 * （见 tree/branchHues.css），少了它卡片和标签会一声不响地失去全部颜色。
 */

const TIER_NAMES = ["入门", "基础", "进阶", "高阶", "原文"];

/** 一页几篇。12：三列整四行，两列六行，手机一列也翻得动。 */
const LIBRARY_PAGE_SIZE = 12;

export function ReadingLibraryPage() {
  const [shelf, setShelf] = useState<LibraryShelf | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [query, setQuery] = useState("");
  const [field, setField] = useState<string>("");
  const [tier, setTier] = useState(0); // 0 = 用服务端给的默认档
  const [busy, setBusy] = useState(false);
  const [page, setPage] = useState(1);

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

  // 🚨 改了筛选就回到第 1 页。她在第 3 页选了一根主枝、结果只有 1 页的话，
  // 不回第 1 页她看到的是一片空白，读起来像「这根枝下面没有文章」。
  useEffect(() => {
    setPage(1);
  }, [query, field]);

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

  const pages = Math.max(1, Math.ceil(visible.length / LIBRARY_PAGE_SIZE));
  const shown = visible.slice((page - 1) * LIBRARY_PAGE_SIZE, page * LIBRARY_PAGE_SIZE);

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
    <div className="learning-landing learning-library mk-branch-hues min-h-full">
      <div className="learning-landing-measure learning-landing-wide mx-auto w-full">
        <div className="flex justify-center">
          <ReadingsTabs active="library" libraryCount={shelf?.articles.length} />
        </div>

        <LandingHeader kind="reading" title="分级阅读" description="每篇报道提供五个难度版本，可按学科和阅读难度选择。" />

        <div className="reading-library-filters flex flex-col gap-3">
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
            <span className="reading-filter-label">学科</span>
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
            <span className="reading-filter-label">阅读难度</span>
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
              {pages > 1 ? ` · 第 ${page} / ${pages} 页` : ""}
            </p>
            {visible.length === 0 ? (
              <p className="mt-10 text-center text-mk-small text-mk-secondary">
                没有匹配的文章。换一个词，或者清掉学科筛选。
              </p>
            ) : (
              <div className="mt-3 grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
                {shown.map((a) => (
                  <LibraryCard
                    // 🚨 key 带上默认档：卡片的选中档是 useState 的初始值，
                    // 只在挂载时读一次。不换 key 的话，切「默认难度」这一排
                    // 按钮会亮，卡片上的字却一个都不动。
                    key={`${a.slug}:${tier || shelf.tier}`}
                    article={a}
                    defaultTier={tier || shelf.tier}
                    busy={busy}
                    onStart={(slug, pick) => void start(slug, pick)}
                    onResume={(id) => navigate(readingPath(id))}
                  />
                ))}
              </div>
            )}
            <Pagination
              page={page}
              pages={pages}
              onPick={(n) => {
                setPage(n);
                window.scrollTo({ top: 0, behavior: "smooth" });
              }}
            />
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
