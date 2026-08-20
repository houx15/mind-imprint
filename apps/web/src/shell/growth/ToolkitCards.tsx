import { useEffect, useMemo, useRef, useState } from "react";
import { createPortal } from "react-dom";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import type { Components } from "react-markdown";
import { Star, X, ArrowRight } from "lucide-react";
import { CARD_REGISTRY, COVER_THEMES, type CardCatalogEntry, type CoverTheme, type CourseSummary } from "@mind-imprint/contracts";
import { api } from "@/api";
import { Icon, Badge, Skeleton, type MacaronName, coverGradientStyle } from "@/ui";

/** Join truthy class fragments with a single space; drops falsy/empty ones. */
function cx(...parts: Array<string | false | null | undefined>): string {
  return parts.filter(Boolean).join(" ");
}

const SURFACE_LABEL: Record<string, string> = { project: "项目", course: "课程", chat: "聊天" };

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

/**
 * Read-only 5-star proficiency display; fill is warning gold (spec §8), never
 * accent. `onDark` swaps the empty-star color for legibility over the card
 * tile's dark cover scrim; the default (light-background contexts, e.g. the
 * detail modal) uses a neutral border tone instead.
 */
function Stars({ n, onDark = false }: { n: number; onDark?: boolean }) {
  return (
    <span aria-label={`熟练度 ${n} 星`} className="inline-flex items-center gap-0.5">
      {Array.from({ length: 5 }, (_, i) => (
        <Icon
          key={i}
          icon={Star}
          size={12}
          className={cx(
            i < n ? "text-mk-warning" : onDark ? "text-white/50" : "text-mk-faint",
            onDark && "drop-shadow-sm",
          )}
          fill={i < n ? "currentColor" : "none"}
        />
      ))}
    </span>
  );
}

type Detail = CardCatalogEntry;

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
            "absolute inset-0 h-full w-full object-cover transition-[filter] duration-200",
            encountered ? "grayscale-0" : "grayscale opacity-[.62] contrast-[.92]",
          )}
        />
      ) : (
        <div
          className={cx(
            "absolute inset-0 flex items-center justify-center p-3.5",
            !encountered && "grayscale opacity-[.62]",
          )}
          style={coverGradientStyle(c.cardId)}
        >
          <span className="text-center text-mk-body font-bold leading-snug text-white drop-shadow-sm">{c.name}</span>
        </div>
      )}

      {/* bottom gradient + stars/name band */}
      <div className="absolute inset-x-0 bottom-0 bg-gradient-to-t from-mk-ink/[.82] to-mk-ink/0 px-2.5 pb-2.5 pt-4">
        {encountered ? (
          <Stars n={c.stars} onDark />
        ) : (
          <span className="text-mk-small font-bold text-white/80">还没遇到</span>
        )}
        <div className="mt-0.5 truncate text-mk-small font-bold text-white">{c.name}</div>
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

function SectionLabel({ children }: { children: React.ReactNode }) {
  return <div className="mb-1.5 text-mk-label text-mk-faint">{children}</div>;
}

// A labeled field whose body is arbitrary content (markdown, chips, …).
function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div>
      <SectionLabel>{label}</SectionLabel>
      <div>{children}</div>
    </div>
  );
}

// Token-styled markdown for card copy. The card `purpose`/`methodology` fields
// are authored with **bold**, lists and `>` quotes; they were previously dropped
// in as raw strings (markup showed literally) — this renders them, using design
// tokens rather than the chat renderer's hardcoded palette.
const CARD_MD: Components = {
  p: ({ children }) => <p style={{ margin: "0 0 8px", lineHeight: 1.7 }}>{children}</p>,
  strong: ({ children }) => <strong style={{ fontWeight: 700, color: "var(--mk-ink)" }}>{children}</strong>,
  em: ({ children }) => <em>{children}</em>,
  ul: ({ children }) => <ul style={{ margin: "0 0 8px", paddingLeft: 20, listStyle: "disc" }}>{children}</ul>,
  ol: ({ children }) => <ol style={{ margin: "0 0 8px", paddingLeft: 20, listStyle: "decimal" }}>{children}</ol>,
  li: ({ children }) => <li style={{ margin: "3px 0", lineHeight: 1.65 }}>{children}</li>,
  a: ({ children, href }) => (
    <a href={href} target="_blank" rel="noreferrer" style={{ color: "var(--mk-accent-600)", textDecoration: "underline" }}>{children}</a>
  ),
  code: ({ children }) => (
    <code style={{ background: "var(--mk-paper)", borderRadius: 4, padding: "1px 5px", fontSize: "0.92em", fontFamily: "ui-monospace, SFMono-Regular, Menlo, monospace" }}>{children}</code>
  ),
  blockquote: ({ children }) => (
    <blockquote style={{ margin: "0 0 8px", paddingLeft: 12, borderLeft: "3px solid var(--mk-border)", color: "var(--mk-muted)" }}>{children}</blockquote>
  ),
};
function CardMd({ text }: { text: string }) {
  return (
    <div style={{ marginBottom: -8, fontSize: 14, color: "var(--mk-secondary)" }}>
      <ReactMarkdown remarkPlugins={[remarkGfm]} components={CARD_MD}>{text}</ReactMarkdown>
    </div>
  );
}

// The courses that teach a card, as tap-through rows — the "去学它" surface the
// student asked for. Empty → renders nothing (caller decides the empty copy).
function RelatedCourses({ label, courses, onOpenCourse }: { label: string; courses: CourseSummary[]; onOpenCourse?: (slug: string) => void }) {
  if (courses.length === 0) return null;
  return (
    <div>
      <SectionLabel>{label}</SectionLabel>
      <div className="flex flex-col gap-2">
        {courses.map((co) => (
          <button
            key={co.slug}
            type="button"
            onClick={() => onOpenCourse?.(co.slug)}
            disabled={!onOpenCourse}
            className={cx(
              "group flex items-center justify-between gap-3 rounded-mk-md border border-mk-border bg-mk-paper px-3.5 py-2.5 text-left transition-colors duration-[120ms] ease-mk",
              onOpenCourse ? "hover:border-mk-accent-300" : "cursor-default",
            )}
          >
            <span className="min-w-0">
              <span className="block truncate text-mk-body font-semibold text-mk-ink">{co.title}</span>
              <span className="text-mk-small text-mk-muted">{co.branch} · {co.time_label}</span>
            </span>
            {onOpenCourse && <Icon icon={ArrowRight} size={16} className="shrink-0 text-mk-muted group-hover:text-mk-accent-600" />}
          </button>
        ))}
      </div>
    </div>
  );
}

function HStat({ value, label, small = false }: { value: string; label: string; small?: boolean }) {
  return (
    <div className="rounded-mk-md border border-mk-border bg-mk-paper px-3 py-3 text-center">
      <div className={cx("truncate font-extrabold text-mk-ink", small ? "text-mk-body" : "text-[20px]")}>{value}</div>
      <div className="mt-0.5 text-mk-small font-semibold text-mk-muted">{label}</div>
    </div>
  );
}

function TabButton({ active, onClick, children }: { active: boolean; onClick: () => void; children: React.ReactNode }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cx(
        "relative px-3 py-2.5 text-mk-body font-semibold transition-colors duration-[120ms] ease-mk",
        active ? "text-mk-accent-600" : "text-mk-muted hover:text-mk-secondary",
      )}
    >
      {children}
      {active && <span className="absolute inset-x-2 -bottom-px h-[2.5px] rounded-mk-full bg-mk-accent-500" />}
    </button>
  );
}

// Wide two-column detail: left = cover art, right = header + two tabs (介绍 /
// 我的练习历史). Built as its own portal rather than the shared Modal so it can be
// wide and full-height (the shared Modal is hard-locked to a narrow width).
function CardDetailModal({ c, courses, onClose, onOpenCourse }: { c: Detail; courses: CourseSummary[]; onClose: () => void; onOpenCourse?: (slug: string) => void }) {
  const [tab, setTab] = useState<"intro" | "history">("intro");
  const [imgFailed, setImgFailed] = useState(false);
  const spec = CARD_REGISTRY[c.cardId];
  const methodology = spec?.steps?.[0]?.methodology;
  const example = c.example || methodology?.example || methodology?.how || "";
  const relatedCourses = useMemo(() => courses.filter((co) => co.card_ids.includes(c.cardId)), [courses, c.cardId]);
  const showImg = Boolean(c.coverUrl) && !imgFailed;

  useEffect(() => {
    function onKey(e: KeyboardEvent) { if (e.key === "Escape") onClose(); }
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [onClose]);

  if (typeof document === "undefined") return null;

  return createPortal(
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
      <div className="absolute inset-0 bg-black/40" onClick={onClose} aria-hidden="true" />
      <div role="dialog" aria-modal="true" aria-label={c.name} className="relative z-10 flex max-h-[88vh] w-full max-w-[820px] overflow-hidden rounded-mk-lg bg-mk-surface shadow-mk-lg">
        {/* left — cover art (hidden on the narrowest screens to keep the text readable) */}
        <div className="relative hidden w-[38%] shrink-0 bg-mk-paper sm:block">
          {showImg ? (
            <img
              src={c.coverUrl}
              alt={c.name}
              onError={() => setImgFailed(true)}
              className={cx("absolute inset-0 h-full w-full object-cover", !c.encountered && "opacity-90 grayscale")}
            />
          ) : (
            <div className="absolute inset-0 flex items-center justify-center p-5" style={coverGradientStyle(c.cardId)}>
              <span className="text-center text-mk-h3 font-bold leading-snug text-white drop-shadow-sm">{c.name}</span>
            </div>
          )}
        </div>

        {/* right — header + tabs + body */}
        <div className="flex min-w-0 flex-1 flex-col">
          <div className="flex items-start justify-between gap-3 border-b border-mk-border px-6 pb-4 pt-5">
            <div className="min-w-0">
              <h2 className="text-[22px] font-extrabold leading-tight text-mk-ink">{c.name}</h2>
              {c.nameEn && <div className="mt-1 text-mk-small text-mk-muted">{c.nameEn}</div>}
              <div className="mt-2.5 flex flex-wrap items-center gap-2.5">
                <Badge tone="draft">{c.category}</Badge>
                {c.encountered ? <Stars n={c.stars} /> : <span className="text-mk-small font-semibold text-mk-faint">还没遇到</span>}
              </div>
            </div>
            <button type="button" onClick={onClose} aria-label="关闭" className="shrink-0 rounded-mk-full p-1.5 text-mk-muted transition-colors duration-[120ms] ease-mk hover:bg-mk-paper hover:text-mk-ink">
              <Icon icon={X} size={18} />
            </button>
          </div>

          <div className="flex gap-1 border-b border-mk-border px-4">
            <TabButton active={tab === "intro"} onClick={() => setTab("intro")}>介绍</TabButton>
            <TabButton active={tab === "history"} onClick={() => setTab("history")}>我的练习历史</TabButton>
          </div>

          <div className="mk-scroll min-h-0 flex-1 overflow-y-auto px-6 py-5">
            {tab === "intro" ? (
              <div className="flex flex-col gap-5">
                <Field label="这张卡帮你做什么"><CardMd text={c.purpose} /></Field>
                {c.stages.length > 0 && (
                  <div>
                    <SectionLabel>适用阶段</SectionLabel>
                    <div className="flex flex-wrap gap-1.5">{c.stages.map((s) => <Badge key={s} tone="done">{s}</Badge>)}</div>
                  </div>
                )}
                {methodology?.why && <Field label="为什么用"><CardMd text={methodology.why} /></Field>}
                {methodology?.how && <Field label="怎么用"><CardMd text={methodology.how} /></Field>}
                {methodology?.when && <Field label="什么时候用"><CardMd text={methodology.when} /></Field>}
                {example && <Field label="一个例子"><CardMd text={example} /></Field>}
                <RelatedCourses label="在这些课程里学它" courses={relatedCourses} onOpenCourse={onOpenCourse} />
              </div>
            ) : c.encountered ? (
              <div className="flex flex-col gap-5">
                <div className="grid grid-cols-3 gap-2.5">
                  <HStat value={String(c.uses)} label="练习次数" />
                  <HStat value={String(c.stars)} label="熟练度（星）" />
                  <HStat small value={c.surfaces.length ? c.surfaces.map((s) => SURFACE_LABEL[s] ?? s).join("·") : "—"} label="出现场景" />
                </div>
                <div className="text-mk-body leading-relaxed text-mk-secondary">
                  你已经练过这张卡 <b className="text-mk-ink">{c.uses}</b> 次
                  {c.lastUsed && <>，最近一次在 <b className="text-mk-ink">{c.lastUsed.slice(0, 10)}</b></>}。
                </div>
                {methodology?.why && <Field label="你练的思路"><CardMd text={methodology.why} /></Field>}
                <RelatedCourses label="再练一次" courses={relatedCourses} onOpenCourse={onOpenCourse} />
              </div>
            ) : (
              <div className="flex flex-col gap-4">
                <div className="rounded-mk-md border border-dashed border-mk-border bg-mk-paper px-4 py-6 text-center">
                  <div className="text-mk-body font-semibold text-mk-secondary">你还没在项目里练过这张卡</div>
                  <div className="mt-1.5 text-mk-small text-mk-muted">在下面的课程里第一次遇到它，练过就会记录在这里。</div>
                </div>
                <RelatedCourses label="去学这张卡" courses={relatedCourses} onOpenCourse={onOpenCourse} />
              </div>
            )}
          </div>
        </div>
      </div>
    </div>,
    document.body,
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

  const groups = useMemo(() => {
    if (!cards) return [];
    return [...CATEGORY_ORDER, OTHER].map(({ key, dot }) => ({
      key, dot,
      cards: cards.filter((c) => (key === OTHER.key
        ? !CATEGORY_ORDER.some((g) => g.key === c.category)
        : c.category === key)),
    })).filter((g) => g.cards.length > 0);
  }, [cards]);

  if (error) return <div className="p-6 text-mk-body text-mk-danger">{error}</div>;
  if (cards === undefined) {
    return (
      <div className="p-6">
        <Skeleton h={16} w={280} className="mb-4" />
        <div className="grid grid-cols-[repeat(auto-fill,minmax(120px,1fr))] gap-3">
          {Array.from({ length: 10 }, (_, i) => (
            <Skeleton key={i} h={180} radius="md" />
          ))}
        </div>
      </div>
    );
  }

  const learnt = cards.filter((c) => c.encountered).length;
  const selectedCard = selected ? cards.find((c) => c.cardId === selected) : undefined;

  return (
    <div className="mk-scroll h-full overflow-y-auto p-6">
      <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
        <div className="text-mk-body leading-relaxed text-mk-secondary">
          全部 {cards.length} 张思维工具卡 · 你已遇到 <b className="text-mk-ink">{learnt}</b> 张。彩色是练过的，灰色是还没遇到的。
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

      {groups.map((g) => (
        <div key={g.key} className="mb-6">
          <div className="mb-3 flex items-center gap-2">
            <span className={cx("h-[9px] w-[9px] shrink-0 rounded-mk-full", g.dot)} />
            <span className="text-mk-h3 text-mk-ink">{g.key}</span>
            <span className="text-mk-small font-semibold text-mk-muted">
              {g.cards.filter((c) => c.encountered).length}/{g.cards.length}
            </span>
          </div>
          <div className="grid grid-cols-[repeat(auto-fill,minmax(120px,1fr))] gap-3">
            {g.cards.map((c) => <CardTile key={c.cardId} c={c} onOpen={() => setSelected(c.cardId)} />)}
          </div>
        </div>
      ))}

      {selectedCard && <CardDetailModal c={selectedCard} courses={courses} onClose={() => setSelected(null)} onOpenCourse={onOpenCourse} />}
    </div>
  );
}
