import { useEffect, useMemo, useRef, useState } from "react";
import { CARD_REGISTRY, COVER_THEMES, type CardCatalogEntry, type CoverTheme } from "@mind-imprint/contracts";
import { api } from "../../api";

const SURFACE_LABEL: Record<string, string> = { project: "项目", course: "课程", chat: "聊天" };

// Fixed display order of the card categories (roughly the course-library arc),
// each with a dot color. Any category not listed falls into a final 其他 group.
const CATEGORY_ORDER: { key: string; dot: string }[] = [
  { key: "探究启动", dot: "#D98263" },
  { key: "信息素养", dot: "#2A3B7A" },
  { key: "溯源与多视角", dot: "#4C9A82" },
  { key: "知识工具", dot: "#7C6BB0" },
  { key: "论证结构", dot: "#C77CA6" },
  { key: "AOK", dot: "#D9A23D" },
  { key: "AI伦理", dot: "#3E8EA8" },
  { key: "反身性与元认知", dot: "#5C6CB0" },
  { key: "成长与沉淀", dot: "#8A9A5B" },
  { key: "学科透镜", dot: "#4C9A82" },
];
const OTHER = { key: "其他", dot: "#9AA1B0" };

// The four cover colorways, with a representative swatch color for the picker.
const THEME_META: Record<CoverTheme, { label: string; swatch: string }> = {
  light: { label: "浅色", swatch: "#EDE7DA" },
  "cyber-sage": { label: "青绿", swatch: "#3F8F73" },
  "cyber-slate": { label: "石板", swatch: "#3A4256" },
  "cyber-warm": { label: "暖调", swatch: "#C9704B" },
};

// A deterministic soft gradient for the text-only face (cards with no cover art,
// or when OSS is off), so the gallery still reads as a wall of distinct cards.
function faceGradient(seed: string): string {
  let h = 0;
  for (let i = 0; i < seed.length; i++) h = (h * 31 + seed.charCodeAt(i)) % 360;
  return `linear-gradient(145deg, hsl(${h} 42% 62%), hsl(${(h + 40) % 360} 46% 48%))`;
}

function Stars({ n }: { n: number }) {
  return (
    <span aria-label={`熟练度 ${n} 星`} style={{ letterSpacing: 1, fontSize: 12, color: "#E9B949", textShadow: "0 1px 2px rgba(0,0,0,.35)" }}>
      {"★".repeat(n)}<span style={{ color: "rgba(255,255,255,.55)" }}>{"★".repeat(Math.max(0, 5 - n))}</span>
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
      style={{
        position: "relative", padding: 0, border: "1px solid #EAECF2", borderRadius: 14,
        overflow: "hidden", cursor: "pointer", background: "#fff", textAlign: "left",
        aspectRatio: "2 / 3", display: "block", width: "100%",
        boxShadow: hover ? "0 10px 26px rgba(15,20,45,.16)" : "0 1px 2px rgba(15,20,45,.05)",
        transform: hover ? "translateY(-2px)" : "none", transition: "box-shadow .15s, transform .15s",
      }}
    >
      {/* cover: image if we have one, else a text face */}
      {c.coverUrl ? (
        <img
          src={c.coverUrl} alt={c.name} loading="lazy"
          style={{
            position: "absolute", inset: 0, width: "100%", height: "100%", objectFit: "cover",
            filter: encountered ? "none" : "grayscale(1) contrast(.92) opacity(.62)",
            transition: "filter .2s",
          }}
        />
      ) : (
        <div style={{
          position: "absolute", inset: 0, background: faceGradient(c.cardId),
          filter: encountered ? "none" : "grayscale(1) opacity(.62)",
          display: "flex", alignItems: "center", justifyContent: "center", padding: 14,
        }}>
          <span style={{ color: "#fff", fontSize: 15, fontWeight: 800, lineHeight: 1.4, textAlign: "center", textShadow: "0 1px 3px rgba(0,0,0,.3)" }}>{c.name}</span>
        </div>
      )}

      {/* bottom gradient + stars/name band */}
      <div style={{
        position: "absolute", left: 0, right: 0, bottom: 0, padding: "18px 10px 9px",
        background: "linear-gradient(to top, rgba(10,14,30,.82), rgba(10,14,30,0))",
      }}>
        {encountered
          ? <Stars n={c.stars} />
          : <span style={{ fontSize: 11, fontWeight: 700, color: "rgba(255,255,255,.82)" }}>还没遇到</span>}
        <div style={{ fontSize: 12, fontWeight: 700, color: "#fff", lineHeight: 1.3, marginTop: 3, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{c.name}</div>
      </div>

      {/* hover translucent description */}
      <div style={{
        position: "absolute", inset: 0, background: "rgba(18,22,40,.86)", color: "#fff",
        padding: "14px 13px", display: "flex", flexDirection: "column", justifyContent: "center",
        opacity: hover ? 1 : 0, pointerEvents: "none", transition: "opacity .16s",
      }}>
        <div style={{ fontSize: 12.5, fontWeight: 800, marginBottom: 6, lineHeight: 1.35 }}>{c.name}</div>
        <div style={{ fontSize: 11.5, lineHeight: 1.6, color: "rgba(255,255,255,.9)", display: "-webkit-box", WebkitLineClamp: 5, WebkitBoxOrient: "vertical", overflow: "hidden" }}>{c.purpose}</div>
        <div style={{ fontSize: 11, marginTop: 9, color: "rgba(255,255,255,.66)", fontWeight: 600 }}>点开看详情 →</div>
      </div>
    </button>
  );
}

function DetailModal({ c, onClose, onOpenCourse }: { c: Detail; onClose: () => void; onOpenCourse?: (courseId: string) => void }) {
  const spec = CARD_REGISTRY[c.cardId];
  const methodology = spec?.steps?.[0]?.methodology;
  const example = c.example || methodology?.example || methodology?.how || "";

  return (
    <div
      style={{ position: "fixed", inset: 0, background: "rgba(22,28,46,.4)", display: "flex", alignItems: "center", justifyContent: "center", zIndex: 1000, padding: 20 }}
      onClick={onClose}
    >
      <div
        onClick={(e) => e.stopPropagation()}
        style={{ background: "#fff", borderRadius: 18, boxShadow: "0 24px 70px rgba(15,20,45,.4)", width: "100%", maxWidth: 460, maxHeight: "88vh", overflow: "auto" }}
      >
        {/* header — cover as a soft translucent backdrop behind the title */}
        <div style={{ position: "relative", padding: "22px 22px 16px", overflow: "hidden", borderBottom: "1px solid #F1F2F6" }}>
          {c.coverUrl && (
            <img src={c.coverUrl} alt="" aria-hidden style={{ position: "absolute", inset: 0, width: "100%", height: "100%", objectFit: "cover", opacity: 0.14, filter: "saturate(1.1)" }} />
          )}
          <div style={{ position: "relative" }}>
            <div style={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", gap: 12 }}>
              <div>
                <div style={{ fontSize: 18, fontWeight: 800, color: "#1C2333", lineHeight: 1.3 }}>{c.name}</div>
                {c.nameEn && <div style={{ fontSize: 12.5, color: "#8A92A3", marginTop: 2 }}>{c.nameEn}</div>}
              </div>
              <button type="button" onClick={onClose} aria-label="关闭" style={{ flex: "none", border: "none", background: "#F1F2F6", borderRadius: 999, width: 30, height: 30, cursor: "pointer", fontSize: 16, color: "#6B7384" }}>×</button>
            </div>
            <div style={{ display: "flex", alignItems: "center", gap: 10, marginTop: 10 }}>
              <span style={{ fontSize: 11.5, fontWeight: 700, color: "#5B6474", background: "rgba(255,255,255,.7)", border: "1px solid #EAECF2", borderRadius: 8, padding: "2px 8px" }}>{c.category}</span>
              {c.encountered ? <Stars n={c.stars} /> : <span style={{ fontSize: 11.5, fontWeight: 700, color: "#9AA1B0" }}>还没遇到</span>}
            </div>
          </div>
        </div>

        <div style={{ padding: "16px 22px 22px" }}>
          <Section label="这张卡帮你做什么">{c.purpose}</Section>
          {c.stages.length > 0 && (
            <div style={{ marginBottom: 16 }}>
              <SectionLabel>适用阶段</SectionLabel>
              <div style={{ display: "flex", flexWrap: "wrap", gap: 6 }}>
                {c.stages.map((s) => <span key={s} style={{ fontSize: 11.5, fontWeight: 600, color: "#4C9A82", background: "#EAF5F0", borderRadius: 8, padding: "3px 9px" }}>{s}</span>)}
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
            <div style={{ marginTop: 4, paddingTop: 14, borderTop: "1px solid #F1F2F6" }}>
              <SectionLabel>你的使用记录</SectionLabel>
              <div style={{ fontSize: 13, color: "#3A4256", lineHeight: 1.7 }}>
                已完成 <b>{c.uses}</b> 次
                {c.surfaces.length > 0 && <> · 出现在 {c.surfaces.map((s) => SURFACE_LABEL[s] ?? s).join(" · ")}</>}
                {c.lastUsed && <> · 最近 {c.lastUsed.slice(5, 10)}</>}
              </div>
              {methodology?.why && (
                <div style={{ marginTop: 10 }}>
                  <SectionLabel>你练的思路</SectionLabel>
                  <div style={{ fontSize: 13, color: "#6B7384", lineHeight: 1.7 }}>{methodology.why}</div>
                </div>
              )}
            </div>
          )}

          {c.courseId && onOpenCourse && (
            <button
              type="button"
              onClick={() => onOpenCourse(c.courseId)}
              style={{ marginTop: 18, width: "100%", border: "none", borderRadius: 12, padding: "11px 16px", background: "#2A3B7A", color: "#fff", fontSize: 13.5, fontWeight: 700, cursor: "pointer" }}
            >
              去学这张卡的课程 →
            </button>
          )}
        </div>
      </div>
    </div>
  );
}

function SectionLabel({ children }: { children: React.ReactNode }) {
  return <div style={{ fontSize: 11, fontWeight: 800, color: "#9AA1B0", letterSpacing: 0.4, marginBottom: 5, textTransform: "uppercase" }}>{children}</div>;
}
function Section({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div style={{ marginBottom: 15 }}>
      <SectionLabel>{label}</SectionLabel>
      <div style={{ fontSize: 13.5, color: "#3A4256", lineHeight: 1.75 }}>{children}</div>
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

  if (error) return <div style={{ padding: 24, color: "#B0432E" }}>{error}</div>;
  if (cards === undefined) return <div style={{ padding: 24, color: "#9AA1B0" }}>正在整理你的工具卡图鉴…</div>;

  const learnt = cards.filter((c) => c.encountered).length;
  const selectedCard = selected ? cards.find((c) => c.cardId === selected) : undefined;

  return (
    <div>
      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 12, marginBottom: 16, flexWrap: "wrap" }}>
        <div style={{ fontSize: 13.5, color: "#6B7384", lineHeight: 1.6 }}>
          全部 {cards.length} 张思维工具卡 · 你已遇到 <b style={{ color: "#1C2333" }}>{learnt}</b> 张。彩色是练过的，灰色是还没遇到的。
        </div>
        <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
          {themeNotice && <span style={{ fontSize: 11.5, color: "#B0432E", fontWeight: 600 }}>{themeNotice}</span>}
          <span style={{ fontSize: 12, color: "#9AA1B0", fontWeight: 600 }}>封面配色</span>
          {COVER_THEMES.map((t) => (
            <button
              key={t} type="button" onClick={() => pickTheme(t)} title={THEME_META[t].label}
              aria-label={THEME_META[t].label} aria-pressed={theme === t}
              style={{
                width: 22, height: 22, borderRadius: 999, cursor: "pointer", background: THEME_META[t].swatch,
                border: theme === t ? "2px solid #1C2333" : "2px solid #fff",
                boxShadow: theme === t ? "0 0 0 2px #1C2333" : "0 0 0 1px #D9DCE4", padding: 0,
              }}
            />
          ))}
        </div>
      </div>

      {groups.map((g) => (
        <div key={g.key} style={{ marginBottom: 26 }}>
          <div style={{ display: "flex", alignItems: "center", gap: 8, marginBottom: 12 }}>
            <span style={{ width: 9, height: 9, borderRadius: 999, background: g.dot, flex: "none" }} />
            <span style={{ fontSize: 14, fontWeight: 700, color: "#1C2333" }}>{g.key}</span>
            <span style={{ fontSize: 12, color: "#9AA1B0", fontWeight: 600 }}>{g.cards.filter((c) => c.encountered).length}/{g.cards.length}</span>
          </div>
          <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fill, minmax(120px, 1fr))", gap: 12 }}>
            {g.cards.map((c) => <CardTile key={c.cardId} c={c} onOpen={() => setSelected(c.cardId)} />)}
          </div>
        </div>
      ))}

      {selectedCard && <DetailModal c={selectedCard} onClose={() => setSelected(null)} onOpenCourse={onOpenCourse} />}
    </div>
  );
}
