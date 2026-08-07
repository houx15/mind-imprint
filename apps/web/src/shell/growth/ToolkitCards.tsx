import { useEffect, useMemo, useRef, useState } from "react";
import { Star } from "lucide-react";
import { CARD_REGISTRY, COVER_THEMES, type CardCatalogEntry, type CoverTheme } from "@mind-imprint/contracts";
import { api } from "@/api";
import { Icon, Badge, Modal, Skeleton, type MacaronName, coverGradientStyle } from "@/ui";

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
const THEME_META: Record<CoverTheme, { label: string; swatch: string }> = {
  light: { label: "浅色", swatch: "bg-mk-paper" },
  "cyber-sage": { label: "青绿", swatch: "bg-mk-matcha" },
  "cyber-slate": { label: "石板", swatch: "bg-mk-ink" },
  "cyber-warm": { label: "暖调", swatch: "bg-mk-peach" },
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
  const encountered = c.encountered;
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
      {/* cover: image if we have one, else a text face */}
      {c.coverUrl ? (
        <img
          src={c.coverUrl}
          alt={c.name}
          loading="lazy"
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

function DetailModal({ c, onClose, onOpenCourse }: { c: Detail; onClose: () => void; onOpenCourse?: (courseId: string) => void }) {
  const spec = CARD_REGISTRY[c.cardId];
  const methodology = spec?.steps?.[0]?.methodology;
  const example = c.example || methodology?.example || methodology?.how || "";

  return (
    <Modal open onClose={onClose} title={c.name}>
      {c.nameEn && <div className="-mt-2 mb-3 text-mk-small text-mk-muted">{c.nameEn}</div>}
      <div className="mb-4 flex items-center gap-2.5">
        <Badge tone="draft">{c.category}</Badge>
        {c.encountered ? <Stars n={c.stars} /> : <span className="text-mk-small font-semibold text-mk-faint">还没遇到</span>}
      </div>

      <Section label="这张卡帮你做什么">{c.purpose}</Section>
      {c.stages.length > 0 && (
        <div className="mb-4">
          <SectionLabel>适用阶段</SectionLabel>
          <div className="flex flex-wrap gap-1.5">
            {c.stages.map((s) => <Badge key={s} tone="done">{s}</Badge>)}
          </div>
        </div>
      )}
      {methodology && (
        <>
          <Section label="为什么用">{methodology.why}</Section>
          <Section label="怎么用">{methodology.how}</Section>
          {methodology.when && <Section label="什么时候用">{methodology.when}</Section>}
        </>
      )}
      {example && <Section label="一个例子">{example}</Section>}

      {c.encountered && (
        <div className="mt-1 border-t border-mk-border pt-3.5">
          <SectionLabel>你的使用记录</SectionLabel>
          <div className="text-mk-body text-mk-secondary">
            已完成 <b>{c.uses}</b> 次
            {c.surfaces.length > 0 && <> · 出现在 {c.surfaces.map((s) => SURFACE_LABEL[s] ?? s).join(" · ")}</>}
            {c.lastUsed && <> · 最近 {c.lastUsed.slice(5, 10)}</>}
          </div>
          {methodology?.why && (
            <div className="mt-2.5">
              <SectionLabel>你练的思路</SectionLabel>
              <div className="text-mk-body text-mk-muted">{methodology.why}</div>
            </div>
          )}
        </div>
      )}

      {c.courseId && onOpenCourse && (
        <button
          type="button"
          onClick={() => onOpenCourse(c.courseId)}
          className="mt-4 w-full rounded-mk-md bg-mk-accent px-4 py-2.5 text-mk-body font-bold text-white transition-colors duration-[120ms] ease-mk hover:bg-mk-accent-600"
        >
          去学这张卡的课程 →
        </button>
      )}
    </Modal>
  );
}

function SectionLabel({ children }: { children: React.ReactNode }) {
  return <div className="mb-1.5 text-mk-label text-mk-faint">{children}</div>;
}
function Section({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="mb-3.5">
      <SectionLabel>{label}</SectionLabel>
      <div className="text-mk-body leading-relaxed text-mk-secondary">{children}</div>
    </div>
  );
}

export function ToolkitCards({ onOpenCourse }: { onOpenCourse?: (courseId: string) => void } = {}) {
  const [cards, setCards] = useState<CardCatalogEntry[] | undefined>(undefined);
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

      {selectedCard && <DetailModal c={selectedCard} onClose={() => setSelected(null)} onOpenCourse={onOpenCourse} />}
    </div>
  );
}
