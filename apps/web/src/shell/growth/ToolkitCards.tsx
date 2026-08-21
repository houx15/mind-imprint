import { useEffect, useMemo, useRef, useState } from "react";
import { COVER_THEMES, type CardCatalogEntry, type CoverTheme, type CourseSummary } from "@mind-imprint/contracts";
import { api } from "@/api";
import { Skeleton, type MacaronName, coverGradientStyle } from "@/ui";
import { CardDetailModal, Stars } from "./CardDetailModal";

/** Join truthy class fragments with a single space; drops falsy/empty ones. */
function cx(...parts: Array<string | false | null | undefined>): string {
  return parts.filter(Boolean).join(" ");
}

// Fixed display order of the card categories (roughly the course-library arc),
// each cycling through the 7 macaron tokens for its group dot. Any category
// not listed falls into a final 其他 group (rendered with the neutral token).
const MACARON_ORDER: MacaronName[] = ["peach", "butter", "matcha", "lake", "mist", "taro", "berry"];
const MACARON_DOT: Record<MacaronName, string> = {
  peach: "bg-mk-peach", butter: "bg-mk-butter", matcha: "bg-mk-matcha",
  lake: "bg-mk-lake", mist: "bg-mk-mist", taro: "bg-mk-taro", berry: "bg-mk-berry",
};
const CATEGORY_KEYS = [
  "探究启动", "信息素养", "溯源与多视角", "知识工具", "论证结构",
  "AOK", "AI伦理", "反身性与元认知", "成长与沉淀", "学科透镜",
];
const CATEGORY_ORDER: { key: string; dot: string }[] = CATEGORY_KEYS.map((key, i) => ({
  key,
  dot: MACARON_DOT[MACARON_ORDER[i % MACARON_ORDER.length]!],
}));
const OTHER = { key: "其他", dot: "bg-mk-faint" };

// The four cover colorways, with a representative swatch color for the picker.
// This is a distinct feature from the UI accent (Settings) — it only controls
// card cover art — so it keeps its own fixed swatch tokens rather than
// following the accent preset.
// Keys are historical; the v2 cover art's four colorways are white/black/green/
// blue, so "cyber-warm" now surfaces the BLUE variant (there is no warm art) and
// is labelled 湖蓝. The key stays put to avoid a DB/enum migration.
const THEME_META: Record<CoverTheme, { label: string; swatch: string }> = {
  light: { label: "浅色", swatch: "bg-mk-paper" },
  "cyber-sage": { label: "青绿", swatch: "bg-mk-matcha" },
  "cyber-slate": { label: "石板", swatch: "bg-mk-ink" },
  "cyber-warm": { label: "湖蓝", swatch: "bg-mk-mist" },
};

// Status segmented control (所有卡片 / 已练习过).
function SegButton({ active, onClick, children }: { active: boolean; onClick: () => void; children: React.ReactNode }) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-pressed={active}
      className={cx(
        "rounded-mk-full px-3.5 py-1.5 text-mk-small font-bold transition-colors duration-[120ms] ease-mk",
        active ? "bg-mk-accent-500 text-white" : "text-mk-muted hover:text-mk-secondary",
      )}
    >
      {children}
    </button>
  );
}

// Category tag chip (with the group's dot).
function TagChip({ active, onClick, dot, children }: { active: boolean; onClick: () => void; dot?: string; children: React.ReactNode }) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-pressed={active}
      className={cx(
        "inline-flex items-center gap-1.5 rounded-mk-full border px-3 py-1.5 text-mk-small font-semibold transition-colors duration-[120ms] ease-mk",
        active ? "border-mk-accent-500 bg-mk-accent-50 text-mk-accent-600" : "border-mk-border bg-mk-surface text-mk-secondary hover:border-mk-accent-300",
      )}
    >
      {dot && <span className={cx("h-[7px] w-[7px] shrink-0 rounded-mk-full", dot)} />}
      {children}
    </button>
  );
}

function CardTile({ c, onOpen }: { c: CardCatalogEntry; onOpen: () => void }) {
  const [hover, setHover] = useState(false);
  const [imgFailed, setImgFailed] = useState(false);
  const encountered = c.encountered;
  const showImg = Boolean(c.coverUrl) && !imgFailed;
  return (
    <button
      type="button"
      onClick={onOpen}
      onMouseEnter={() => setHover(true)}
      onMouseLeave={() => setHover(false)}
      title={c.name}
      className={cx(
        "relative block aspect-[2/3] w-full overflow-hidden rounded-mk-md border border-mk-border bg-mk-surface p-0 text-left",
        "transition-shadow duration-[150ms] ease-mk",
        hover ? "shadow-mk-md" : "shadow-mk-xs",
      )}
    >
      {/* cover: image if we have one (and it loaded), else a text face */}
      {showImg ? (
        <img
          src={c.coverUrl}
          alt={c.name}
          loading="lazy"
          onError={() => setImgFailed(true)}
          className={cx(
            // Un-encountered cards keep the colorway (so the 封面配色 switch is
            // visible everywhere) but read as "not yet earned" — softly dimmed
            // and lightly desaturated, never fully grey, so a mostly-unpracticed
            // gallery still recolors when the theme changes.
            "absolute inset-0 h-full w-full object-cover transition-[filter] duration-200",
            encountered ? "grayscale-0" : "opacity-[.6] grayscale-[.35] contrast-[.95]",
          )}
        />
      ) : (
        <div
          className={cx(
            "absolute inset-0 flex items-center justify-center p-3.5",
            !encountered && "opacity-[.6] grayscale-[.35]",
          )}
          style={coverGradientStyle(c.cardId)}
        >
          <span className="text-center text-mk-body font-bold leading-snug text-white drop-shadow-sm">{c.name}</span>
        </div>
      )}

      {/* Bottom gradient + stars band. No name here on purpose: the v3 cover art
          carries the card's title in the artwork itself, so a text label under it
          just repeats what the student already reads. The name is still on the
          tile for non-visual paths — img alt, the button title tooltip, the hover
          overlay — and the text-face fallback below prints it in the middle. */}
      <div className="absolute inset-x-0 bottom-0 bg-gradient-to-t from-mk-ink/[.82] to-mk-ink/0 px-2.5 pb-2.5 pt-4">
        {encountered ? (
          <Stars n={c.stars} onDark />
        ) : (
          <span className="text-mk-small font-bold text-white/80">还没遇到</span>
        )}
      </div>

      {/* hover translucent description */}
      <div
        className={cx(
          "absolute inset-0 flex flex-col justify-center bg-mk-ink p-3.5 text-white transition-opacity duration-[160ms] ease-mk",
          hover ? "opacity-100" : "pointer-events-none opacity-0",
        )}
      >
        <div className="mb-1.5 text-mk-small font-extrabold leading-snug">{c.name}</div>
        <div className="line-clamp-5 text-mk-small leading-relaxed text-white/90">{c.purpose}</div>
        <div className="mt-2 text-mk-small font-semibold text-white/70">点开看详情 →</div>
      </div>
    </button>
  );
}

export function ToolkitCards({ onOpenCourse }: { onOpenCourse?: (courseId: string) => void } = {}) {
  const [cards, setCards] = useState<CardCatalogEntry[] | undefined>(undefined);
  // Course summaries drive the "在这些课程里学它" section of the detail modal —
  // a card's related courses = those whose card_ids include it (best-effort;
  // the gallery still works if this fetch fails).
  const [courses, setCourses] = useState<CourseSummary[]>([]);
  const [theme, setTheme] = useState<CoverTheme>("light");
  const [error, setError] = useState<string | null>(null);
  const [themeNotice, setThemeNotice] = useState<string | null>(null);
  const [selected, setSelected] = useState<string | null>(null);
  // Top filters: status (所有卡片 / 已练习过) and category tag ("all" or a
  // category key). Both are pure UI, applied on top of the fetched catalog.
  const [statusFilter, setStatusFilter] = useState<"all" | "practiced">("all");
  const [tagFilter, setTagFilter] = useState<string>("all");
  // Monotonic token so an out-of-order theme re-fetch (double-click) can't land
  // a stale theme's covers over a newer selection.
  const themeReq = useRef(0);

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        const res = await api.getCardsCatalog();
        if (!cancelled) { setCards(res.cards); setTheme(res.theme); }
      } catch {
        if (!cancelled) setError("加载失败，请重试");
      }
    })();
    void api.listCourses().then((cs) => { if (!cancelled) setCourses(cs); }).catch(() => {});
    return () => { cancelled = true; };
  }, []);

  // Picking a colorway re-fetches fresh signed covers and persists the choice.
  // A failure here is cosmetic: it shows a self-clearing notice and rolls the
  // selection back — it must NEVER blank the gallery (that's what `error` gates,
  // and `error` is only ever set by the initial load).
  async function pickTheme(t: CoverTheme) {
    if (t === theme) return;
    const prev = theme;
    const token = ++themeReq.current;
    setTheme(t); // optimistic
    setThemeNotice(null);
    try {
      const res = await api.getCardsCatalog(t);
      if (themeReq.current !== token) return; // a newer pick superseded this one
      setCards(res.cards);
      void api.setCardTheme(t).catch(() => {
        // covers already updated; a failed persist just reverts on next load
        setThemeNotice("配色没能保存，刷新后可能恢复");
        window.setTimeout(() => setThemeNotice(null), 3000);
      });
    } catch {
      if (themeReq.current !== token) return;
      setTheme(prev); // roll back the optimistic swatch
      setThemeNotice("换配色失败，请重试");
      window.setTimeout(() => setThemeNotice(null), 3000);
    }
  }

  // All non-empty category groups (drives the tag chips + their counts).
  const allGroups = useMemo(() => {
    if (!cards) return [];
    return [...CATEGORY_ORDER, OTHER].map(({ key, dot }) => ({
      key, dot,
      cards: cards.filter((c) => (key === OTHER.key
        ? !CATEGORY_ORDER.some((g) => g.key === c.category)
        : c.category === key)),
    })).filter((g) => g.cards.length > 0);
  }, [cards]);

  // Groups actually shown: narrowed by the selected tag, then by the status
  // filter (已练习过 keeps only encountered cards), dropping any emptied group.
  const visibleGroups = useMemo(() => {
    return allGroups
      .filter((g) => tagFilter === "all" || g.key === tagFilter)
      .map((g) => ({ ...g, cards: statusFilter === "practiced" ? g.cards.filter((c) => c.encountered) : g.cards }))
      .filter((g) => g.cards.length > 0);
  }, [allGroups, tagFilter, statusFilter]);

  if (error) return <div className="p-6 text-mk-body text-mk-danger">{error}</div>;
  if (cards === undefined) {
    return (
      <div className="p-6">
        <Skeleton h={16} w={280} className="mb-4" />
        <div className="grid grid-cols-[repeat(auto-fill,minmax(158px,1fr))] gap-4">
          {Array.from({ length: 8 }, (_, i) => (
            <Skeleton key={i} h={237} radius="md" />
          ))}
        </div>
      </div>
    );
  }

  const learnt = cards.filter((c) => c.encountered).length;
  const selectedCard = selected ? cards.find((c) => c.cardId === selected) : undefined;
  // Count shown on a chip reflects the current status filter (so it matches what
  // you'd actually see if you tapped it).
  const countIn = (cs: CardCatalogEntry[]) => (statusFilter === "practiced" ? cs.filter((c) => c.encountered).length : cs.length);

  return (
    <div className="mk-scroll h-full overflow-y-auto p-6">
      <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
        <div className="text-mk-body leading-relaxed text-mk-secondary">
          全部 {cards.length} 张思维工具卡 · 你已遇到 <b className="text-mk-ink">{learnt}</b> 张。练过的更鲜亮，还没遇到的会淡一些。
        </div>
        <div className="flex items-center gap-2">
          {themeNotice && <span className="text-mk-small font-semibold text-mk-danger">{themeNotice}</span>}
          <span className="text-mk-small font-semibold text-mk-muted">封面配色</span>
          {COVER_THEMES.map((t) => (
            <button
              key={t}
              type="button"
              onClick={() => pickTheme(t)}
              title={THEME_META[t].label}
              aria-label={THEME_META[t].label}
              aria-pressed={theme === t}
              className={cx(
                "h-[22px] w-[22px] shrink-0 rounded-mk-full border-2 border-mk-surface p-0 transition-shadow duration-[120ms] ease-mk",
                THEME_META[t].swatch,
                theme === t ? "ring-2 ring-mk-ink" : "ring-1 ring-mk-border",
              )}
            />
          ))}
        </div>
      </div>

      {/* filters: status (所有卡片 / 已练习过) + category tags */}
      <div className="mb-6 flex flex-wrap items-center gap-x-3 gap-y-2.5">
        <div className="inline-flex shrink-0 rounded-mk-full border border-mk-border p-0.5">
          <SegButton active={statusFilter === "all"} onClick={() => setStatusFilter("all")}>所有卡片 {cards.length}</SegButton>
          <SegButton active={statusFilter === "practiced"} onClick={() => setStatusFilter("practiced")}>已练习过 {learnt}</SegButton>
        </div>
        <div className="flex flex-wrap items-center gap-1.5">
          <TagChip active={tagFilter === "all"} onClick={() => setTagFilter("all")}>全部</TagChip>
          {allGroups.map((g) => (
            <TagChip key={g.key} active={tagFilter === g.key} dot={g.dot} onClick={() => setTagFilter(g.key)}>
              {g.key}
              <span className="text-mk-small font-bold opacity-60">{countIn(g.cards)}</span>
            </TagChip>
          ))}
        </div>
      </div>

      {visibleGroups.length === 0 ? (
        <div className="rounded-mk-md border border-dashed border-mk-border bg-mk-paper px-4 py-10 text-center text-mk-body text-mk-muted">
          {statusFilter === "practiced" ? "这里还没有你练习过的卡片——去课程里遇到第一张吧。" : "没有匹配的卡片。"}
        </div>
      ) : (
        visibleGroups.map((g) => (
          <div key={g.key} className="mb-8">
            <div className="mb-3 flex items-center gap-2">
              <span className={cx("h-[9px] w-[9px] shrink-0 rounded-mk-full", g.dot)} />
              <span className="text-mk-h3 text-mk-ink">{g.key}</span>
              <span className="text-mk-small font-semibold text-mk-muted">
                {g.cards.filter((c) => c.encountered).length}/{g.cards.length}
              </span>
            </div>
            <div className="grid grid-cols-[repeat(auto-fill,minmax(158px,1fr))] gap-4">
              {g.cards.map((c) => <CardTile key={c.cardId} c={c} onOpen={() => setSelected(c.cardId)} />)}
            </div>
          </div>
        ))
      )}

      {selectedCard && <CardDetailModal c={selectedCard} courses={courses} onClose={() => setSelected(null)} onOpenCourse={onOpenCourse} />}
    </div>
  );
}
