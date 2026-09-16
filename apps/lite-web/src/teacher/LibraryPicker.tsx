import { useEffect, useState, type KeyboardEvent, type ReactNode } from "react";
import { Check } from "lucide-react";
import { getLibraryShelf, type LibraryArticle } from "../api/library";
import { getClassRecommendations, type ClassRecommendations } from "../api/teacherLibrary";
import { ArticleCardBody } from "../readings/ArticleCardBody";
import {
  disciplineChips,
  errorText,
  filterArticles,
  filterByDisciplines,
  pageArticles,
  pickArticle,
  tierLabel,
} from "./assignmentLogic";
import { Chip, LABEL_CLS } from "./formParts";
import "../tree/branchHues.css";
import "../learning/landing.css";
import "./teacher.css";

/** Cards shown in the full grid before 显示全部 (divides into 2, 3 and 4 columns). */
const GRID_PAGE = 12;
const RECOMMEND_LIMIT = 8;
/** Discipline chips shown before 更多. */
const CHIP_LIMIT = 8;

/**
 * LibraryPicker — choose one article from 分级阅读库 and a level for it.
 *
 * Three parts, top to bottom:
 *   为这个班推荐  the class recommendation (GET …/classes/{id}/library/recommended),
 *                one horizontal row; hidden when `classId` is empty. A failed
 *                load is reported inside this row only.
 *   全部文章     the shelf the student end uses (`getLibraryShelf`); only its
 *                article list is read — its own recommendations and default
 *                tier are computed for the caller, who here is the teacher.
 *   难度         the level chips for the selected article.
 *
 * Cards are the student shelf's display (`ArticleCardBody`); the whole card is
 * the click target and selection is by slug, so an article selected in the
 * row is also selected in the grid.
 *
 * `tier === null` leaves `tier` out of the payload, so the server picks the
 * level when she starts — 按学生水平, unless `nullLabel` names a different
 * rule the caller wants shown for that same choice (个性化阅读's 更换 dialog
 * passes 按作业难度 when the homework has its own class-wide chip).
 *
 * The root is a CSS size container (teacher.css): the row and grid respond to
 * the width of the card they sit in, not to the viewport.
 */
export function LibraryPicker({
  classId,
  slug,
  tier,
  onChange,
  nullLabel = tierLabel(null),
}: {
  classId: string;
  slug: string;
  tier: number | null;
  onChange: (next: { slug: string; tier: number | null }) => void;
  nullLabel?: string;
}) {
  const [articles, setArticles] = useState<LibraryArticle[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [nonce, setNonce] = useState(0);
  const [query, setQuery] = useState("");
  const [disciplines, setDisciplines] = useState<string[]>([]);
  const [expanded, setExpanded] = useState(false);
  const [allChips, setAllChips] = useState(false);

  const [recs, setRecs] = useState<ClassRecommendations | null>(null);
  const [recError, setRecError] = useState<string | null>(null);
  const [recNonce, setRecNonce] = useState(0);

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

  useEffect(() => {
    setRecs(null);
    setRecError(null);
    if (!classId) return;
    let cancelled = false;
    getClassRecommendations(classId, RECOMMEND_LIMIT)
      .then((r) => {
        if (!cancelled) setRecs(r);
      })
      .catch((e: unknown) => {
        if (!cancelled) setRecError(errorText(e));
      });
    return () => {
      cancelled = true;
    };
  }, [classId, recNonce]);

  const pick = (next: string) => onChange(pickArticle({ slug, tier }, next));
  const selected =
    articles?.find((a) => a.slug === slug) ?? recs?.articles.find((a) => a.slug === slug) ?? null;
  const levels = selected && Array.isArray(selected.levels) ? selected.levels : [];

  const chips = disciplineChips(articles ?? [], { limit: CHIP_LIMIT, expanded: allChips, chosen: disciplines });
  const filtering = query.trim() !== "" || disciplines.length > 0;
  const matched = articles ? filterByDisciplines(filterArticles(articles, query), disciplines) : [];
  const shown = expanded || filtering ? matched : pageArticles(matched, GRID_PAGE, slug);
  const hiddenCount = matched.length - shown.length;

  return (
    <div className="teacher-library-picker mk-branch-hues flex flex-col gap-6">
      {classId && (
        <section className="flex flex-col gap-2.5" aria-label="为这个班推荐">
          <div className="flex flex-wrap items-baseline justify-between gap-2">
            <span className={LABEL_CLS}>为这个班推荐</span>
            {recs && recs.tier !== null && recs.articles.length > 0 && (
              <span className="text-mk-label text-mk-muted">建议难度：{tierLabel(recs.tier)}</span>
            )}
          </div>
          {recError ? (
            <div className="text-mk-small font-semibold text-mk-danger">
              推荐加载失败：{recError}{" "}
              <button type="button" onClick={() => setRecNonce((n) => n + 1)} className="cursor-pointer underline">
                重试
              </button>
            </div>
          ) : recs === null ? (
            <div className="text-mk-small text-mk-muted">加载中…</div>
          ) : recs.articles.length === 0 ? (
            <div className="text-mk-small text-mk-muted">暂无推荐</div>
          ) : (
            <div className="teacher-pick-row">
              {recs.articles.map((a) => (
                <PickCard
                  key={a.slug}
                  article={a}
                  selected={a.slug === slug}
                  onPick={() => pick(a.slug)}
                  note={
                    (a.why.length > 0 || a.readCount > 0) && (
                      <p className="flex flex-wrap gap-x-3 gap-y-0.5 text-mk-label text-mk-muted">
                        {a.why.length > 0 && <span>班级兴趣：{a.why.join("、")}</span>}
                        {a.readCount > 0 && <span>已读 {a.readCount} 人</span>}
                      </p>
                    )
                  }
                />
              ))}
            </div>
          )}
        </section>
      )}

      <section className="flex flex-col gap-3" aria-label="全部文章">
        <div className="flex flex-wrap items-baseline justify-between gap-2">
          <span className={LABEL_CLS}>全部文章</span>
          {articles && (
            <span className="text-mk-label text-mk-muted">
              {filtering ? `${matched.length} / ${articles.length} 篇` : `${articles.length} 篇`}
            </span>
          )}
        </div>

        {error ? (
          <div className="text-mk-small font-semibold text-mk-danger">
            加载失败：{error}{" "}
            <button type="button" onClick={() => setNonce((n) => n + 1)} className="cursor-pointer underline">
              重试
            </button>
          </div>
        ) : articles === null ? (
          <div className="text-mk-small text-mk-muted">加载中…</div>
        ) : articles.length === 0 ? (
          <div className="text-mk-small text-mk-muted">暂无文章</div>
        ) : (
          <>
            <input
              type="search"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder="按标题搜索"
              aria-label="按标题搜索"
              className="w-full rounded-mk-md border border-mk-border bg-mk-surface px-3 py-2 text-mk-small text-mk-ink outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
            />
            {chips.shown.length > 0 && (
              <div className="flex flex-wrap gap-2" role="group" aria-label="学科筛选">
                <Chip active={disciplines.length === 0} onClick={() => setDisciplines([])}>
                  全部学科
                </Chip>
                {chips.shown.map((t) => (
                  <Chip
                    key={t.id}
                    active={disciplines.includes(t.id)}
                    onClick={() =>
                      setDisciplines((d) => (d.includes(t.id) ? d.filter((x) => x !== t.id) : [...d, t.id]))
                    }
                  >
                    {t.zh}
                  </Chip>
                ))}
                {(chips.hidden > 0 || allChips) && (
                  <button
                    type="button"
                    aria-expanded={allChips}
                    onClick={() => setAllChips((v) => !v)}
                    className="rounded-mk-full px-3 py-1.5 text-mk-small text-mk-muted underline decoration-mk-border underline-offset-2 transition-colors duration-[120ms] ease-mk hover:text-mk-accent-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
                  >
                    {allChips ? "收起" : `更多（${chips.hidden}）`}
                  </button>
                )}
              </div>
            )}

            {shown.length === 0 ? (
              <div className="py-3 text-mk-small text-mk-muted">无匹配文章</div>
            ) : (
              <div className="teacher-pick-grid">
                {shown.map((a) => (
                  <PickCard key={a.slug} article={a} selected={a.slug === slug} onPick={() => pick(a.slug)} />
                ))}
              </div>
            )}

            {!filtering && (hiddenCount > 0 || expanded) && matched.length > GRID_PAGE && (
              <button
                type="button"
                onClick={() => setExpanded((v) => !v)}
                className="self-center rounded-mk-full border border-mk-border bg-mk-surface px-4 py-1.5 text-mk-small text-mk-secondary transition-colors duration-[120ms] ease-mk hover:border-mk-accent-200 hover:text-mk-accent-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
              >
                {expanded ? "收起" : `显示全部 ${matched.length} 篇`}
              </button>
            )}
          </>
        )}
      </section>

      {selected && (
        <section className="flex flex-col gap-2 border-t border-mk-border pt-4" aria-label="难度">
          <div className="flex flex-wrap items-baseline gap-x-2">
            <span className={LABEL_CLS}>已选</span>
            <span className="text-mk-small font-semibold text-mk-ink">{selected.zhTitle || selected.title}</span>
          </div>
          <span className={LABEL_CLS}>难度</span>
          <div className="flex flex-wrap gap-2">
            <Chip active={tier === null} onClick={() => onChange({ slug, tier: null })}>
              {nullLabel}
            </Chip>
            {levels.map((lv) => (
              <Chip key={lv.tier} active={tier === lv.tier} onClick={() => onChange({ slug, tier: lv.tier })}>
                {lv.name || tierLabel(lv.tier)}
              </Chip>
            ))}
          </div>
        </section>
      )}
    </div>
  );
}

/** One selectable article. The whole card is the control; it holds no other
 *  interactive element, so a `role="button"` wrapper stays valid. */
function PickCard({
  article,
  selected,
  onPick,
  note,
}: {
  article: LibraryArticle;
  selected: boolean;
  onPick: () => void;
  note?: ReactNode;
}) {
  const onKeyDown = (e: KeyboardEvent<HTMLElement>) => {
    if (e.key === "Enter" || e.key === " ") {
      // Space would scroll the page; Enter must not reach an enclosing form.
      e.preventDefault();
      onPick();
    }
  };
  return (
    <article
      role="button"
      tabIndex={0}
      aria-pressed={selected}
      aria-label={article.zhTitle || article.title}
      onClick={onPick}
      onKeyDown={onKeyDown}
      className={
        "teacher-pick-card reading-library-card flex cursor-pointer flex-col overflow-hidden rounded-mk-md border bg-mk-surface transition-[box-shadow,border-color] duration-[120ms] ease-mk focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200 " +
        (selected ? "border-mk-accent shadow-mk-sm" : "border-mk-border shadow-mk-xs hover:border-mk-accent-200 hover:shadow-mk-sm")
      }
      style={
        selected
          ? {
              boxShadow: "0 0 0 1px var(--mk-accent-500), var(--mk-shadow-sm)",
              background: "color-mix(in srgb, var(--mk-accent-500) 5%, var(--mk-surface))",
            }
          : undefined
      }
    >
      <ArticleCardBody
        compact
        article={article}
        note={note}
        badge={
          selected && (
            // Colours come from `.teacher-pick-badge` (teacher.css), not an
            // inline style: landing.css repaints every `span[style]` inside a
            // card in dark mode.
            <span className="teacher-pick-badge mt-0.5 flex shrink-0 items-center gap-1 rounded-mk-full border px-2 py-0.5 text-mk-label font-semibold">
              <Check size={12} aria-hidden="true" />
              已选
            </span>
          )
        }
      />
    </article>
  );
}
